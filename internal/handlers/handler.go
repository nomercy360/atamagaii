package handlers

import (
	"atamagaii/internal/db"
	telegram "github.com/go-telegram/bot"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"net/http"
)

type Handler struct {
	bot       *telegram.Bot
	db        *db.Storage
	jwtSecret string
	botToken  string
}

func NewHandler(
	bot *telegram.Bot,
	db *db.Storage,
	jwtSecret string,
	botToken string,
) *Handler {
	return &Handler{
		bot:       bot,
		db:        db,
		jwtSecret: jwtSecret,
		botToken:  botToken,
	}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {

	e.POST("/webhook", h.HandleWebhook)
	e.POST("/auth/telegram", h.TelegramAuth)

	v1 := e.Group("/v1")

	h.AddFlashcardRoutes(v1)
}

func GetUserIDFromToken(c echo.Context) (string, error) {
	user, ok := c.Get("user").(*jwt.Token)
	if !ok || user == nil {
		return "", echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
	}

	claims, ok := user.Claims.(*JWTClaims)
	if !ok || claims == nil {
		return "", echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
	}

	return claims.UID, nil
}
