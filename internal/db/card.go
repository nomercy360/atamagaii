package db

import (
	"database/sql"
	"errors"
	"fmt"
	nanoid "github.com/matoous/go-nanoid/v2"
	"math"
	"sort"
	"time"
)

type Card struct {
	ID              string        `db:"id" json:"id"`
	DeckID          string        `db:"deck_id" json:"deck_id"`
	Fields          string        `db:"fields" json:"fields"`
	UserID          string        `db:"user_id" json:"user_id"`
	NextReview      *time.Time    `db:"next_review" json:"next_review,omitempty"`
	Interval        time.Duration `db:"interval" json:"interval"`
	Ease            float64       `db:"ease" json:"ease"`
	ReviewCount     int           `db:"review_count" json:"review_count"`
	LapsCount       int           `db:"laps_count" json:"laps_count"`
	LastReviewedAt  *time.Time    `db:"last_reviewed_at" json:"last_reviewed_at,omitempty"`
	FirstReviewedAt *time.Time    `db:"first_reviewed_at" json:"first_reviewed_at,omitempty"`
	State           string        `db:"state" json:"state"`
	LearningStep    int           `db:"learning_step" json:"learning_step"`
	CreatedAt       time.Time     `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time     `db:"updated_at" json:"updated_at"`
	DeletedAt       *time.Time    `db:"deleted_at" json:"deleted_at,omitempty"`
}
type VocabularyItem struct {
	Term                  string `json:"term"`                    // Primary term in native script
	Transcription         string `json:"transcription,omitempty"` // Reading aid (pinyin, romaji, etc.)
	TermWithTranscription string `json:"term_with_transcription"` // Term with reading aids embedded (was WordFurigana)

	// Meanings in different languages
	MeaningEn string `json:"meaning_en,omitempty"` // English translation
	MeaningRu string `json:"meaning_ru,omitempty"` // Russian translation

	// Example sentences
	ExampleNative            string `json:"example_native"`                       // Example in native script (was ExampleJa)
	ExampleWithTranscription string `json:"example_with_transcription,omitempty"` // Example with reading aids (was ExampleFurigana)
	ExampleEn                string `json:"example_en,omitempty"`                 // English example translation
	ExampleRu                string `json:"example_ru,omitempty"`                 // Russian example translation

	// Metadata
	Frequency         int    `json:"frequency,omitempty"`          // Usage frequency data
	LanguageCode      string `json:"language_code"`                // ISO 639-1 language code (e.g., "ja", "zh", "en")
	TranscriptionType string `json:"transcription_type,omitempty"` // Type of transcription (furigana, pinyin, etc.)

	// Media
	AudioWord    string `json:"audio_word,omitempty"`    // Audio for term pronunciation
	AudioExample string `json:"audio_example,omitempty"` // Audio for example sentence
	ImageURL     string `json:"image_url,omitempty"`     // Illustration image
}

func (s *Storage) AddCard(userID, deckID, fields string) (*Card, error) {
	cardID := nanoid.Must()
	now := time.Now()

	query := `
		INSERT INTO cards (
			id, deck_id, fields, user_id, ease, review_count, laps_count,
			learning_step, state, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(
		query,
		cardID,
		deckID,
		fields,
		userID,
		DefaultEase,
		0,     // review_count
		0,     // laps_count
		0,     // learning_step
		"new", // state
		now,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("error adding card: %w", err)
	}

	return &Card{
		ID:           cardID,
		DeckID:       deckID,
		Fields:       fields,
		UserID:       userID,
		Ease:         DefaultEase,
		ReviewCount:  0,
		LapsCount:    0,
		State:        "new",
		LearningStep: 0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (s *Storage) AddCardsInBatch(userID, deckID string, fieldsArray []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.Prepare(`
		INSERT INTO cards (
			id, deck_id, fields, user_id, ease, review_count, laps_count,
			learning_step, state, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("error preparing statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for i, fields := range fieldsArray {
		cardID := nanoid.Must()
		_, err = stmt.Exec(
			cardID,
			deckID,
			fields,
			userID,
			DefaultEase,
			0,     // review_count
			0,     // laps_count
			0,     // learning_step
			"new", // state
			now,
			now,
		)
		if err != nil {
			return fmt.Errorf("error inserting card %d: %w", i, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

func (s *Storage) GetNewCards(userID string, deckID string, limit, limitPerDay int) ([]Card, error) {
	today := time.Now().Truncate(24 * time.Hour)

	countNewStartedTodayQuery := `
		SELECT COUNT(*)
		FROM cards c
		WHERE c.user_id = ?
		AND c.deck_id = ?
		AND c.deleted_at IS NULL
		AND c.first_reviewed_at >= ?
	`

	var newCardsStartedToday int
	err := s.db.QueryRow(countNewStartedTodayQuery, userID, deckID, today).Scan(&newCardsStartedToday)
	if err != nil {
		return nil, fmt.Errorf("error counting new cards started today: %w", err)
	}

	if newCardsStartedToday >= limitPerDay {
		return nil, nil
	}

	remainingNewCards := limitPerDay - newCardsStartedToday
	if remainingNewCards > limit {
		remainingNewCards = limit
	}

	query := `
		SELECT id, deck_id, fields, user_id, next_review, interval, ease,
		       review_count, laps_count, last_reviewed_at, first_reviewed_at, state,
		       learning_step, created_at, updated_at, deleted_at
		FROM cards c
		WHERE c.user_id = ?
		AND c.deck_id = ?
		AND c.deleted_at IS NULL
		AND c.state = 'new'
		ORDER BY c.created_at ASC
		LIMIT ?
	`

	rows, err := s.db.Query(query, userID, deckID, remainingNewCards)
	if err != nil {
		return nil, fmt.Errorf("error getting new cards: %w", err)
	}
	defer rows.Close()

	var cards []Card
	for rows.Next() {
		var card Card
		var intervalNs int64

		if err := rows.Scan(
			&card.ID,
			&card.DeckID,
			&card.Fields,
			&card.UserID,
			&card.NextReview,
			&intervalNs,
			&card.Ease,
			&card.ReviewCount,
			&card.LapsCount,
			&card.LastReviewedAt,
			&card.FirstReviewedAt,
			&card.State,
			&card.LearningStep,
			&card.CreatedAt,
			&card.UpdatedAt,
			&card.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning new card: %w", err)
		}

		card.Interval = time.Duration(intervalNs)
		cards = append(cards, card)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating new card rows: %w", err)
	}

	return cards, nil
}

func (s *Storage) GetDueCardCount(userID string) (int, error) {
	todayEnd := time.Now().Truncate(24 * time.Hour).Add(24*time.Hour - time.Nanosecond)
	today := time.Now().Truncate(24 * time.Hour)

	// Query to count learning, review, and new cards available for study today
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN (state = 'learning' OR state = 'relearning') AND next_review <= ? THEN 1 ELSE 0 END), 0) +
			COALESCE(SUM(CASE WHEN state = 'review' AND next_review <= ? THEN 1 ELSE 0 END), 0) +
			COALESCE((
				SELECT COUNT(*) FROM (
					SELECT id FROM cards
					WHERE user_id = ? AND deleted_at IS NULL
					AND state = 'new'
					AND (first_reviewed_at IS NULL OR first_reviewed_at < ?)
					LIMIT (
						SELECT SUM(new_cards_per_day) - (
							SELECT COUNT(*) FROM cards
							WHERE user_id = ? AND deleted_at IS NULL
							AND first_reviewed_at >= ?
						) FROM decks WHERE user_id = ? AND deleted_at IS NULL
					)
				) as new_count
			), 0) as total_due_count
		FROM cards
		WHERE user_id = ? AND deleted_at IS NULL
	`

	var count int
	err := s.db.QueryRow(query, todayEnd, todayEnd, userID, today, userID, today, userID, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error getting due card count: %w", err)
	}

	return count, nil
}

func (s *Storage) GetDueCards(userID string, deckID string, limit int) ([]Card, error) {
	todayEnd := time.Now().Truncate(24 * time.Hour).Add(24*time.Hour - time.Nanosecond)
	// todayEnd := time.Now()

	query := `
		SELECT id, deck_id, fields, user_id, next_review, interval, ease,
		       review_count, laps_count, last_reviewed_at, first_reviewed_at, state,
		       learning_step, created_at, updated_at, deleted_at
		FROM cards c
		WHERE c.user_id = ?
		AND c.deck_id = ?
		AND c.deleted_at IS NULL
		AND c.next_review IS NOT NULL
		AND c.next_review <= ?
		ORDER BY c.next_review ASC
		LIMIT ?
	`

	rows, err := s.db.Query(query, userID, deckID, todayEnd, limit)
	if err != nil {
		return nil, fmt.Errorf("error getting due cards: %w", err)
	}
	defer rows.Close()

	var cards []Card
	for rows.Next() {
		var card Card
		var intervalNs int64

		if err := rows.Scan(
			&card.ID,
			&card.DeckID,
			&card.Fields,
			&card.UserID,
			&card.NextReview,
			&intervalNs,
			&card.Ease,
			&card.ReviewCount,
			&card.LapsCount,
			&card.LastReviewedAt,
			&card.FirstReviewedAt,
			&card.State,
			&card.LearningStep,
			&card.CreatedAt,
			&card.UpdatedAt,
			&card.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning due card: %w", err)
		}

		card.Interval = time.Duration(intervalNs)
		cards = append(cards, card)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating due card rows: %w", err)
	}

	return cards, nil
}

func CalculatePreviewInterval(card Card, rating int) time.Duration {
	params, err := calculateNextReviewParameters(
		CardState(card.State),
		card.LearningStep,
		card.Interval, // Card's current interval
		card.Ease,     // Card's current ease
		rating,
	)
	if err != nil {
		fmt.Printf("Warning: calculatePreviewInterval failed for card ID %s (state: %s, rating: %d): %v. Returning default.\n", card.ID, card.State, rating, err)
		return LearningStep1Duration
	}
	return params.Interval
}

func (s *Storage) GetCardsForReview(
	userID string,
	deckID string,
	limit int,
	newCardsLimitForDay int,
) ([]Card, error) {
	reviewLimit := newCardsLimitForDay * 10

	reviewCards, err := s.GetDueCards(userID, deckID, reviewLimit)
	if err != nil {
		return nil, fmt.Errorf("error getting review cards: %w", err)
	}

	if len(reviewCards) >= limit {
		SortCardsForReview(reviewCards, time.Now())
		return reviewCards[:limit], nil
	}

	newCardsNeeded := limit - len(reviewCards)
	if newCardsNeeded <= 0 {
		return reviewCards, nil
	}

	newCards, err := s.GetNewCards(userID, deckID, newCardsNeeded, newCardsLimitForDay)
	if err != nil {
		return nil, fmt.Errorf("error getting new cards: %w", err)
	}

	combinedCards := append(reviewCards, newCards...)

	// Sort cards according to the spaced repetition algorithm priority
	SortCardsForReview(combinedCards, time.Now())

	return combinedCards, nil
}

func FormatSimpleDuration(d time.Duration) string {
	if d <= 0 {
		// For display, a 0 or negative interval after calculation (before fallback) might appear as a very short step.
		// After fallbacks in calculatePreviewInterval, d should be positive.
		// If it still somehow is <=0, show a minimal positive duration.
		return "1m" // Or "~1m", or based on LearningStep1Duration
	}

	seconds := int64(roundSeconds(d.Seconds()))
	minutes := int64(roundSeconds(d.Minutes()))
	hours := int64(roundSeconds(d.Hours()))
	days := d.Hours() / 24.0

	if days >= 365.0*0.95 { // About a year
		numYears := math.Round(days/365.0*10) / 10 // Rounded to 1 decimal place
		if numYears < 1.0 {
			numYears = 1.0
		}
		return fmt.Sprintf("%.1fy", numYears)
	}
	if days >= 30.0*0.95 { // About a month
		numMonths := math.Round(days / 30.0)
		if numMonths < 1.0 {
			numMonths = 1.0
		}
		return fmt.Sprintf("%.0fmo", numMonths)
	}
	if days >= 1.0*0.95 { // About a day
		numDays := math.Round(days)
		if numDays < 1.0 {
			numDays = 1.0
		}
		return fmt.Sprintf("%.0fd", numDays)
	}
	if hours >= 1 {
		return fmt.Sprintf("%dh", hours)
	}
	if minutes >= 1 {
		return fmt.Sprintf("%dm", minutes)
	}
	if seconds < 1 {
		seconds = 1
	} // Ensure at least 1s for very short positive durations
	return fmt.Sprintf("%ds", seconds)
}

func roundSeconds(f float64) float64 {
	if f < 0 {
		return math.Ceil(f - 0.5)
	}
	return math.Floor(f + 0.5)
}

// SortCardsForReview sorts a slice of cards according to spaced repetition algorithm priority:
// 1. Learning/Relearning cards first with next_review_time >= referenceTime
// 2. Then review cards
// 3. Then new cards
// 4. Finally, learning/relearning cards with next_review_time < referenceTime
// Within each category, cards are sorted by next review time or creation date
func SortCardsForReview(cards []Card, referenceTime time.Time) {
	// Define sorting priority
	sort.SliceStable(cards, func(i, j int) bool {
		a, b := cards[i], cards[j]

		category := func(c Card) int {
			switch CardState(c.State) {
			case StateLearning, StateRelearning:
				if c.NextReview != nil && c.NextReview.Before(referenceTime) {
					return 0 // Learning/relearning, due now or in the past
				}
				return 3 // Learning/relearning, but in the future
			case StateReview:
				return 1
			case StateNew:
				return 2
			default:
				return 4 // Fallback for unknown states
			}
		}

		catA := category(a)
		catB := category(b)
		if catA != catB {
			return catA < catB
		}

		// Sort by appropriate fields based on card state
		switch catA {
		case 0, 3: // Learning/relearning cards (due now or in future)
			// Sort by last_reviewed_at
			if a.LastReviewedAt == nil && b.LastReviewedAt == nil {
				return a.CreatedAt.Before(b.CreatedAt) // Fallback to creation time if both have no review time
			}
			if a.LastReviewedAt == nil {
				return true // Prioritize cards that haven't been reviewed yet
			}
			if b.LastReviewedAt == nil {
				return false // Prioritize cards that haven't been reviewed yet
			}
			return a.LastReviewedAt.Before(*b.LastReviewedAt) // Earlier reviewed first

		case 1: // Review cards
			// For review cards, sort by next_review time
			if a.NextReview == nil && b.NextReview == nil {
				return a.CreatedAt.Before(b.CreatedAt) // Fallback to creation time
			}
			if a.NextReview == nil {
				return true // Prioritize cards with no next review time
			}
			if b.NextReview == nil {
				return false // Prioritize cards with no next review time
			}
			return a.NextReview.Before(*b.NextReview) // Earlier due date first

		case 2: // New cards
			// Sort by created_at
			return a.CreatedAt.Before(b.CreatedAt) // Older cards first

		default:
			return false
		}
	})
}

func (s *Storage) GetCard(cardID string, userID string) (*Card, error) {
	query := `
		SELECT id, deck_id, fields, user_id, next_review, interval, ease,
		       review_count, laps_count, last_reviewed_at, first_reviewed_at, state,
		       learning_step, created_at, updated_at, deleted_at
		FROM cards
		WHERE id = ? AND user_id = ? AND deleted_at IS NULL
	`

	var card Card
	var intervalNs int64

	err := s.db.QueryRow(query, cardID, userID).Scan(
		&card.ID,
		&card.DeckID,
		&card.Fields,
		&card.UserID,
		&card.NextReview,
		&intervalNs,
		&card.Ease,
		&card.ReviewCount,
		&card.LapsCount,
		&card.LastReviewedAt,
		&card.FirstReviewedAt,
		&card.State,
		&card.LearningStep,
		&card.CreatedAt,
		&card.UpdatedAt,
		&card.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error getting card: %w", err)
	}

	card.Interval = time.Duration(intervalNs)
	return &card, nil
}

func (s *Storage) UpdateCardFields(cardID string, fields string) error {
	now := time.Now()
	query := `
		UPDATE cards
		SET fields = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	result, err := s.db.Exec(query, fields, now, cardID)
	if err != nil {
		return fmt.Errorf("error updating card fields: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking updated rows: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}
