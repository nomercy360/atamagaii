package db

import (
	"database/sql"
	"errors"
	"fmt"
	nanoid "github.com/matoous/go-nanoid/v2"
	"time"
)

type Deck struct {
	ID             string     `db:"id" json:"id"`
	Name           string     `db:"name" json:"name"`
	Description    string     `db:"description" json:"description"`
	Level          string     `db:"level" json:"level"`
	NewCardsPerDay int        `db:"new_cards_per_day" json:"new_cards_per_day"`
	UserID         string     `db:"user_id" json:"user_id"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
	NewCards       int        `json:"new_cards,omitempty" db:"-"`
	LearningCards  int        `json:"learning_cards,omitempty" db:"-"`
	ReviewCards    int        `json:"review_cards,omitempty" db:"-"`
}

func (s *Storage) CreateDeck(userID, name, description, level string) (*Deck, error) {
	deckID := nanoid.Must()
	now := time.Now()
	defaultNewCardsPerDay := 20

	query := `
		INSERT INTO decks (id, name, description, level, new_cards_per_day, user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query, deckID, name, description, level, defaultNewCardsPerDay, userID, now, now)
	if err != nil {
		return nil, fmt.Errorf("error creating deck: %w", err)
	}

	return &Deck{
		ID:             deckID,
		Name:           name,
		Description:    description,
		Level:          level,
		NewCardsPerDay: defaultNewCardsPerDay,
		UserID:         userID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func (s *Storage) GetDecks(userID string) ([]Deck, error) {
	query := `
		SELECT id, name, description, level, new_cards_per_day, user_id, created_at, updated_at, deleted_at
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
			&deck.NewCardsPerDay,
			&deck.UserID,
			&deck.CreatedAt,
			&deck.UpdatedAt,
			&deck.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning deck: %w", err)
		}

		stats, err := s.GetDeckStatistics(userID, deck.ID, deck.NewCardsPerDay)
		if err != nil {
			return nil, fmt.Errorf("error getting deck statistics: %w", err)
		}

		deck.NewCards = stats.NewCards
		deck.LearningCards = stats.LearningCards
		deck.ReviewCards = stats.ReviewCards

		decks = append(decks, deck)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating deck rows: %w", err)
	}

	return decks, nil
}

func (s *Storage) GetDeck(deckID string) (*Deck, error) {
	query := ` 
		SELECT id, name, description, level, new_cards_per_day, user_id, created_at, updated_at, deleted_at
		FROM decks
		WHERE id = ? AND deleted_at IS NULL
	`

	var deck Deck
	err := s.db.QueryRow(query, deckID).Scan(
		&deck.ID,
		&deck.Name,
		&deck.Description,
		&deck.Level,
		&deck.NewCardsPerDay,
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

	stats, err := s.GetDeckStatistics(deck.UserID, deck.ID, deck.NewCardsPerDay)
	if err != nil {
		return nil, fmt.Errorf("error getting deck statistics: %w", err)
	}

	deck.NewCards = stats.NewCards
	deck.LearningCards = stats.LearningCards
	deck.ReviewCards = stats.ReviewCards

	return &deck, nil
}

func (s *Storage) UpdateDeckNewCardsPerDay(deckID string, newCardsPerDay int) error {
	if newCardsPerDay < 0 {
		return fmt.Errorf("new cards per day cannot be negative")
	}

	query := `
		UPDATE decks
		SET new_cards_per_day = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	now := time.Now()
	result, err := s.db.Exec(query, newCardsPerDay, now, deckID)
	if err != nil {
		return fmt.Errorf("error updating deck new cards per day: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (s *Storage) DeleteDeck(deckID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	now := time.Now()

	deckQuery := `
		UPDATE decks
		SET deleted_at = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`
	result, err := tx.Exec(deckQuery, now, now, deckID)
	if err != nil {
		return fmt.Errorf("error marking deck as deleted: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	cardsQuery := `
		UPDATE cards
		SET deleted_at = ?, updated_at = ?
		WHERE deck_id = ? AND deleted_at IS NULL
	`
	_, err = tx.Exec(cardsQuery, now, now, deckID)
	if err != nil {
		return fmt.Errorf("error marking cards as deleted: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

type DeckStatistics struct {
	NewCards      int `json:"new_cards"`
	LearningCards int `json:"learning_cards"`
	ReviewCards   int `json:"review_cards"`
}

func (s *Storage) GetDeckStatistics(userID string, deckID string, newCardsPerDay int) (*DeckStatistics, error) {
	stats := &DeckStatistics{}
	todayEnd := time.Now().Truncate(24 * time.Hour).Add(24*time.Hour - time.Nanosecond)

	dueDueQuery := `
        SELECT
            COALESCE(SUM(CASE WHEN (cp.state = 'learning' OR cp.state = 'relearning') AND cp.next_review <= ? THEN 1 ELSE 0 END), 0) as learning_due_count,
            COALESCE(SUM(CASE WHEN cp.state = 'review' AND cp.next_review <= ? THEN 1 ELSE 0 END), 0) as review_due_count
        FROM card_progress cp
        JOIN cards c ON cp.card_id = c.id
        WHERE cp.user_id = ? AND c.deck_id = ? AND c.deleted_at IS NULL;
    `

	err := s.db.QueryRow(dueDueQuery, todayEnd, todayEnd, userID, deckID).Scan(
		&stats.LearningCards,
		&stats.ReviewCards,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("error calculating due cards statistics for deck %s: %w", deckID, err)
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
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("error counting new cards started today: %w", err)
	}

	newCardsRemaining := newCardsPerDay - newCardsStartedToday
	if newCardsRemaining < 0 {
		newCardsRemaining = 0
	}

	countTotalNewCardsQuery := `
		SELECT COUNT(*)
		FROM cards c
		LEFT JOIN card_progress p ON c.id = p.card_id AND p.user_id = ?
		WHERE c.deck_id = ? AND c.deleted_at IS NULL
		AND p.card_id IS NULL
	`

	var totalNewCards int
	err = s.db.QueryRow(countTotalNewCardsQuery, userID, deckID).Scan(&totalNewCards)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("error counting total new cards: %w", err)
	}

	if totalNewCards < newCardsRemaining {
		stats.NewCards = totalNewCards
	} else {
		stats.NewCards = newCardsRemaining
	}

	return stats, nil
}
