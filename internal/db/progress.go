package db

import (
	"fmt"
	nanoid "github.com/matoous/go-nanoid/v2"
	"math"
	"math/rand" // IMPORTANT: Seed this generator once at application startup (e.g., rand.Seed(time.Now().UnixNano()))
	"time"
)

const (
	MaxReviewIntervalDays int = 365 * 10 // Maximum review interval in days

	// LearningStep1Duration Learning steps (durations) - used for initial learning and relearning
	LearningStep1Duration = 1 * time.Minute
	LearningStep2Duration = 10 * time.Minute
	// GraduateToReviewIntervalDays Graduation interval (when moving from Learning/Relearning to Review state)
	GraduateToReviewIntervalDays float64 = 1.0 // Days, for "Good"

	FuzzPercentage float64 = 0.05 // 5% fuzz for review intervals > 1 day
	DefaultEase            = 2.5
	MinEaseFactor          = 1.3
)

const (
	RatingAgain = 1
	RatingGood  = 2
)

type CardState string

const (
	StateNew        CardState = "new"
	StateLearning   CardState = "learning"
	StateReview     CardState = "review"
	StateRelearning CardState = "relearning"
)

type Review struct {
	ID           string        `db:"id" json:"id"`
	UserID       string        `db:"user_id" json:"user_id"`
	CardID       string        `db:"card_id" json:"card_id"`
	Rating       int           `db:"rating" json:"rating"` // 1=Again, 2=Good
	ReviewedAt   time.Time     `db:"reviewed_at" json:"reviewed_at"`
	TimeSpentMs  int           `db:"time_spent_ms" json:"time_spent_ms"`
	PrevInterval time.Duration `db:"prev_interval" json:"prev_interval"`
	NewInterval  time.Duration `db:"new_interval" json:"new_interval"`
	PrevEase     float64       `db:"prev_ease" json:"prev_ease"`
	NewEase      float64       `db:"new_ease" json:"new_ease"`
}

const ()

// In db/review.go

