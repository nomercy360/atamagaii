package handler

import (
	"atamagaii/internal/ai"
	"atamagaii/internal/contract"
	"atamagaii/internal/db"
	"atamagaii/internal/middleware"
	"atamagaii/internal/storage"
	telegram "github.com/go-telegram/bot"
	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"net/http"
)

type Handler struct {
	bot             *telegram.Bot
	db              *db.Storage
	jwtSecret       string
	botToken        string
	webAppURL       string
	storageProvider storage.Provider
	openaiClient    *ai.OpenAIClient
}

func New(
	bot *telegram.Bot,
	db *db.Storage,
	jwtSecret string,
	botToken string,
	webAppURL string,
	storageProvider storage.Provider,
	openaiClient *ai.OpenAIClient,
) *Handler {
	return &Handler{
		bot:             bot,
		db:              db,
		jwtSecret:       jwtSecret,
		botToken:        botToken,
		webAppURL:       webAppURL,
		storageProvider: storageProvider,
		openaiClient:    openaiClient,
	}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {

	e.POST("/webhook", h.HandleWebhook)
	e.POST("/auth/telegram", h.TelegramAuth)

	v1 := e.Group("/v1")

	v1.Use(echojwt.WithConfig(middleware.GetUserAuthConfig(h.jwtSecret)))

	h.AddFlashcardRoutes(v1)
	h.AddAnkiImportRoutes(v1)

	// Task routes
	v1.GET("/tasks", h.GetTasks)
	v1.GET("/tasks/by-deck", h.GetTasksPerDeck)
	v1.POST("/tasks/submit", h.SubmitTaskResponse)
	
	// User routes
	v1.PUT("/user", h.UpdateUserHandler)
}

func GetUserIDFromToken(c echo.Context) (string, error) {
	user, ok := c.Get("user").(*jwt.Token)
	if !ok || user == nil {
		return "", echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
	}

	claims, ok := user.Claims.(*contract.JWTClaims)
	if !ok || claims == nil {
		return "", echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
	}

	return claims.UID, nil
}
