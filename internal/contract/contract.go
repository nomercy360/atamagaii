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

type CardFields struct {
	Word            string `json:"word"`
	Reading         string `json:"reading"`
	WordFurigana    string `json:"word_furigana"`
	MeaningEn       string `json:"meaning_en"`
	MeaningRu       string `json:"meaning_ru"`
	ExampleJa       string `json:"example_ja"`
	ExampleEn       string `json:"example_en"`
	ExampleRu       string `json:"example_ru"`
	ExampleFurigana string `json:"example_furigana"`
	Frequency       int    `json:"frequency"`
	AudioWord       string `json:"audio_word"`
	AudioExample    string `json:"audio_example"`
	ImageURL        string `json:"image_url,omitempty"`
}
type CardResponse struct {
	ID              string        `json:"id"`
	DeckID          string        `json:"deck_id"`
	Fields          CardFields    `json:"fields"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	DeletedAt       *time.Time    `json:"deleted_at,omitempty"`
	NextReview      *time.Time    `json:"next_review,omitempty"`
	Interval        time.Duration `json:"interval,omitempty"`
	Ease            float64       `json:"ease,omitempty"`
	ReviewCount     int           `json:"review_count,omitempty"`
	LapsCount       int           `json:"laps_count,omitempty"`
	LastReviewedAt  *time.Time    `json:"last_reviewed_at,omitempty"`
	FirstReviewedAt *time.Time    `json:"first_reviewed_at,omitempty"`
	State           string        `json:"state,omitempty"`
	LearningStep    int           `json:"learning_step,omitempty"`
}

type ReviewCardResponse struct {
	Stats *db.DeckStatistics `json:"stats"`
}
