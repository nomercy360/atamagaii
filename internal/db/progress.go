package db

import (
	"errors"
	"fmt"
	nanoid "github.com/matoous/go-nanoid/v2"
	"math"
	"time"
)

// Constants for the SuperMemo SM-2 algorithm
const (
	MinEase     = 1.3
	DefaultEase = 2.5
	EaseModHard = 0.15
	EaseModGood = 0.0
	EaseModEasy = 0.15
)

// Review represents a card review event
type Review struct {
	ID           string    `db:"id" json:"id"`
	UserID       string    `db:"user_id" json:"user_id"`
	CardID       string    `db:"card_id" json:"card_id"`
	Rating       int       `db:"rating" json:"rating"` // 1=Again, 2=Hard, 3=Good, 4=Easy
	ReviewedAt   time.Time `db:"reviewed_at" json:"reviewed_at"`
	TimeSpentMs  int       `db:"time_spent_ms" json:"time_spent_ms"`
	PrevInterval int       `db:"prev_interval" json:"prev_interval"`
	NewInterval  int       `db:"new_interval" json:"new_interval"`
	PrevEase     float64   `db:"prev_ease" json:"prev_ease"`
	NewEase      float64   `db:"new_ease" json:"new_ease"`
}

// CardProgress tracks a user's progress with a specific card
type CardProgress struct {
	UserID          string     `db:"user_id" json:"user_id"`
	CardID          string     `db:"card_id" json:"card_id"`
	NextReview      *time.Time `db:"next_review" json:"next_review,omitempty"`
	Interval        int        `db:"interval" json:"interval"`         // in days
	Ease            float64    `db:"ease" json:"ease"`                 // SM-2 ease factor
	ReviewCount     int        `db:"review_count" json:"review_count"` // total reviews
	LapsCount       int        `db:"laps_count" json:"laps_count"`     // times forgotten
	LastReviewedAt  *time.Time `db:"last_reviewed_at" json:"last_reviewed_at,omitempty"`
	FirstReviewedAt *time.Time `db:"first_reviewed_at" json:"first_reviewed_at,omitempty"` // when this card was first studied
}

// ReviewCard processes a card review using the SM-2 spaced repetition algorithm
func (s *Storage) ReviewCard(userID, cardID string, rating int, timeSpentMs int) error {
	if rating < 1 || rating > 4 {
		return errors.New("invalid rating: must be between 1 and 4")
	}

	// Get current progress or create new one
	var progress CardProgress
	query := `
		SELECT user_id, card_id, next_review, interval, ease, review_count, laps_count, last_reviewed_at, first_reviewed_at
		FROM card_progress 
		WHERE user_id = ? AND card_id = ?
	`

	err := s.db.QueryRow(query, userID, cardID).Scan(
		&progress.UserID,
		&progress.CardID,
		&progress.NextReview,
		&progress.Interval,
		&progress.Ease,
		&progress.ReviewCount,
		&progress.LapsCount,
		&progress.LastReviewedAt,
		&progress.FirstReviewedAt,
	)

	isNew := false
	if err != nil {
		// No existing progress, create new
		isNew = true
		progress = CardProgress{
			UserID: userID,
			CardID: cardID,
			Ease:   DefaultEase,
		}
	}

	// Record previous values for the review history
	prevInterval := progress.Interval
	prevEase := progress.Ease
	now := time.Now()

	// Update card progress
	progress.ReviewCount++
	progress.LastReviewedAt = &now

	// Calculate new interval and ease based on rating
	// This implements a simplified version of the SM-2 algorithm
	if rating == 1 { // Again (Fail)
		progress.Interval = 1 // Reset to 1 day
		progress.LapsCount++
		progress.Ease = math.Max(progress.Ease-0.2, MinEase) // Decrease ease
	} else {
		// Apply SM-2 algorithm
		if progress.Interval == 0 {
			// First review
			switch rating {
			case 2: // Hard
				progress.Interval = 1
			case 3: // Good
				progress.Interval = 3
			case 4: // Easy
				progress.Interval = 7
			}
		} else {
			// Apply ease modifications
			switch rating {
			case 2: // Hard
				progress.Ease = math.Max(progress.Ease-EaseModHard, MinEase)
				progress.Interval = int(float64(progress.Interval) * 1.2)
			case 3: // Good
				progress.Interval = int(float64(progress.Interval) * progress.Ease)
			case 4: // Easy
				progress.Ease += EaseModEasy
				progress.Interval = int(float64(progress.Interval) * progress.Ease * 1.3)
			}
		}
	}

	// Cap very long intervals at 365 days to prevent excessive intervals
	if progress.Interval > 365 {
		progress.Interval = 365
	}

	// Calculate next review date
	nextReview := now.AddDate(0, 0, progress.Interval)
	progress.NextReview = &nextReview

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
		prevInterval,
		progress.Interval,
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
		// For new card progress entries, set first_reviewed_at to now
		progress.FirstReviewedAt = &now

		progressQuery = `
			INSERT INTO card_progress (user_id, card_id, next_review, interval, ease, review_count, laps_count, last_reviewed_at, first_reviewed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		args = []interface{}{
			userID,
			cardID,
			progress.NextReview,
			progress.Interval,
			progress.Ease,
			progress.ReviewCount,
			progress.LapsCount,
			progress.LastReviewedAt,
			progress.FirstReviewedAt,
		}
	} else {
		progressQuery = `
			UPDATE card_progress
			SET next_review = ?, interval = ?, ease = ?, review_count = ?, laps_count = ?, last_reviewed_at = ?
			WHERE user_id = ? AND card_id = ?
		`
		args = []interface{}{
			progress.NextReview,
			progress.Interval,
			progress.Ease,
			progress.ReviewCount,
			progress.LapsCount,
			progress.LastReviewedAt,
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

// GetCardProgress gets the progress for a specific card
func (s *Storage) GetCardProgress(userID, cardID string) (*CardProgress, error) {
	query := `
		SELECT user_id, card_id, next_review, interval, ease, review_count, laps_count, last_reviewed_at, first_reviewed_at
		FROM card_progress
		WHERE user_id = ? AND card_id = ?
	`

	var progress CardProgress
	err := s.db.QueryRow(query, userID, cardID).Scan(
		&progress.UserID,
		&progress.CardID,
		&progress.NextReview,
		&progress.Interval,
		&progress.Ease,
		&progress.ReviewCount,
		&progress.LapsCount,
		&progress.LastReviewedAt,
		&progress.FirstReviewedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("error getting card progress: %w", err)
	}

	return &progress, nil
}

func (s *Storage) GetDueCardCount(userID string) (int, error) {
	decks, err := s.GetDecks(userID)
	if err != nil {
		return 0, fmt.Errorf("error fetching decks for due card counting: %w", err)
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
