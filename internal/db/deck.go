package db

import (
	"atamagaii/internal/utils"
	"database/sql"
	"errors"
	"fmt"
	nanoid "github.com/matoous/go-nanoid/v2"
	"time"
)

type Deck struct {
	ID                string          `db:"id" json:"id"`
	Name              string          `db:"name" json:"name"`
	Level             string          `db:"level" json:"level"`
	LanguageCode      string          `db:"language_code" json:"language_code"`           // ISO 639-1 language code (e.g., "ja", "en", "th")
	TranscriptionType string          `db:"transcription_type" json:"transcription_type"` // Type of transcription/reading aids
	NewCardsPerDay    int             `db:"new_cards_per_day" json:"new_cards_per_day"`
	UserID            string          `db:"user_id" json:"user_id"`
	CreatedAt         time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time       `db:"updated_at" json:"updated_at"`
	DeletedAt         *time.Time      `db:"deleted_at" json:"deleted_at,omitempty"`
	Stats             *DeckStatistics `json:"stats,omitempty"`
}

func (s *Storage) CreateDeck(userID, name, level string, languageCode string, transcriptionType string) (*Deck, error) {
	deckID := nanoid.Must()
	now := time.Now()
	defaultNewCardsPerDay := 20

	// Default to Japanese if no language code specified
	if languageCode == "" {
		languageCode = "jp"
	}

	// Default transcription type based on language
	if transcriptionType == "" {
		switch languageCode {
		case "jp":
			transcriptionType = "furigana"
		case "zh":
			transcriptionType = "pinyin"
		case "th":
			transcriptionType = "thai_romanization"
		case "ge":
			transcriptionType = "mkhedruli"
		default:
			transcriptionType = "none"
		}
	}

	query := `
		INSERT INTO decks (id, name, level, language_code, transcription_type, new_cards_per_day, user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query, deckID, name, level, languageCode, transcriptionType, defaultNewCardsPerDay, userID, now, now)
	if err != nil {
		return nil, fmt.Errorf("error creating deck: %w", err)
	}

	return &Deck{
		ID:                deckID,
		Name:              name,
		Level:             level,
		LanguageCode:      languageCode,
		TranscriptionType: transcriptionType,
		NewCardsPerDay:    defaultNewCardsPerDay,
		UserID:            userID,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func (s *Storage) GetDecks(userID string) ([]Deck, error) {
	query := `
		SELECT id, name, level, language_code, transcription_type, new_cards_per_day, user_id, created_at, updated_at, deleted_at
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
			&deck.Level,
			&deck.LanguageCode,
			&deck.TranscriptionType,
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

		deck.Stats = stats
		decks = append(decks, deck)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating deck rows: %w", err)
	}

	return decks, nil
}

