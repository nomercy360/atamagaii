package contract

import (
	"atamagaii/internal/db"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
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
