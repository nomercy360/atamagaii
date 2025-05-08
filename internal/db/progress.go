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

// Constants for the Anki-like algorithm
const (
	MinEase          = 1.3
	MaxEase          = 2.5
	DefaultEase      = 2.5
	EaseBonus        = 1.3 // Bonus for Easy rating
	EasePenaltyAgain = -0.8
	EasePenaltyHard  = -0.2
	EaseBonusEasy    = 0.15
	MaxInterval      = 365 // Maximum interval in days
	LearningStep1Min = 1   // First learning step: 1 minute
	LearningStep2Min = 10  // Second learning step: 10 minutes
)

// CardState represents the state of a card
type CardState string

const (
	StateNew        CardState = "new"
	StateLearning   CardState = "learning"
	StateReview     CardState = "review"
	StateRelearning CardState = "relearning"
)

// Review represents a card review event
type Review struct {
	ID           string        `db:"id" json:"id"`
	UserID       string        `db:"user_id" json:"user_id"`
	CardID       string        `db:"card_id" json:"card_id"`
	Rating       int           `db:"rating" json:"rating"` // 1=Again, 2=Hard, 3=Good, 4=Easy
	ReviewedAt   time.Time     `db:"reviewed_at" json:"reviewed_at"`
	TimeSpentMs  int           `db:"time_spent_ms" json:"time_spent_ms"`
	PrevInterval time.Duration `db:"prev_interval" json:"prev_interval"`
	NewInterval  time.Duration `db:"new_interval" json:"new_interval"`
	PrevEase     float64       `db:"prev_ease" json:"prev_ease"`
	NewEase      float64       `db:"new_ease" json:"new_ease"`
}

type CardProgress struct {
	UserID          string        `db:"user_id" json:"user_id"`
	CardID          string        `db:"card_id" json:"card_id"`
	NextReview      *time.Time    `db:"next_review" json:"next_review,omitempty"`
	Interval        time.Duration `db:"interval" json:"interval"`         // stored as a duration string (e.g. "10m", "24h")
	Ease            float64       `db:"ease" json:"ease"`                 // Anki-like ease factor
	ReviewCount     int           `db:"review_count" json:"review_count"` // total reviews
	LapsCount       int           `db:"laps_count" json:"laps_count"`     // times forgotten
	LastReviewedAt  *time.Time    `db:"last_reviewed_at" json:"last_reviewed_at,omitempty"`
	FirstReviewedAt *time.Time    `db:"first_reviewed_at" json:"first_reviewed_at,omitempty"`
	State           CardState     `db:"state" json:"state"`                 // new, learning, review, relearning
	LearningStep    int           `db:"learning_step" json:"learning_step"` // tracks current learning step
}

