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

// Constants for the SRS algorithm (Anki SM-2 inspired)
const (
	MinEase     float64 = 1.3 // Minimum ease factor
	MaxEase     float64 = 2.5 // Maximum ease factor
	DefaultEase float64 = 2.5 // Initial ease factor for new cards

	// Ease factor adjustments based on rating
	EaseAdjAgain float64 = -0.20 // Penalty for "Again"
	EaseAdjHard  float64 = -0.15 // Penalty for "Hard"
	EaseAdjGood  float64 = 0.0   // No change for "Good" (SM-2 standard)
	EaseAdjEasy  float64 = +0.15 // Bonus for "Easy"

	// Interval multipliers
	HardIntervalMultiplier      float64 = 1.2 // For "Hard" in review: Interval_new = Interval_old * HardIntervalMultiplier
	EasyBonusIntervalMultiplier float64 = 1.3 // For "Easy" in review: Interval_new = Interval_old * Ease * EasyBonusIntervalMultiplier

	MaxReviewIntervalDays int = 365 // Maximum interval in days

	// Learning steps (durations) - used for initial learning and relearning
	LearningStep1Duration time.Duration = 1 * time.Minute
	LearningStep2Duration time.Duration = 10 * time.Minute
	// Number of learning/relearning steps before graduation (e.g., 2 steps: 0 and 1)
	TotalLearningSteps int = 2

	// Graduation intervals (when moving from Learning/Relearning to Review state)
	GraduateToReviewIntervalGoodDays float64 = 1.0 // Days, for "Good"
	GraduateToReviewIntervalEasyDays float64 = 4.0 // Days, for "Easy" from initial learning
	// Special interval for "Easy" out of relearning, often shorter to confirm retention
	GraduateFromRelearningToReviewIntervalEasyDays float64 = 2.0 // Days

	FuzzPercentage float64 = 0.05 // 5% fuzz for review intervals > 1 day
)

type CardState string

const (
	StateNew        CardState = "new"
	StateLearning   CardState = "learning"
	StateReview     CardState = "review"
	StateRelearning CardState = "relearning"
)

// Review represents a card review event (structure seems fine)
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

// CardProgress represents the learning progress of a card (structure seems fine)
type CardProgress struct {
	UserID          string        `db:"user_id" json:"user_id"`
	CardID          string        `db:"card_id" json:"card_id"`
	NextReview      *time.Time    `db:"next_review" json:"next_review,omitempty"`
	Interval        time.Duration `db:"interval" json:"interval"`         // Stored as nanoseconds, but represents logical interval
	Ease            float64       `db:"ease" json:"ease"`                 // Anki-like ease factor
	ReviewCount     int           `db:"review_count" json:"review_count"` // Total reviews (all ratings)
	LapsCount       int           `db:"laps_count" json:"laps_count"`     // Times forgotten (rated "Again" in Review/Relearning)
	LastReviewedAt  *time.Time    `db:"last_reviewed_at" json:"last_reviewed_at,omitempty"`
	FirstReviewedAt *time.Time    `db:"first_reviewed_at" json:"first_reviewed_at,omitempty"`
	State           CardState     `db:"state" json:"state"`                 // new, learning, review, relearning
	LearningStep    int           `db:"learning_step" json:"learning_step"` // Tracks current learning/relearning step (0-indexed)
}

