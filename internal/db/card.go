package db

import (
	"database/sql"
	"errors"
	"fmt"
	nanoid "github.com/matoous/go-nanoid/v2"
	"time"
)

type Card struct {
	ID        string     `db:"id" json:"id"`
	DeckID    string     `db:"deck_id" json:"deck_id"`
	Fields    string     `db:"fields" json:"fields"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

type CardWithProgress struct {
	Card
	NextReview      *time.Time    `db:"next_review" json:"next_review,omitempty"`
	Interval        time.Duration `db:"interval" json:"interval,omitempty"`
	Ease            *float64      `db:"ease" json:"ease,omitempty"`
	ReviewCount     *int          `db:"review_count" json:"review_count,omitempty"`
	LapsCount       *int          `db:"laps_count" json:"laps_count,omitempty"`
	LastReviewedAt  *time.Time    `db:"last_reviewed_at" json:"last_reviewed_at,omitempty"`
	FirstReviewedAt *time.Time    `db:"first_reviewed_at" json:"first_reviewed_at,omitempty"`
	State           string        `db:"state" json:"state,omitempty"`
	LearningStep    int           `db:"learning_step" json:"learning_step,omitempty"`
}

type VocabularyItem struct {
	Kanji       string    `json:"kanji"`
	Kana        string    `json:"kana"`
	Translation string    `json:"translation"`
	Examples    []Example `json:"examples"`
	Level       string    `json:"level"`
	AudioURL    string    `json:"audio_url"`
}

type Example struct {
	Sentence    []Fragment `json:"sentence"`
	Translation string     `json:"translation"`
	AudioURL    string     `json:"audio_url"`
}

type Fragment struct {
	Fragment string  `json:"fragment"`
	Furigana *string `json:"furigana"`
}

func (s *Storage) AddCard(deckID, fields string) (*Card, error) {
	cardID := nanoid.Must()
	now := time.Now()

	query := `
		INSERT INTO cards (id, deck_id, fields, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query, cardID, deckID, fields, now, now)
	if err != nil {
		return nil, fmt.Errorf("error adding card: %w", err)
	}

	return &Card{
		ID:        cardID,
		DeckID:    deckID,
		Fields:    fields,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (s *Storage) AddCardsInBatch(deckID string, fieldsArray []string) error {
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
		INSERT INTO cards (id, deck_id, fields, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("error preparing statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for i, fields := range fieldsArray {
		cardID := nanoid.Must()
		_, err = stmt.Exec(cardID, deckID, fields, now, now)
		if err != nil {
			return fmt.Errorf("error inserting card %d: %w", i, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

func (s *Storage) GetNewCards(userID string, deckID string, limit int) ([]CardWithProgress, error) {
	deck, err := s.GetDeck(deckID)
	if err != nil {
		return nil, fmt.Errorf("error getting deck settings: %w", err)
	}

	today := time.Now().Truncate(24 * time.Hour)

	countNewStartedTodayQuery := `
		SELECT COUNT(*) 
		FROM card_progress p
		JOIN cards c ON p.card_id = c.id
		WHERE p.user_id = ? 
		AND c.deck_id = ?
		AND c.deleted_at IS NULL
		AND p.first_reviewed_at >= ?
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
		return []CardWithProgress{}, nil
	}

	query := `
		SELECT c.id,
		       c.deck_id,
		       c.fields,
		       c.created_at,
		       c.updated_at,
		       c.deleted_at
		FROM cards c
		LEFT JOIN card_progress p ON c.id = p.card_id AND p.user_id = ?
		JOIN decks d ON c.deck_id = d.id AND d.user_id = ?
		WHERE c.deck_id = ? AND c.deleted_at IS NULL
		AND p.card_id IS NULL
		ORDER BY c.id
		LIMIT ?
	`

	rows, err := s.db.Query(query, userID, userID, deckID, newCardsRemaining)
	if err != nil {
		return nil, fmt.Errorf("error getting new cards: %w", err)
	}
	defer rows.Close()

	var cards []CardWithProgress
	for rows.Next() {
		var card CardWithProgress
		if err := rows.Scan(
			&card.ID,
			&card.DeckID,
			&card.Fields,
			&card.CreatedAt,
			&card.UpdatedAt,
			&card.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning new card: %w", err)
		}
		cards = append(cards, card)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating new card rows: %w", err)
	}

	return cards, nil
}

func (s *Storage) GetDueCards(userID string, deckID string, limit int) ([]CardWithProgress, error) {
	todayEnd := time.Now().Truncate(24 * time.Hour).Add(24*time.Hour - time.Nanosecond)

	query := `
		SELECT c.id, c.deck_id, c.fields, c.created_at, c.updated_at, c.deleted_at,
		       p.next_review, p.interval, p.ease, p.review_count, p.laps_count, p.last_reviewed_at, p.first_reviewed_at,
		       p.state, p.learning_step
		FROM cards c
		JOIN card_progress p ON c.id = p.card_id AND p.user_id = ?
		JOIN decks d ON c.deck_id = d.id AND d.user_id = ?
		WHERE c.deck_id = ? AND c.deleted_at IS NULL
		AND p.next_review IS NOT NULL AND p.next_review <= ?
		ORDER BY p.next_review ASC
		LIMIT ?
	`

	rows, err := s.db.Query(query, userID, userID, deckID, todayEnd, limit)
	if err != nil {
		return nil, fmt.Errorf("error getting due cards: %w", err)
	}
	defer rows.Close()

	var cards []CardWithProgress
	for rows.Next() {
		var card CardWithProgress
		var intervalNs int64
		var state string
		var learningStep int
		if err := rows.Scan(
			&card.ID,
			&card.DeckID,
			&card.Fields,
			&card.CreatedAt,
			&card.UpdatedAt,
			&card.DeletedAt,
			&card.NextReview,
			&intervalNs,
			&card.Ease,
			&card.ReviewCount,
			&card.LapsCount,
			&card.LastReviewedAt,
			&card.FirstReviewedAt,
			&state,
			&learningStep,
		); err != nil {
			return nil, fmt.Errorf("error scanning due card: %w", err)
		}

		card.Interval = time.Duration(intervalNs)
		card.State = state
		card.LearningStep = learningStep

		cards = append(cards, card)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating due card rows: %w", err)
	}

	return cards, nil
}

func (s *Storage) GetReviewCards(userID string, deckID string, limit int) ([]CardWithProgress, error) {
	return s.GetDueCards(userID, deckID, limit)
}

func (s *Storage) GetCardsWithProgress(userID string, deckID string, limit int) ([]CardWithProgress, error) {
	reviewCards, err := s.GetReviewCards(userID, deckID, limit)
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

func (s *Storage) GetCard(cardID string) (*Card, error) {
	query := `
		SELECT id, deck_id, fields, created_at, updated_at, deleted_at
		FROM cards
		WHERE id = ? AND deleted_at IS NULL
	`

	var card Card
	err := s.db.QueryRow(query, cardID).Scan(
		&card.ID,
		&card.DeckID,
		&card.Fields,
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

	return &card, nil
}
