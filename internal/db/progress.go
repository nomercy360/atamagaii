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
	LearningStep1Duration time.Duration = 1 * time.Minute
	LearningStep2Duration time.Duration = 10 * time.Minute
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

func (s *Storage) ReviewCard(card *Card, rating int, timeSpentMs int) error {
	now := time.Now()
	prevInterval := card.Interval // Interval value *before* this review
	prevEase := card.Ease         // Ease value *before* this review
	currentState := CardState(card.State)

	if currentState == StateNew {
		card.Ease = DefaultEase
	} else {
		if card.Ease < MinEaseFactor {
			card.Ease = MinEaseFactor
		}
	}

	switch currentState {
	case StateNew:
		card.State = string(StateLearning)
		card.LearningStep = 1

		card.Interval = LearningStep1Duration

	case StateLearning:
		if rating == RatingAgain {
			card.LearningStep = 1
			card.Interval = LearningStep1Duration
		} else if rating == RatingGood {
			if card.LearningStep == 1 {
				card.LearningStep = 2
				card.Interval = LearningStep2Duration
			} else {
				card.State = string(StateReview)
				card.LearningStep = 0
				card.Interval = time.Duration(GraduateToReviewIntervalDays * 24 * float64(time.Hour))
			}
		}

	case StateReview:
		if rating == RatingAgain { // Lapse
			card.State = string(StateRelearning)
			card.LearningStep = 1
			card.Interval = LearningStep1Duration
			card.Ease = math.Max(MinEaseFactor, card.Ease-0.20) // Reduce current ease
			card.LapsCount++
		} else if rating == RatingGood {
			card.Ease = math.Max(MinEaseFactor, card.Ease+0.10)

			calculatedIntervalValue := float64(prevInterval) * card.Ease
			card.Interval = time.Duration(calculatedIntervalValue)

			minReviewInterval := time.Duration(GraduateToReviewIntervalDays * 24 * float64(time.Hour))
			if card.Interval < minReviewInterval {
				card.Interval = minReviewInterval
			}

			oneDay := 24 * time.Hour
			if card.Interval > oneDay && FuzzPercentage > 0.0 {
				fuzzRangeSeconds := card.Interval.Seconds() * FuzzPercentage
				fuzzAmountSeconds := (rand.Float64()*2.0 - 1.0) * fuzzRangeSeconds
				card.Interval += time.Duration(fuzzAmountSeconds * float64(time.Second))

				if card.Interval < minReviewInterval {
					card.Interval = minReviewInterval
				}
			}
		}

	case StateRelearning:
		if rating == RatingAgain {
			card.LearningStep = 1 // Reset to first relearning step
			card.Interval = LearningStep1Duration
		} else if rating == RatingGood {
			if card.LearningStep == 1 {
				card.LearningStep = 2
				card.Interval = LearningStep2Duration
			} else { // Assuming current step is 2 (or any other step implies graduation)
				card.State = string(StateReview)
				card.LearningStep = 0 // No longer in a specific learning step
				card.Interval = time.Duration(GraduateToReviewIntervalDays * 24 * float64(time.Hour))
			}
		}
	default:
		return fmt.Errorf("unknown card state: '%s' for card ID %s", card.State, card.ID)
	}

	maxIntervalDuration := time.Duration(MaxReviewIntervalDays) * 24 * time.Hour
	if card.Interval > maxIntervalDuration {
		card.Interval = maxIntervalDuration
	}

	if card.Interval <= 0 {
		if card.State == string(StateLearning) || card.State == string(StateRelearning) {
			card.Interval = LearningStep1Duration
		} else if card.State == string(StateReview) {
			card.Interval = time.Duration(GraduateToReviewIntervalDays * 24 * float64(time.Hour))
		} else {
			card.Interval = LearningStep1Duration
		}
	}

	nextReviewTime := now.Add(card.Interval)
	card.NextReview = &nextReviewTime

	card.ReviewCount++
	card.LastReviewedAt = &now
	if card.FirstReviewedAt == nil {
		card.FirstReviewedAt = &now
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}

	defer tx.Rollback()

	reviewQuery := `
		INSERT INTO reviews (id, user_id, card_id, rating, reviewed_at, time_spent_ms, prev_interval, new_interval, prev_ease, new_ease)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	prevIntervalNs := prevInterval.Nanoseconds()
	newIntervalNs := card.Interval.Nanoseconds()

	_, err = tx.Exec(reviewQuery,
		nanoid.Must(),
		card.UserID,
		card.ID,
		rating,
		now,
		timeSpentMs,
		prevIntervalNs,
		newIntervalNs,
		prevEase,
		card.Ease,
	)
	if err != nil {
		return fmt.Errorf("error creating review: %w", err)
	}

	updateCardQuery := `
		UPDATE cards
		SET next_review = ?, interval = ?, ease = ?, review_count = ?,
		    laps_count = ?, last_reviewed_at = ?, first_reviewed_at = ?,
		    state = ?, learning_step = ?, updated_at = ?
		WHERE id = ? AND user_id = ?
	`
	_, err = tx.Exec(updateCardQuery,
		card.NextReview,
		card.Interval.Nanoseconds(),
		card.Ease,
		card.ReviewCount,
		card.LapsCount,
		card.LastReviewedAt,
		card.FirstReviewedAt,
		card.State,
		card.LearningStep,
		now, // updated_at
		card.ID,
		card.UserID,
	)
	if err != nil {
		return fmt.Errorf("error updating card: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

func (s *Storage) GetDueCardCount(userID string) (int, error) {
	reviewTime := time.Now().Truncate(24 * time.Hour).Add(24*time.Hour - time.Nanosecond)
	query := `
		SELECT COUNT(*)
		FROM cards
		WHERE user_id = ? AND next_review IS NOT NULL AND next_review <= ? AND deleted_at IS NULL
	`
	var count int
	err := s.db.QueryRow(query, userID, reviewTime).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error getting due card count: %w", err)
	}

	return count, nil
}