func (s *Storage) ReviewCard(userID, cardID string, rating int, timeSpentMs int) error {
	if rating < 1 || rating > 4 {
		return errors.New("invalid rating: must be between 1 and 4")
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
			LearningStep: 0, // Start at step 0
			Interval:     0, // Will be set by first review
		}
	} else if err != nil {
		return fmt.Errorf("error fetching card progress: %w", err)
	}

	now := time.Now()
	prevInterval := progress.Interval
	prevEase := progress.Ease
	currentState := progress.State // Capture state before modification for logic

	// --- Handle State: New or Learning ---
	if currentState == StateNew || currentState == StateLearning {
		progress.State = StateLearning // Ensure state is learning
		switch rating {
		case 1: // Again
			progress.LearningStep = 0 // Reset to first learning step
			progress.Interval = LearningStep1Duration
			progress.Ease = math.Max(MinEase, progress.Ease+EaseAdjAgain)
		case 2: // Hard
			// For learning phase, "Hard" often behaves like "Again" or repeats current step with penalty.
			// Simpler: reset to first step, like "Again", but with potentially different ease penalty.
			progress.LearningStep = 0
			progress.Interval = LearningStep1Duration
			progress.Ease = math.Max(MinEase, progress.Ease+EaseAdjHard)
		case 3: // Good
			progress.LearningStep++
			progress.Ease = math.Max(MinEase, progress.Ease+EaseAdjGood) // Usually no change or slight positive
			if progress.LearningStep == 1 {                              // Advanced from step 0 to step 1
				progress.Interval = LearningStep2Duration
			} else { // Graduated from all learning steps (e.g., step 1 to graduation)
				progress.State = StateReview
				progress.Interval = time.Duration(GraduateToReviewIntervalGoodDays*24) * time.Hour
				progress.LearningStep = 0 // Reset for potential future relearning
			}
		case 4: // Easy
			progress.State = StateReview // Graduate immediately
			progress.Interval = time.Duration(GraduateToReviewIntervalEasyDays*24) * time.Hour
			progress.Ease = math.Min(MaxEase, progress.Ease+EaseAdjEasy)
			progress.LearningStep = 0 // Reset for potential future relearning
		}
	} else if currentState == StateReview {
		// --- Handle State: Review ---
		switch rating {
		case 1: // Again (Lapsed)
			progress.State = StateRelearning
			progress.LapsCount++
			progress.Ease = math.Max(MinEase, progress.Ease+EaseAdjAgain)
			progress.LearningStep = 0 // Start relearning steps
			progress.Interval = LearningStep1Duration
		case 2: // Hard
			progress.Ease = math.Max(MinEase, progress.Ease+EaseAdjHard)
			// Interval_new = Interval_old * HardIntervalMultiplier
			// Ensure interval is at least 1 day
			currentIntervalSec := math.Max(progress.Interval.Seconds(), 24*3600) // Min 1 day for calc base
			newIntervalSec := currentIntervalSec * HardIntervalMultiplier
			progress.Interval = time.Duration(newIntervalSec) * time.Second
		case 3: // Good
			progress.Ease = math.Max(MinEase, progress.Ease+EaseAdjGood)
			// Interval_new = Interval_old * Ease
			// Ensure interval is at least 1 day
			currentIntervalSec := math.Max(progress.Interval.Seconds(), 24*3600) // Min 1 day for calc base
			newIntervalSec := currentIntervalSec * progress.Ease
			progress.Interval = time.Duration(newIntervalSec) * time.Second
		case 4: // Easy
			progress.Ease = math.Min(MaxEase, progress.Ease+EaseAdjEasy)
			// Interval_new = Interval_old * Ease * EasyBonusIntervalMultiplier
			// Ensure interval is at least 1 day
			currentIntervalSec := math.Max(progress.Interval.Seconds(), 24*3600) // Min 1 day for calc base
			newIntervalSec := currentIntervalSec * progress.Ease * EasyBonusIntervalMultiplier
			progress.Interval = time.Duration(newIntervalSec) * time.Second
		}
	} else if currentState == StateRelearning {
		// --- Handle State: Relearning ---
		// (Similar to learning, but graduation intervals might differ, and ease already penalized on lapse)
		switch rating {
		case 1: // Again
			progress.LearningStep = 0 // Reset to first relearning step
			progress.Interval = LearningStep1Duration
			// Ease already penalized when lapsed. Further penalty could be too much,
			// but some systems do apply a smaller one here or just reset step.
			// progress.Ease = math.Max(MinEase, progress.Ease + EaseAdjAgain * 0.5) // Optional: smaller hit
		case 2: // Hard
			progress.LearningStep = 0 // Reset to first relearning step (or repeat current)
			progress.Interval = LearningStep1Duration
			// progress.Ease = math.Max(MinEase, progress.Ease + EaseAdjHard * 0.5) // Optional
		case 3: // Good
			progress.LearningStep++
			if progress.LearningStep == 1 { // Advanced from relearning step 0 to step 1
				progress.Interval = LearningStep2Duration
			} else { // Graduated from all relearning steps
				progress.State = StateReview
				progress.Interval = time.Duration(GraduateToReviewIntervalGoodDays*24) * time.Hour // Standard good graduation
				progress.LearningStep = 0
			}
		case 4: // Easy
			progress.State = StateReview // Graduate immediately from relearning
			progress.Interval = time.Duration(GraduateFromRelearningToReviewIntervalEasyDays*24) * time.Hour
			progress.Ease = math.Min(MaxEase, progress.Ease+EaseAdjEasy) // Give an ease boost
			progress.LearningStep = 0
		}
	}

	// --- Common adjustments for intervals (Review State) ---
	if progress.State == StateReview {
		// Ensure minimum interval of 1 day for review cards if calculation resulted in less
		// (except for cards just graduating to 1 day).
		// The calculations for Hard/Good/Easy in Review already use a 1-day base if current interval is less.
		// So, this explicit check might only be needed if those calculations could yield <1 day from a >1 day base.
		if progress.Interval < 24*time.Hour {
			isRecentGraduation := (currentState == StateLearning || currentState == StateRelearning) &&
				(progress.Interval == time.Duration(GraduateToReviewIntervalGoodDays*24)*time.Hour)
			if !isRecentGraduation { // Don't override a fresh 1-day graduation interval upwards
				progress.Interval = 24 * time.Hour
			}
		}

		// Apply fuzz factor for intervals > 1 day to prevent clumping
		if progress.Interval > 24*time.Hour && FuzzPercentage > 0 {
			fuzzAmount := time.Duration(float64(progress.Interval.Nanoseconds()) * FuzzPercentage)
			if fuzzAmount > 0 {
				// Generate random offset: [-fuzzAmount, +fuzzAmount)
				randomOffset := time.Duration(rand.Int63n(2*fuzzAmount.Nanoseconds()) - fuzzAmount.Nanoseconds())
				progress.Interval += randomOffset
				// Ensure interval didn't become too short due to negative fuzz
				if progress.Interval < 24*time.Hour {
					progress.Interval = 24 * time.Hour
				}
			}
		}

		// Cap interval at maximum
		maxIntervalDuration := time.Duration(MaxReviewIntervalDays) * 24 * time.Hour
		if progress.Interval > maxIntervalDuration {
			progress.Interval = maxIntervalDuration
		}
	}

	// Set next review time
	// For learning/relearning steps (short intervals), add directly.
	// For review intervals (days), it's good practice to schedule for the start of the day
	// or ensure it's at least 'now + interval'. Here, 'now + interval' is used.
	nextReviewTime := now.Add(progress.Interval)
	progress.NextReview = &nextReviewTime

	// Update general progress fields
	progress.ReviewCount++
	progress.LastReviewedAt = &now
	if progress.FirstReviewedAt == nil || isNew { // Ensure first reviewed is set for new cards
		progress.FirstReviewedAt = &now
	}

	// --- Database Operations (Transaction) ---
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if not committed

	// Create review history entry
	reviewQuery := `
		INSERT INTO reviews (id, user_id, card_id, rating, reviewed_at, time_spent_ms, prev_interval, new_interval, prev_ease, new_ease)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = tx.Exec(reviewQuery,
		nanoid.Must(), // Consider error handling for nanoid if strict
		userID,
		cardID,
		rating,
		now,
		timeSpentMs,
		prevInterval.String(),      // Store as string
		progress.Interval.String(), // Store as string
		prevEase,
		progress.Ease,
	)
	if err != nil {
		return fmt.Errorf("error creating review: %w", err)
	}

	// Update or insert progress
	if isNew {
		progressQuery := `
			INSERT INTO card_progress (user_id, card_id, next_review, interval, ease, review_count, laps_count, last_reviewed_at, first_reviewed_at, state, learning_step)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		_, err = tx.Exec(progressQuery,
			userID, cardID, progress.NextReview, progress.Interval.String(), progress.Ease,
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
			progress.NextReview, progress.Interval.String(), progress.Ease, progress.ReviewCount,
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
