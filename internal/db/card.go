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
	Front     string     `db:"front" json:"front"`
	Back      string     `db:"back" json:"back"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

type CardWithProgress struct {
	Card
	NextReview     *time.Time `db:"next_review" json:"next_review,omitempty"`
	Interval       *int       `db:"interval" json:"interval,omitempty"`
	Ease           *float64   `db:"ease" json:"ease,omitempty"`
	ReviewCount    *int       `db:"review_count" json:"review_count,omitempty"`
	LapsCount      *int       `db:"laps_count" json:"laps_count,omitempty"`
	LastReviewedAt *time.Time `db:"last_reviewed_at" json:"last_reviewed_at,omitempty"`
}

type VocabularyItem struct {
	Kanji       string    `json:"kanji"`
	Kana        string    `json:"kana"`
	Translation string    `json:"translation"`
	Examples    []Example `json:"examples"`
	Level       string    `json:"level"`
}

type Example struct {
	Sentence    []Fragment `json:"sentence"`
	Translation string     `json:"translation"`
}

type Fragment struct {
	Fragment string  `json:"fragment"`
	Furigana *string `json:"furigana"`
}

func (s *Storage) AddCard(deckID, front, back string) (*Card, error) {
	cardID := nanoid.Must()
	now := time.Now()

	query := `
		INSERT INTO cards (id, deck_id, front, back, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query, cardID, deckID, front, back, now, now)
	if err != nil {
		return nil, fmt.Errorf("error adding card: %w", err)
	}

	return &Card{
		ID:        cardID,
		DeckID:    deckID,
		Front:     front,
		Back:      back,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (s *Storage) AddCardsInBatch(deckID string, fronts, backs []string) error {
	if len(fronts) != len(backs) {
		return fmt.Errorf("number of fronts (%d) does not match number of backs (%d)", len(fronts), len(backs))
	}

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
		INSERT INTO cards (id, deck_id, front, back, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("error preparing statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for i := range fronts {
		cardID := nanoid.Must()
		_, err = stmt.Exec(cardID, deckID, fronts[i], backs[i], now, now)
		if err != nil {
			return fmt.Errorf("error inserting card %d: %w", i, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

func (s *Storage) GetCardsWithProgress(userID string, deckID string, limit int) ([]CardWithProgress, error) {
	query := `
		SELECT c.id, c.deck_id, c.front, c.back, c.created_at, c.updated_at, c.deleted_at,
		       p.next_review, p.interval, p.ease, p.review_count, p.laps_count, p.last_reviewed_at
		FROM cards c
		LEFT JOIN card_progress p ON c.id = p.card_id AND p.user_id = ?
		JOIN decks d ON c.deck_id = d.id AND d.user_id = ?
		WHERE c.deck_id = ? AND c.deleted_at IS NULL
		AND (p.next_review IS NULL OR p.next_review <= CURRENT_TIMESTAMP)
		ORDER BY p.next_review IS NULL DESC, p.next_review ASC
		LIMIT ?
	`
	rows, err := s.db.Query(query, userID, userID, deckID, limit)
	if err != nil {
		return nil, fmt.Errorf("error getting cards with progress: %w", err)
	}
	defer rows.Close()

	var cards []CardWithProgress
	for rows.Next() {
		var card CardWithProgress
		if err := rows.Scan(
			&card.ID,
			&card.DeckID,
			&card.Front,
			&card.Back,
			&card.CreatedAt,
			&card.UpdatedAt,
			&card.DeletedAt,
			&card.NextReview,
			&card.Interval,
			&card.Ease,
			&card.ReviewCount,
			&card.LapsCount,
			&card.LastReviewedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning card with progress: %w", err)
		}
		cards = append(cards, card)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating card rows: %w", err)
	}

	return cards, nil
}

func (s *Storage) GetCard(cardID string) (*Card, error) {
	query := `
		SELECT id, deck_id, front, back, created_at, updated_at, deleted_at
		FROM cards
		WHERE id = ? AND deleted_at IS NULL
	`

	var card Card
	err := s.db.QueryRow(query, cardID).Scan(
		&card.ID,
		&card.DeckID,
		&card.Front,
		&card.Back,
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
