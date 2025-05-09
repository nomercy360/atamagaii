package db

import (
	"errors"
	"fmt"
	nanoid "github.com/matoous/go-nanoid/v2"
	"math"
	"math/rand"
	"time"
)

const (
	MinEase     float64 = 1.3 // Minimum ease factor
	MaxEase     float64 = 2.5 // Maximum ease factor
	DefaultEase float64 = 2.5 // Initial ease factor for new cards

	// Ease factor adjustments based on rating
	EaseAdjIDontKnow      float64 = -0.20 // Penalty for "Again" (like original "Again")
	EaseAdjIKnow          float64 = 0.0   // No change for "Good" (like original "Good")
	MaxReviewIntervalDays int     = 36500 // Maximum interval in days (Increased to ~100 years as often preferred)

	// LearningStep1Duration Learning steps (durations) - used for initial learning and relearning
	LearningStep1Duration time.Duration = 1 * time.Minute
	LearningStep2Duration time.Duration = 10 * time.Minute
	// TotalLearningSteps Number of learning/relearning steps before graduation (e.g., 2 steps: 0 and 1)
	TotalLearningSteps int = 2 // Means steps 0 and 1. After step 1 + IKnow, card graduates.
	// GraduateToReviewIntervalDays Graduation interval (when moving from Learning/Relearning to Review state)
	GraduateToReviewIntervalDays float64 = 1.0 // Days, for "Good"

	FuzzPercentage float64 = 0.05 // 5% fuzz for review intervals > 1 day
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
	Rating       int           `db:"rating" json:"rating"` // 1=IDontKnow, 2=IKnow
	ReviewedAt   time.Time     `db:"reviewed_at" json:"reviewed_at"`
	TimeSpentMs  int           `db:"time_spent_ms" json:"time_spent_ms"`
	PrevInterval time.Duration `db:"prev_interval" json:"prev_interval"` // Stored as string in DB
	NewInterval  time.Duration `db:"new_interval" json:"new_interval"`   // Stored as string in DB
	PrevEase     float64       `db:"prev_ease" json:"prev_ease"`
	NewEase      float64       `db:"new_ease" json:"new_ease"`
}

func (s *Storage) ReviewCard(userID, cardID string, rating int, timeSpentMs int) error {
	if rating != RatingAgain && rating != RatingGood {
		return errors.New("invalid rating: must be 1 (Again) or 2 (Good)")
	}

	card, err := s.GetCard(cardID, userID)
	isNew := false
	if err != nil && errors.Is(err, ErrNotFound) {
		return fmt.Errorf("card not found: %w", err)
	} else if err != nil {
		return fmt.Errorf("error fetching card: %w", err)
	}

	// If the card has state "new" and no reviews yet, it's considered new
	if card.State == string(StateNew) && card.ReviewCount == 0 {
		isNew = true
	}

	now := time.Now()
	prevInterval := card.Interval
	prevEase := card.Ease
	currentState := CardState(card.State)

	if currentState == StateNew || currentState == StateLearning {
		card.State = string(StateLearning)
		switch rating {
		case RatingAgain:
			card.LearningStep = 0 // Reset to first learning step
			card.Interval = LearningStep1Duration
			card.Ease = math.Max(MinEase, card.Ease+EaseAdjIDontKnow)
		case RatingGood:
			card.LearningStep++
			card.Ease = math.Max(MinEase, card.Ease+EaseAdjIKnow) // Typically no change or slight positive
			if card.LearningStep < TotalLearningSteps {
				// Example: TotalLearningSteps = 2. Steps are 0, 1.
				// If current step was 0, now 1.
				if card.LearningStep == 1 {
					card.Interval = LearningStep2Duration
				}
				// Add more 'else if' for more learning steps if TotalLearningSteps > 2
			} else { // Graduated from all learning steps
				card.State = string(StateReview)
				card.Interval = time.Duration(GraduateToReviewIntervalDays*24) * time.Hour
				card.LearningStep = 0 // Reset for potential future relearning
			}
		default:
			return fmt.Errorf("internal error: unhandled rating %d in new/learning state", rating)
		}
	} else if currentState == StateReview {
		switch rating {
		case RatingAgain:
			card.State = string(StateRelearning)
			card.LapsCount++
			card.Ease = math.Max(MinEase, card.Ease+EaseAdjIDontKnow)
			card.LearningStep = 0 // Start relearning steps
			card.Interval = LearningStep1Duration
		case RatingGood:
			card.Ease = math.Max(MinEase, card.Ease+EaseAdjIKnow)
			// Interval_new = Interval_old * Ease
			currentIntervalSec := math.Max(card.Interval.Seconds(), 24*3600) // Min 1 day for calc base
			newIntervalSec := currentIntervalSec * card.Ease
			card.Interval = time.Duration(newIntervalSec) * time.Second
		default:
			return fmt.Errorf("internal error: unhandled rating %d in review state", rating)
		}
	} else if currentState == StateRelearning {
		switch rating {
		case RatingAgain:
			card.LearningStep = 0 // Reset to first relearning step
			card.Interval = LearningStep1Duration
			// Ease already penalized when lapsed. No further ease penalty here for simplicity.
		case RatingGood:
			card.LearningStep++
			if card.LearningStep < TotalLearningSteps {
				if card.LearningStep == 1 { // Advanced from relearning step 0 to step 1
					card.Interval = LearningStep2Duration
				}
			} else { // Graduated from all relearning steps
				card.State = string(StateReview)
				card.Interval = time.Duration(GraduateToReviewIntervalDays*24) * time.Hour // Standard graduation
				card.LearningStep = 0
			}
		default:
			return fmt.Errorf("internal error: unhandled rating %d in relearning state", rating)
		}
	}

	if CardState(card.State) == StateReview {
		// Ensure minimum interval of 1 day for review cards if calculation resulted in less.
		// Check if this card just graduated to StateReview with the standard 1-day interval.
		isRecentGraduationToReview := (currentState == StateLearning || currentState == StateRelearning) &&
			(card.Interval == time.Duration(GraduateToReviewIntervalDays*24)*time.Hour)

		if card.Interval < 24*time.Hour && !isRecentGraduationToReview {
			card.Interval = 24 * time.Hour
		}

		if card.Interval > 24*time.Hour {
			fuzzAmountNano := float64(card.Interval.Nanoseconds()) * FuzzPercentage
			if fuzzAmountNano > 0 {
				randomOffset := time.Duration(rand.Int63n(int64(2*fuzzAmountNano)) - int64(fuzzAmountNano))
				card.Interval += randomOffset
				if card.Interval < 24*time.Hour { // Ensure fuzz doesn't make it too short
					card.Interval = 24 * time.Hour
				}
			}
		}

		maxIntervalDuration := time.Duration(MaxReviewIntervalDays) * 24 * time.Hour
		if card.Interval > maxIntervalDuration {
			card.Interval = maxIntervalDuration
		}
	}

	nextReviewTime := now.Add(card.Interval)
	card.NextReview = &nextReviewTime

	card.ReviewCount++
	card.LastReviewedAt = &now
	if isNew || card.FirstReviewedAt == nil {
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
		nanoid.Must(), userID, cardID,
		rating, // Store the actual user rating (1 or 2)
		now,
		timeSpentMs,
		prevIntervalNs,
		newIntervalNs,
		prevEase, card.Ease,
	)
	if err != nil {
		return fmt.Errorf("error creating review: %w", err)
	}

	// Update card with new progress values
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
		cardID,
		userID,
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