func (s *Storage) ReviewCard(userID, cardID string, rating int, timeSpentMs int) error {
	if rating < 1 || rating > 4 {
		return errors.New("invalid rating: must be between 1 and 4")
	}

	// Reuse the GetCardProgress to properly handle interval
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

	// Record previous values for the review history
	prevInterval := progress.Interval
	prevEase := progress.Ease
	now := time.Now()

	// Update card progress
	progress.ReviewCount++
	progress.LastReviewedAt = &now
	if progress.FirstReviewedAt == nil {
		progress.FirstReviewedAt = &now
	}

	// Handle card state transitions and interval calculations
	if progress.State == StateNew || progress.State == StateLearning {
		// Handle learning phase
		if rating == 1 { // Again
			progress.LearningStep = 0
			progress.Interval = time.Duration(LearningStep1Min) * time.Minute
			progress.Ease = math.Max(progress.Ease+EasePenaltyAgain, MinEase)
		} else if rating == 2 { // Hard
			progress.LearningStep = 0
			progress.Interval = time.Duration(LearningStep1Min) * time.Minute
			progress.Ease = math.Max(progress.Ease+EasePenaltyHard, MinEase)
		} else if rating == 3 { // Good
			progress.LearningStep++
			if progress.LearningStep == 1 {
				progress.State = StateLearning
				progress.Interval = time.Duration(LearningStep2Min) * time.Minute
			} else {
				// Graduate to review
				progress.State = StateReview
				progress.Interval = 24 * time.Hour // First review in 1 day
				progress.LearningStep = 0
			}
		} else { // Easy
			// Graduate immediately to review
			progress.State = StateReview
			progress.Interval = 4 * 24 * time.Hour // Easy cards get 4 days
			progress.LearningStep = 0
			progress.Ease = math.Min(progress.Ease+EaseBonusEasy, MaxEase)
		}
		// Set next review based on interval
		nextReview := now.Add(progress.Interval)
		progress.NextReview = &nextReview
	} else if progress.State == StateReview || progress.State == StateRelearning {
		// Handle review or relearning phase
		if rating == 1 { // Again
			progress.State = StateRelearning
			progress.LapsCount++
			progress.Ease = math.Max(progress.Ease+EasePenaltyAgain, MinEase)
			progress.Interval = time.Minute // Relearn with short interval
			progress.LearningStep = 0
			nextReview := now.Add(progress.Interval)
			progress.NextReview = &nextReview
		} else {
			// Calculate new interval for Hard, Good, Easy
			var newInterval time.Duration

			if rating == 2 { // Hard
				progress.Ease = math.Max(progress.Ease+EasePenaltyHard, MinEase)
				newInterval = time.Duration(float64(progress.Interval.Hours()) * 1.2 * float64(time.Hour))
			} else if rating == 3 { // Good
				if progress.ReviewCount == 1 {
					newInterval = 24 * time.Hour // 1 day
				} else if progress.ReviewCount == 2 {
					newInterval = 4 * 24 * time.Hour // 4 days
				} else {
					newInterval = time.Duration(float64(progress.Interval) * progress.Ease)
				}
			} else { // Easy
				progress.Ease = math.Min(progress.Ease+EaseBonusEasy, MaxEase)
				newInterval = time.Duration(float64(progress.Interval) * progress.Ease * EaseBonus)
			}

			// Apply interval fuzzing (Anki-style)
			if newInterval > 24*time.Hour {
				fuzz := time.Duration(float64(newInterval) * 0.05) // 5% fuzz
				if fuzz > 0 {
					r := rand.Intn(int(fuzz)*2) - int(fuzz)
					newInterval += time.Duration(r)
				}
			}

			// Cap interval
			maxIntervalDuration := time.Duration(MaxInterval) * 24 * time.Hour
			if newInterval > maxIntervalDuration {
				newInterval = maxIntervalDuration
			}

			progress.Interval = newInterval

			// If relearning and rating >= 2, graduate back to review
			if progress.State == StateRelearning && rating >= 2 {
				progress.State = StateReview
			}

			// Set next review time
			nextReview := now.Add(progress.Interval)
			progress.NextReview = &nextReview
		}
	}

	// Save the review in transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}

	// Create review history entry
	reviewQuery := `
		INSERT INTO reviews (id, user_id, card_id, rating, reviewed_at, time_spent_ms, prev_interval, new_interval, prev_ease, new_ease)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = tx.Exec(reviewQuery,
		nanoid.Must(),
		userID,
		cardID,
		rating,
		now,
		timeSpentMs,
		prevInterval.String(),
		progress.Interval.String(),
		prevEase,
		progress.Ease,
	)

	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error creating review: %w", err)
	}

	// Update or insert progress
	var progressQuery string
	var args []interface{}

	if isNew {
		progressQuery = `
			INSERT INTO card_progress (user_id, card_id, next_review, interval, ease, review_count, laps_count, last_reviewed_at, first_reviewed_at, state, learning_step)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		args = []interface{}{
			userID,
			cardID,
			progress.NextReview,
			progress.Interval.String(),
			progress.Ease,
			progress.ReviewCount,
			progress.LapsCount,
			progress.LastReviewedAt,
			progress.FirstReviewedAt,
			progress.State,
			progress.LearningStep,
		}
	} else {
		progressQuery = `
			UPDATE card_progress
			SET next_review = ?, interval = ?, ease = ?, review_count = ?, laps_count = ?, last_reviewed_at = ?, state = ?, learning_step = ?
			WHERE user_id = ? AND card_id = ?
		`
		args = []interface{}{
			progress.NextReview,
			progress.Interval.String(),
			progress.Ease,
			progress.ReviewCount,
			progress.LapsCount,
			progress.LastReviewedAt,
			progress.State,
			progress.LearningStep,
			userID,
			cardID,
		}
	}

	_, err = tx.Exec(progressQuery, args...)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error updating card progress: %w", err)
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
	var intervalStr string
	err := s.db.QueryRow(query, userID, cardID).Scan(
		&progress.UserID,
		&progress.CardID,
		&progress.NextReview,
		&intervalStr,
		&progress.Ease,
		&progress.ReviewCount,
		&progress.LapsCount,
		&progress.LastReviewedAt,
		&progress.FirstReviewedAt,
		&progress.State,
		&progress.LearningStep,
	)

	if err != nil {
		return nil, fmt.Errorf("error getting card progress: %w", err)
	}

	// Parse the interval string to time.Duration
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing interval duration: %w", err)
	}
	progress.Interval = interval

	return &progress, nil
}

func (s *Storage) GetDueCardCount(userID string) (int, error) {
	decks, err := s.GetDecks(userID)
	if err != nil {
		return 0, fmt.Errorf("error fetching decks for due card count: %w", err)
	}

	var totalDueCount int
	for _, deck := range decks {
		// Use the total count from deck metrics (NewCards already limited in UpdateDeckMetrics)
		dueCount := deck.NewCards + deck.LearningCards + deck.ReviewCards
		totalDueCount += dueCount
	}

	return totalDueCount, nil
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
	} else {
		query = `
			DELETE FROM card_progress
			WHERE user_id = ?
		`
		args = []interface{}{userID}
	}

	_, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("error resetting card progress: %w", err)
	}

	return nil
}
