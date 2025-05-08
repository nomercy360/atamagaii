package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type User struct {
	ID         string     `db:"id" json:"id"`
	TelegramID int64      `db:"telegram_id" json:"telegram_id"`
	Level      string     `db:"level" json:"level"`
	Points     float64    `db:"points" json:"points"`
	Name       *string    `db:"name" json:"name"`
	Username   *string    `db:"username" json:"username"`
	AvatarURL  *string    `db:"avatar_url" json:"avatar_url"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt  *time.Time `db:"deleted_at" json:"deleted_at"`
}

const (
	ModeExercise = "exercise"
	ModeVocab    = "vocab"
)

func (s *Storage) GetUser(telegramID int64) (*User, error) {
	var user User
	query := `SELECT id, telegram_id, username, avatar_url, name, level, points, created_at, updated_at FROM users WHERE telegram_id = ?`
	err := s.db.QueryRow(query, telegramID).Scan(
		&user.ID,
		&user.TelegramID,
		&user.Username,
		&user.AvatarURL,
		&user.Name,
		&user.Level,
		&user.Points,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error getting user: %w", err)
	}
	return &user, nil
}

func (s *Storage) SaveUser(user *User) error {
	query := `
		INSERT INTO users 
		    (id, telegram_id, level, username, avatar_url, name)
		VALUES (?, ?, ?, ?, ?, ?)`

	_, err := s.db.Exec(query, user.ID, user.TelegramID, user.Level, user.Username, user.AvatarURL, user.Name)
	if err != nil {
		return fmt.Errorf("error saving user: %w", err)
	}

	return nil
}

func (s *Storage) UpdateUser(user *User) error {
	query := `
		UPDATE users
		SET username = ?, avatar_url = ?, name = ?
		WHERE telegram_id = ?`

	_, err := s.db.Exec(query,
		user.Username, user.AvatarURL, user.Name, user.TelegramID)
	if err != nil {
		return fmt.Errorf("error updating user: %w", err)
	}

	return nil
}