func (s *Storage) ReviewCard(card *Card, rating int, timeSpentMs int) error {
	now := time.Now()

	// Store original values for logging and specific logic
	initialCardState := CardState(card.State)
	prevInterval := card.Interval
	prevEase := card.Ease

	// 1. Calculate next parameters using the core function
	params, err := calculateNextReviewParameters(
		initialCardState,
		card.LearningStep,
		card.Interval, // This is the interval *before* this review
		card.Ease,     // This is the ease *before* this review
		rating,
	)

	if err != nil {
		return fmt.Errorf("failed to calculate next review parameters for card %s: %w", card.ID, err)
	}

	// 2. Update card fields based on calculated parameters
	card.State = string(params.State)
	card.LearningStep = params.LearningStep
	card.Ease = params.Ease
	card.Interval = params.Interval // This is the base new interval (unfuzzed)

	// 3. Handle LapsCount (specific to Review -> Relearning transition)
	if initialCardState == StateReview && rating == RatingAgain {
		card.LapsCount++
	}

	// 4. Apply Fuzzing if applicable (only for actual reviews, not previews)
	oneDay := 24 * time.Hour
	// Fuzzing condition: original state was Review, rating was Good, calculated interval > 1 day
	if initialCardState == StateReview && rating == RatingGood && card.Interval > oneDay && FuzzPercentage > 0.0 {
		fuzzRangeSeconds := card.Interval.Seconds() * FuzzPercentage
		// IMPORTANT: Ensure rand is seeded at application startup: rand.Seed(time.Now().UnixNano())
		fuzzAmountSeconds := (rand.Float64()*2.0 - 1.0) * fuzzRangeSeconds
		fuzzedInterval := card.Interval + time.Duration(fuzzAmountSeconds*float64(time.Second))

		// Re-apply clamps after fuzzing, as fuzzing might push it out of bounds
		minReviewInterval := time.Duration(GraduateToReviewIntervalDays * 24 * float64(time.Hour))
		if fuzzedInterval < minReviewInterval {
			fuzzedInterval = minReviewInterval
		}

		maxIntervalDuration := time.Duration(MaxReviewIntervalDays) * 24 * time.Hour
		if fuzzedInterval > maxIntervalDuration {
			fuzzedInterval = maxIntervalDuration
		}

		// Ensure interval is not zero/negative after fuzzing, using the *new* card state (params.State)
		if fuzzedInterval <= 0 {
			if params.State == StateLearning || params.State == StateRelearning {
				fuzzedInterval = LearningStep1Duration
			} else if params.State == StateReview {
				fuzzedInterval = time.Duration(GraduateToReviewIntervalDays * 24 * float64(time.Hour))
			} else {
				fuzzedInterval = LearningStep1Duration
			}
		}
		card.Interval = fuzzedInterval // Assign the fuzzed and re-clamped interval
	}

	// 5. Finalize other card updates
	nextReviewTime := now.Add(card.Interval)
	card.NextReview = &nextReviewTime
	card.ReviewCount++
	card.LastReviewedAt = &now
	if card.FirstReviewedAt == nil {
		card.FirstReviewedAt = &now
	}

	// 6. Database transaction (largely unchanged)
	tx, dbErr := s.db.Begin()
	if dbErr != nil {
		return fmt.Errorf("error starting transaction: %w", dbErr)
	}
	defer tx.Rollback() // Defer rollback in case of panic or early return

	reviewQuery := `
		INSERT INTO reviews (id, user_id, card_id, rating, reviewed_at, time_spent_ms, prev_interval, new_interval, prev_ease, new_ease)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	prevIntervalNs := prevInterval.Nanoseconds()
	newIntervalNs := card.Interval.Nanoseconds() // Use the final (possibly fuzzed) interval

	_, dbErr = tx.Exec(reviewQuery,
		nanoid.Must(), card.UserID, card.ID, rating, now, timeSpentMs,
		prevIntervalNs, newIntervalNs, prevEase, card.Ease, // Use prevEase and new card.Ease
	)
	if dbErr != nil {
		return fmt.Errorf("error creating review: %w", dbErr)
	}

	updateCardQuery := `
		UPDATE cards
		SET next_review = ?, interval = ?, ease = ?, review_count = ?,
		    laps_count = ?, last_reviewed_at = ?, first_reviewed_at = ?,
		    state = ?, learning_step = ?, updated_at = ?
		WHERE id = ? AND user_id = ?
	`
	_, dbErr = tx.Exec(updateCardQuery,
		card.NextReview, card.Interval.Nanoseconds(), card.Ease, card.ReviewCount,
		card.LapsCount, card.LastReviewedAt, card.FirstReviewedAt,
		card.State, card.LearningStep, now, // updated_at
		card.ID, card.UserID,
	)
	if dbErr != nil {
		return fmt.Errorf("error updating card: %w", dbErr)
	}

	if dbErr := tx.Commit(); dbErr != nil {
		return fmt.Errorf("error committing transaction: %w", dbErr)
	}

	return nil
}

type NextReviewParameters struct {
	Interval     time.Duration
	Ease         float64
	State        CardState
	LearningStep int
}

func calculateNextReviewParameters(
	currentState CardState,
	currentLearningStep int,
	currentInterval time.Duration,
	currentEase float64,
	rating int,
) (NextReviewParameters, error) {
	params := NextReviewParameters{
		Interval:     currentInterval,     // Start with current, will be updated
		Ease:         currentEase,         // Start with current, will be updated
		State:        currentState,        // Start with current, will be updated
		LearningStep: currentLearningStep, // Start with current, will be updated
	}

	effectivePrevEase := params.Ease // Ease to be used for calculations
	if currentState == StateNew {
		params.Ease = DefaultEase
		effectivePrevEase = DefaultEase
	} else {
		if params.Ease < MinEaseFactor {
			params.Ease = MinEaseFactor
			effectivePrevEase = MinEaseFactor
		}
	}

	switch currentState {
	case StateNew:
		params.State = StateLearning
		params.LearningStep = 1
		if rating == RatingAgain {
			params.Interval = LearningStep1Duration
		} else if rating == RatingGood {
			params.LearningStep = 2
			params.Interval = LearningStep2Duration
		}
		// Ease is set to DefaultEase for new cards, no adjustment here

	case StateLearning:
		if rating == RatingAgain {
			params.LearningStep = 1 // Reset to first step
			params.Interval = LearningStep1Duration
		} else if rating == RatingGood {
			if params.LearningStep == 1 {
				params.LearningStep = 2
				params.Interval = LearningStep2Duration
			} else { // Graduating from learning (e.g., from step 2)
				params.State = StateReview
				params.LearningStep = 0 // No longer in a specific learning step
				params.Interval = time.Duration(GraduateToReviewIntervalDays * 24 * float64(time.Hour))
			}
		}
		// Ease generally doesn't change during learning steps unless it's a new card (handled by initial ease setting)

	case StateReview:
		if rating == RatingAgain { // Lapse
			params.State = StateRelearning
			params.LearningStep = 2
			params.Interval = LearningStep2Duration
			params.Ease = math.Max(MinEaseFactor, effectivePrevEase-0.20) // Use ease before this review
		} else if rating == RatingGood {
			// State remains StateReview
			params.Ease = math.Max(MinEaseFactor, effectivePrevEase+0.10) // Use ease before this review

			calculatedIntervalValue := float64(currentInterval) * params.Ease // currentInterval is prevInterval here
			params.Interval = time.Duration(calculatedIntervalValue)

			minReviewInterval := time.Duration(GraduateToReviewIntervalDays * 24 * float64(time.Hour))
			if params.Interval < minReviewInterval {
				params.Interval = minReviewInterval
			}
			// Fuzzing is applied later in ReviewCard if needed, not here
		}

	case StateRelearning:
		if rating == RatingAgain {
			params.LearningStep = 1 // Reset to first relearning step
			params.Interval = LearningStep1Duration
		} else if rating == RatingGood {
			if params.LearningStep == 1 {
				params.LearningStep = 2
				params.Interval = LearningStep2Duration
			} else { // Graduating from relearning
				params.State = StateReview
				params.LearningStep = 0
				params.Interval = time.Duration(GraduateToReviewIntervalDays * 24 * float64(time.Hour))
			}
		}
		// Ease is not changed during relearning steps (it was adjusted at the lapse)

	default:
		return NextReviewParameters{}, fmt.Errorf("unknown card state: '%s'", currentState)
	}

	// Apply MaxReviewIntervalDays cap
	maxIntervalDuration := time.Duration(MaxReviewIntervalDays) * 24 * time.Hour
	if params.Interval > maxIntervalDuration {
		params.Interval = maxIntervalDuration
	}

	// Apply zero/negative interval fallback based on the *newly calculated* state (params.State)
	if params.Interval <= 0 {
		if params.State == StateLearning || params.State == StateRelearning {
			params.Interval = LearningStep1Duration
		} else if params.State == StateReview {
			params.Interval = time.Duration(GraduateToReviewIntervalDays * 24 * float64(time.Hour))
		} else { // Should not be StateNew here as it transitions out
			params.Interval = LearningStep1Duration // Fallback for any unexpected scenario
		}
	}

	return params, nil
}
