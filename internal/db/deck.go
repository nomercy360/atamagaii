package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Deck struct {
	ID          string     `db:"id" json:"id"`
	Name        string     `db:"name" json:"name"`
	Description string     `db:"description" json:"description"`
	Level       string     `db:"level" json:"level"` // N5, N4, N3, etc.
	UserID      string     `db:"user_id" json:"user_id"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

func (s *Storage) GetDecks(userID string) ([]Deck, error) {
	query := `
		SELECT id, name, description, level, user_id, created_at, updated_at, deleted_at
		FROM decks 
		WHERE user_id = ? AND deleted_at IS NULL
	`
	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("error getting decks: %w", err)
	}
	defer rows.Close()

	var decks []Deck
	for rows.Next() {
		var deck Deck
		if err := rows.Scan(
			&deck.ID,
			&deck.Name,
			&deck.Description,
			&deck.Level,
			&deck.UserID,
			&deck.CreatedAt,
			&deck.UpdatedAt,
			&deck.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning deck: %w", err)
		}
		decks = append(decks, deck)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating deck rows: %w", err)
	}

	return decks, nil
}

func (s *Storage) GetDeck(deckID string) (*Deck, error) {
	query := ` 
		SELECT id, name, description, level, user_id, created_at, updated_at, deleted_at
		FROM decks
		WHERE id = ? AND deleted_at IS NULL
	`

	var deck Deck
	err := s.db.QueryRow(query, deckID).Scan(
		&deck.ID,
		&deck.Name,
		&deck.Description,
		&deck.Level,
		&deck.UserID,
		&deck.CreatedAt,
		&deck.UpdatedAt,
		&deck.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error getting deck: %w", err)
	}

	return &deck, nil
}
