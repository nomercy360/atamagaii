package db

import (
	"database/sql"
	"errors"
	"fmt"
	nanoid "github.com/matoous/go-nanoid/v2"
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

func (s *Storage) GetNewCards(userID string, deckID string, limit int) ([]Card, error) {
	deck, err := s.GetDeck(deckID)
	if err != nil {
		return nil, fmt.Errorf("error getting deck settings: %w", err)
	}

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
	err = s.db.QueryRow(countNewStartedTodayQuery, userID, deckID, today).Scan(&newCardsStartedToday)
	if err != nil {
		return nil, fmt.Errorf("error counting new cards started today: %w", err)
	}

	newCardsRemaining := deck.NewCardsPerDay - newCardsStartedToday
	if newCardsRemaining < 0 {
		newCardsRemaining = 0
	}

	if newCardsRemaining == 0 {
		return []Card{}, nil
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

	rows, err := s.db.Query(query, userID, deckID, newCardsRemaining)
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

func (s *Storage) GetCardsForReview(userID string, deckID string, limit int) ([]Card, error) {
	reviewCards, err := s.GetDueCards(userID, deckID, limit)
	if err != nil {
		return nil, fmt.Errorf("error getting review cards: %w", err)
	}

	if len(reviewCards) >= limit {
		return reviewCards[:limit], nil
	}

	newCardsNeeded := limit - len(reviewCards)

	newCards, err := s.GetNewCards(userID, deckID, newCardsNeeded)
	if err != nil {
		return nil, fmt.Errorf("error getting new cards: %w", err)
	}

	combinedCards := append(reviewCards, newCards...)

	return combinedCards, nil
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