func (s *Storage) GetDeck(deckID string) (*Deck, error) {
	query := `
		SELECT id, name, level, language_code, transcription_type, new_cards_per_day, user_id, created_at, updated_at, deleted_at
		FROM decks
		WHERE id = ? AND deleted_at IS NULL
	`

	var deck Deck
	err := s.db.QueryRow(query, deckID).Scan(
		&deck.ID,
		&deck.Name,
		&deck.Level,
		&deck.LanguageCode,
		&deck.TranscriptionType,
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

	deck.Stats = stats

	return &deck, nil
}

func (s *Storage) UpdateDeckSettings(deckID string, deck *Deck) error {
	query := `
		UPDATE decks
		SET new_cards_per_day = ?, name = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	now := time.Now()
	result, err := s.db.Exec(query, deck.NewCardsPerDay, deck.Name, now, deckID)
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

	// First, get all the card IDs for this deck
	cardIDsQuery := `
		SELECT id, user_id FROM cards
		WHERE deck_id = ? AND deleted_at IS NULL
	`
	rows, err := tx.Query(cardIDsQuery, deckID)
	if err != nil {
		return fmt.Errorf("error fetching card IDs: %w", err)
	}

	// Store card IDs and user IDs to mark reviews as deleted
	type CardInfo struct {
		ID     string
		UserID string
	}
	var cardInfos []CardInfo

	for rows.Next() {
		var cardInfo CardInfo
		if err := rows.Scan(&cardInfo.ID, &cardInfo.UserID); err != nil {
			rows.Close()
			return fmt.Errorf("error scanning card info: %w", err)
		}
		cardInfos = append(cardInfos, cardInfo)
	}
	rows.Close()

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating card rows: %w", err)
	}

	// Mark deck as deleted
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

	// Mark cards as deleted
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
	NewCards            int `json:"new_cards"`
	LearningCards       int `json:"learning_cards"`
	ReviewCards         int `json:"review_cards"`
	CompletedTodayCards int `json:"completed_today_cards"`
}

func (s *Storage) GetDeckStatistics(userID string, deckID string, newCardsPerDay int) (*DeckStatistics, error) {
	stats := &DeckStatistics{}
	todayEnd := time.Now().Truncate(24 * time.Hour).Add(24*time.Hour - time.Nanosecond)

	today := time.Now().Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	dueDueQuery := `
        SELECT
            COALESCE(SUM(CASE WHEN (c.state = 'learning' OR c.state = 'relearning') AND c.next_review <= ? THEN 1 ELSE 0 END), 0) as learning_due_count,
            COALESCE(SUM(CASE WHEN c.state = 'review' AND c.next_review <= ? THEN 1 ELSE 0 END), 0) as review_due_count,
            COALESCE(SUM(CASE WHEN c.last_reviewed_at >= ? AND c.last_reviewed_at < ? AND c.next_review >= ? THEN 1 ELSE 0 END), 0) as completed_today_count
        FROM cards c
        WHERE c.user_id = ? AND c.deck_id = ? AND c.deleted_at IS NULL;
    `

	err := s.db.QueryRow(dueDueQuery, todayEnd, todayEnd, today, tomorrow, tomorrow, userID, deckID).Scan(
		&stats.LearningCards,
		&stats.ReviewCards,
		&stats.CompletedTodayCards,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("error calculating due cards statistics for deck %s: %w", deckID, err)
	}

	// today variable is already defined above
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
		WHERE c.user_id = ?
		AND c.deck_id = ?
		AND c.deleted_at IS NULL
		AND c.state = 'new'
		AND c.review_count = 0
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

func (s *Storage) GetOrCreateGeneratedDeck(userID string, languageCode string, transcriptionType string) (*Deck, error) {
	query := `
		SELECT id, name, level, language_code, transcription_type, new_cards_per_day, user_id, created_at, updated_at, deleted_at
		FROM decks
		WHERE user_id = ? AND language_code = ? AND name LIKE 'Generated %' AND deleted_at IS NULL
		LIMIT 1
	`

	var deck Deck
	err := s.db.QueryRow(query, userID, languageCode).Scan(
		&deck.ID,
		&deck.Name,
		&deck.Level,
		&deck.LanguageCode,
		&deck.TranscriptionType,
		&deck.NewCardsPerDay,
		&deck.UserID,
		&deck.CreatedAt,
		&deck.UpdatedAt,
		&deck.DeletedAt,
	)

	if err == nil {
		stats, err := s.GetDeckStatistics(userID, deck.ID, deck.NewCardsPerDay)
		if err != nil {
			return nil, fmt.Errorf("error getting deck statistics: %w", err)
		}
		deck.Stats = stats
		return &deck, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		languageName := utils.GetLanguageNameFromCode(languageCode)

		name := fmt.Sprintf("Generated %s Cards", languageName)
		level := "mixed"

		return s.CreateDeck(userID, name, level, languageCode, transcriptionType)
	}

	return nil, fmt.Errorf("error finding generated deck: %w", err)
}
