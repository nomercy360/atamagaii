package db

import (
	"database/sql"
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

type CardProgress struct {
	UserID          string        `db:"user_id" json:"user_id"`
	CardID          string        `db:"card_id" json:"card_id"`
	NextReview      *time.Time    `db:"next_review" json:"next_review,omitempty"`
	Interval        time.Duration `db:"interval" json:"interval"` // Stored as nanoseconds (or string if DB type is text)
	Ease            float64       `db:"ease" json:"ease"`
	ReviewCount     int           `db:"review_count" json:"review_count"`
	LapsCount       int           `db:"laps_count" json:"laps_count"`
	LastReviewedAt  *time.Time    `db:"last_reviewed_at" json:"last_reviewed_at,omitempty"`
	FirstReviewedAt *time.Time    `db:"first_reviewed_at" json:"first_reviewed_at,omitempty"`
	State           CardState     `db:"state" json:"state"`
	LearningStep    int           `db:"learning_step" json:"learning_step"`
}

func (s *Storage) ReviewCard(userID, cardID string, rating int, timeSpentMs int) error {
	if rating != RatingAgain && rating != RatingGood {
		return errors.New("invalid rating: must be 1 (Again) or 2 (Good)")
	}

	progress, err := s.GetCardProgress(userID, cardID)
	isNew := false
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		isNew = true
		progress = &CardProgress{
			UserID:       userID,
			CardID:       cardID,
			Ease:         DefaultEase,
			State:        StateNew,
			LearningStep: 0,
			Interval:     0,
		}
	} else if err != nil {
		return fmt.Errorf("error fetching card progress: %w", err)
	}

	now := time.Now()
	prevInterval := progress.Interval
	prevEase := progress.Ease
	currentState := progress.State

	if currentState == StateNew || currentState == StateLearning {
		progress.State = StateLearning
		switch rating {
		case RatingAgain:
			progress.LearningStep = 0 // Reset to first learning step
			progress.Interval = LearningStep1Duration
			progress.Ease = math.Max(MinEase, progress.Ease+EaseAdjIDontKnow)
		case RatingGood:
			progress.LearningStep++
			progress.Ease = math.Max(MinEase, progress.Ease+EaseAdjIKnow) // Typically no change or slight positive
			if progress.LearningStep < TotalLearningSteps {
				// Example: TotalLearningSteps = 2. Steps are 0, 1.
				// If current step was 0, now 1.
				if progress.LearningStep == 1 {
					progress.Interval = LearningStep2Duration
				}
				// Add more 'else if' for more learning steps if TotalLearningSteps > 2
			} else { // Graduated from all learning steps
				progress.State = StateReview
				progress.Interval = time.Duration(GraduateToReviewIntervalDays*24) * time.Hour
				progress.LearningStep = 0 // Reset for potential future relearning
			}
		default:
			return fmt.Errorf("internal error: unhandled rating %d in new/learning state", rating)
		}
	} else if currentState == StateReview {
		switch rating {
		case RatingAgain:
			progress.State = StateRelearning
			progress.LapsCount++
			progress.Ease = math.Max(MinEase, progress.Ease+EaseAdjIDontKnow)
			progress.LearningStep = 0 // Start relearning steps
			progress.Interval = LearningStep1Duration
		case RatingGood:
			progress.Ease = math.Max(MinEase, progress.Ease+EaseAdjIKnow)
			// Interval_new = Interval_old * Ease
			currentIntervalSec := math.Max(progress.Interval.Seconds(), 24*3600) // Min 1 day for calc base
			newIntervalSec := currentIntervalSec * progress.Ease
			progress.Interval = time.Duration(newIntervalSec) * time.Second
		default:
			return fmt.Errorf("internal error: unhandled rating %d in review state", rating)
		}
	} else if currentState == StateRelearning {
		switch rating {
		case RatingAgain:
			progress.LearningStep = 0 // Reset to first relearning step
			progress.Interval = LearningStep1Duration
			// Ease already penalized when lapsed. No further ease penalty here for simplicity.
		case RatingGood:
			progress.LearningStep++
			if progress.LearningStep < TotalLearningSteps {
				if progress.LearningStep == 1 { // Advanced from relearning step 0 to step 1
					progress.Interval = LearningStep2Duration
				}
			} else { // Graduated from all relearning steps
				progress.State = StateReview
				progress.Interval = time.Duration(GraduateToReviewIntervalDays*24) * time.Hour // Standard graduation
				progress.LearningStep = 0
			}
		default:
			return fmt.Errorf("internal error: unhandled rating %d in relearning state", rating)
		}
	}

	if progress.State == StateReview {
		// Ensure minimum interval of 1 day for review cards if calculation resulted in less.
		// Check if this card just graduated to StateReview with the standard 1-day interval.
		isRecentGraduationToReview := (currentState == StateLearning || currentState == StateRelearning) &&
			(progress.Interval == time.Duration(GraduateToReviewIntervalDays*24)*time.Hour)

		if progress.Interval < 24*time.Hour && !isRecentGraduationToReview {
			progress.Interval = 24 * time.Hour
		}

		if progress.Interval > 24*time.Hour {
			fuzzAmountNano := float64(progress.Interval.Nanoseconds()) * FuzzPercentage
			if fuzzAmountNano > 0 {
				randomOffset := time.Duration(rand.Int63n(int64(2*fuzzAmountNano)) - int64(fuzzAmountNano))
				progress.Interval += randomOffset
				if progress.Interval < 24*time.Hour { // Ensure fuzz doesn't make it too short
					progress.Interval = 24 * time.Hour
				}
			}
		}

		maxIntervalDuration := time.Duration(MaxReviewIntervalDays) * 24 * time.Hour
		if progress.Interval > maxIntervalDuration {
			progress.Interval = maxIntervalDuration
		}
	}

	nextReviewTime := now.Add(progress.Interval)
	progress.NextReview = &nextReviewTime

	progress.ReviewCount++
	progress.LastReviewedAt = &now
	if isNew || progress.FirstReviewedAt == nil {
		progress.FirstReviewedAt = &now
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
	newIntervalNs := progress.Interval.Nanoseconds()

	_, err = tx.Exec(reviewQuery,
		nanoid.Must(), userID, cardID,
		rating, // Store the actual user rating (1 or 2)
		now,
		timeSpentMs,
		prevIntervalNs,
		newIntervalNs,
		prevEase, progress.Ease,
	)
	if err != nil {
		return fmt.Errorf("error creating review: %w", err)
	}

	if isNew {
		progressQuery := `
			INSERT INTO card_progress (user_id, card_id, next_review, interval, ease, review_count, laps_count, last_reviewed_at, first_reviewed_at, state, learning_step)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		_, err = tx.Exec(progressQuery,
			userID, cardID, progress.NextReview, progress.Interval.Nanoseconds(), progress.Ease,
			progress.ReviewCount, progress.LapsCount, progress.LastReviewedAt,
			progress.FirstReviewedAt, progress.State, progress.LearningStep,
		)
	} else {
		progressQuery := `
			UPDATE card_progress
			SET next_review = ?, interval = ?, ease = ?, review_count = ?, laps_count = ?, last_reviewed_at = ?, state = ?, learning_step = ?
			WHERE user_id = ? AND card_id = ?
		`
		_, err = tx.Exec(progressQuery,
			progress.NextReview, progress.Interval.Nanoseconds(), progress.Ease, progress.ReviewCount,
			progress.LapsCount, progress.LastReviewedAt, progress.State, progress.LearningStep,
			userID, cardID,
		)
	}

	if err != nil {
		return fmt.Errorf("error updating/inserting card progress: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

func (s *Storage) GetCardProgress(userID, cardID string) (*CardProgress, error) {
	query := `
		SELECT user_id, card_id, next_review, interval, ease, review_count, laps_count, last_reviewed_at, first_reviewed_at, state, learning_step
		FROM card_progress
		WHERE user_id = ? AND card_id = ?
	`

	var progress CardProgress
	var intervalNs int64
	err := s.db.QueryRow(query, userID, cardID).Scan(
		&progress.UserID,
		&progress.CardID,
		&progress.NextReview,
		&intervalNs,
		&progress.Ease,
		&progress.ReviewCount,
		&progress.LapsCount,
		&progress.LastReviewedAt,
		&progress.FirstReviewedAt,
		&progress.State,
		&progress.LearningStep,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err // Return sql.ErrNoRows as is, so ReviewCard can detect new cards
		}
		return nil, fmt.Errorf("error getting card progress: %w", err)
	}

	progress.Interval = time.Duration(intervalNs)

	return &progress, nil
}

func (s *Storage) GetDueCardCount(userID string) (int, error) {
	reviewTime := time.Now().Truncate(24 * time.Hour).Add(24*time.Hour - time.Nanosecond)
	query := `
		SELECT COUNT(*)
		FROM card_progress
		WHERE user_id = ? AND next_review <= ?
	`
	var count int
	err := s.db.QueryRow(query, userID, reviewTime).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error getting due card count: %w", err)
	}

	return count, nil
}

func (s *Storage) ResetProgress(userID string, deckID string) error {
	var query string
	var args []interface{}

	if deckID != "" {
		query = ` 
			DELETE FROM card_progress
			WHERE user_id = ? AND card_id IN (
				SELECT id FROM cards WHERE deck_id = ? 
			)
		`
		args = []interface{}{userID, deckID}

		reviewQuery := `
			DELETE FROM reviews
			WHERE user_id = ? AND card_id IN (
				SELECT id FROM cards WHERE deck_id = ?
			)
		`
		if _, err := s.db.Exec(reviewQuery, userID, deckID); err != nil {
			return fmt.Errorf("error resetting review history for deck %s: %w", deckID, err)
		}

	} else {
		query = `DELETE FROM card_progress WHERE user_id = ?`
		args = []interface{}{userID}

		reviewQuery := `DELETE FROM reviews WHERE user_id = ?`
		if _, err := s.db.Exec(reviewQuery, userID); err != nil {
			return fmt.Errorf("error resetting all review history for user %s: %w", userID, err)
		}
	}

	_, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("error resetting card progress: %w", err)
	}
	return nil
}
