package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// UserSettings holds user preferences for task generation
type UserSettings struct {
	MaxTasksPerDay int        `json:"max_tasks_per_day"`
	TaskTypes      []TaskType `json:"task_types"`
}

type User struct {
	ID           string        `db:"id" json:"id"`
	TelegramID   int64         `db:"telegram_id" json:"telegram_id"`
	Points       float64       `db:"points" json:"points"`
	Name         *string       `db:"name" json:"name"`
	Username     *string       `db:"username" json:"username"`
	LanguageCode string        `db:"language_code" json:"language_code"`
	AvatarURL    *string       `db:"avatar_url" json:"avatar_url"`
	Settings     *UserSettings `db:"settings" json:"settings,omitempty"`
	SettingsJSON *string       `db:"-" json:"-"` // Used for SQL operations
	CreatedAt    time.Time     `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time     `db:"updated_at" json:"updated_at"`
	DeletedAt    *time.Time    `db:"deleted_at" json:"deleted_at"`
}

const (
	ModeExercise = "exercise"
	ModeVocab    = "vocab"
)

// GetUserByID retrieves a user by their ID
func (s *Storage) GetUserByID(userID string) (*User, error) {
	var user User
	var settingsStr sql.NullString
	query := `SELECT id, telegram_id, username, avatar_url, name, points, language_code, settings, created_at, updated_at FROM users WHERE id = ?`
	err := s.db.QueryRow(query, userID).Scan(
		&user.ID,
		&user.TelegramID,
		&user.Username,
		&user.AvatarURL,
		&user.Name,
		&user.Points,
		&user.LanguageCode,
		&settingsStr,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error getting user by ID: %w", err)
	}

	// Parse settings if present
	if settingsStr.Valid && settingsStr.String != "" {
		var settings UserSettings
		if err := json.Unmarshal([]byte(settingsStr.String), &settings); err != nil {
			return nil, fmt.Errorf("error parsing user settings: %w", err)
		}
		user.Settings = &settings
	} else {
		// Set default settings if none exist
		user.Settings = &UserSettings{
			MaxTasksPerDay: 10, // Default to 10 tasks per day
			TaskTypes: []TaskType{
				TaskTypeVocabRecall,
				TaskTypeSentenceTranslation,
				TaskTypeAudio,
			},
		}
	}

	return &user, nil
}

// GetUser retrieves a user by their Telegram ID
func (s *Storage) GetUser(telegramID int64) (*User, error) {
	var user User
	var settingsStr sql.NullString
	query := `SELECT id, telegram_id, username, avatar_url, name, points, language_code, settings, created_at, updated_at FROM users WHERE telegram_id = ?`
	err := s.db.QueryRow(query, telegramID).Scan(
		&user.ID,
		&user.TelegramID,
		&user.Username,
		&user.AvatarURL,
		&user.Name,
		&user.Points,
		&user.LanguageCode,
		&settingsStr,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error getting user: %w", err)
	}

	// Parse settings if present
	if settingsStr.Valid && settingsStr.String != "" {
		var settings UserSettings
		if err := json.Unmarshal([]byte(settingsStr.String), &settings); err != nil {
			return nil, fmt.Errorf("error parsing user settings: %w", err)
		}
		user.Settings = &settings
	} else {
		// Set default settings if none exist
		user.Settings = &UserSettings{
			MaxTasksPerDay: 10, // Default to 10 tasks per day
			TaskTypes: []TaskType{
				TaskTypeVocabRecall,
				TaskTypeSentenceTranslation,
				TaskTypeAudio,
			},
		}
	}

	return &user, nil
}

func (s *Storage) SaveUser(user *User) error {
	// Serialize settings to JSON if present
	var settingsJSON *string
	if user.Settings != nil {
		settingsBytes, err := json.Marshal(user.Settings)
		if err != nil {
			return fmt.Errorf("error serializing user settings: %w", err)
		}
		settingsStr := string(settingsBytes)
		settingsJSON = &settingsStr
	}

	query := `
		INSERT INTO users
		    (id, telegram_id, username, avatar_url, name, language_code, settings)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.Exec(query, user.ID, user.TelegramID, user.Username, user.AvatarURL, user.Name, user.LanguageCode, settingsJSON)
	if err != nil {
		return fmt.Errorf("error saving user: %w", err)
	}

	return nil
}

func (s *Storage) UpdateUser(user *User) error {
	// Serialize settings to JSON if present
	var settingsJSON *string
	if user.Settings != nil {
		settingsBytes, err := json.Marshal(user.Settings)
		if err != nil {
			return fmt.Errorf("error serializing user settings: %w", err)
		}
		settingsStr := string(settingsBytes)
		settingsJSON = &settingsStr
	}

	query := `
		UPDATE users
		SET username = ?, avatar_url = ?, name = ?, language_code = ?, settings = ?, updated_at = ?
		WHERE telegram_id = ?`

	now := time.Now()

	_, err := s.db.Exec(query,
		user.Username, user.AvatarURL, user.Name, user.LanguageCode, settingsJSON, now, user.TelegramID)
	if err != nil {
		return fmt.Errorf("error updating user: %w", err)
	}

	return nil
}
