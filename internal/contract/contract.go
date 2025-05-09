package contract

import (
	"atamagaii/internal/db"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

type JWTClaims struct {
	jwt.RegisteredClaims
	UID    string `json:"uid,omitempty"`
	ChatID int64  `json:"chat_id,omitempty"`
}

type AuthTelegramRequest struct {
	Query string `json:"query"`
}

type AuthTelegramResponse struct {
	Token string  `json:"token"`
	User  db.User `json:"user"`
}

func (a AuthTelegramRequest) Validate() error {
	if a.Query == "" {
		return fmt.Errorf("query cannot be empty")
	}
	return nil
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// CardFront represents the front of a flashcard with properly unmarshaled JSON
type CardFront struct {
	Kanji string `json:"kanji"`
	Kana  string `json:"kana"`
}

type CardBackExample struct {
	Sentence    []CardBackFragment `json:"sentence"`
	Translation string             `json:"translation"`
	AudioURL    string             `json:"audio_url"`
}

type CardBackFragment struct {
	Fragment string  `json:"fragment"`
	Furigana *string `json:"furigana"`
}

type CardBack struct {
	Translation string            `json:"translation"`
	AudioURL    string            `json:"audio_url"`
	Examples    []CardBackExample `json:"examples"`
}

type CardResponse struct {
	ID              string        `json:"id"`
	DeckID          string        `json:"deck_id"`
	Front           CardFront     `json:"front"`
	Back            CardBack      `json:"back"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	DeletedAt       *time.Time    `json:"deleted_at,omitempty"`
	NextReview      *time.Time    `json:"next_review,omitempty"`
	Interval        time.Duration `json:"interval,omitempty"`
	Ease            *float64      `json:"ease,omitempty"`
	ReviewCount     *int          `json:"review_count,omitempty"`
	LapsCount       *int          `json:"laps_count,omitempty"`
	LastReviewedAt  *time.Time    `json:"last_reviewed_at,omitempty"`
	FirstReviewedAt *time.Time    `json:"first_reviewed_at,omitempty"`
	State           string        `json:"state,omitempty"`
	LearningStep    int           `json:"learning_step,omitempty"`
}
