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
	Term                     string `json:"term"`
	Transcription            string `json:"transcription"`
	TranscriptionType        string `json:"transcription_type"`
	TermWithTranscription    string `json:"term_with_transcription"`
	MeaningEn                string `json:"meaning_en"`
	MeaningRu                string `json:"meaning_ru"`
	ExampleNative            string `json:"example_native"`
	ExampleEn                string `json:"example_en"`
	ExampleRu                string `json:"example_ru"`
	ExampleWithTranscription string `json:"example_with_transcription"`
	Frequency                int    `json:"frequency"`
	AudioWord                string `json:"audio_word"`
	AudioExample             string `json:"audio_example"`
	ImageURL                 string `json:"image_url,omitempty"`
}
type CardResponse struct {
	ID              string                       `json:"id"`
	DeckID          string                       `json:"deck_id"`
	Fields          CardFields                   `json:"fields"`
	CreatedAt       time.Time                    `json:"created_at"`
	UpdatedAt       time.Time                    `json:"updated_at"`
	DeletedAt       *time.Time                   `json:"deleted_at,omitempty"`
	NextReview      *time.Time                   `json:"next_review,omitempty"`
	Interval        time.Duration                `json:"interval,omitempty"`
	Ease            float64                      `json:"ease,omitempty"`
	ReviewCount     int                          `json:"review_count,omitempty"`
	LapsCount       int                          `json:"laps_count,omitempty"`
	LastReviewedAt  *time.Time                   `json:"last_reviewed_at,omitempty"`
	FirstReviewedAt *time.Time                   `json:"first_reviewed_at,omitempty"`
	State           string                       `json:"state,omitempty"`
	LearningStep    int                          `json:"learning_step,omitempty"`
	NextIntervals   PotentialIntervalsForDisplay `json:"next_intervals,omitempty"`
}

type ReviewCardResponse struct {
	Stats     *db.DeckStatistics `json:"stats"`
	NextCards []CardResponse     `json:"next_cards"`
}

type PotentialIntervalsForDisplay struct {
	Again string `json:"again"`
	Good  string `json:"good"`
}
