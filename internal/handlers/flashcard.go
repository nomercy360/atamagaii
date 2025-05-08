package handlers

import (
	"atamagaii/internal/db"
	"errors"
	"github.com/labstack/echo/v4"
	"net/http"
	"strconv"
)

type ReviewCardRequest struct {
	CardID      string `json:"card_id" validate:"required"`
	Rating      int    `json:"rating" validate:"required,min=1,max=4"`
	TimeSpentMs int    `json:"time_spent_ms" validate:"required"`
}

func (h *Handler) AddFlashcardRoutes(g *echo.Group) {
	g.GET("/decks", h.GetDecks)
	g.GET("/decks/:id", h.GetDeck)

	g.GET("/cards/due", h.GetDueCards)

	g.POST("/cards/:id/review", h.ReviewCard)
	g.GET("/stats", h.GetStats)
}

func (h *Handler) GetDecks(c echo.Context) error {
	userID, err := GetUserIDFromToken(c)
	if err != nil {
		return err
	}

	decks, err := h.db.GetDecks(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch decks")
	}

	return c.JSON(http.StatusOK, decks)
}

func (h *Handler) GetDeck(c echo.Context) error {
	userID, err := GetUserIDFromToken(c)
	if err != nil {
		return err
	}

	deckID := c.Param("id")
	if deckID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Deck ID is required")
	}

	deck, err := h.db.GetDeck(deckID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Deck not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch deck")
	}

	if deck.UserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	return c.JSON(http.StatusOK, deck)
}

func (h *Handler) GetDueCards(c echo.Context) error {
	userID, err := GetUserIDFromToken(c)
	if err != nil {
		return err
	}

	deckID := c.QueryParam("deck_id")
	if deckID != "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Deck ID is required")
	}

	deck, err := h.db.GetDeck(deckID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Deck not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch deck")
	}

	if deck.UserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	limitStr := c.QueryParam("limit")
	limit := 20 // Default limit
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			limit = 20
		}
	}

	cards, err := h.db.GetCardsWithProgress(userID, deckID, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch due cards")
	}

	return c.JSON(http.StatusOK, cards)
}

func (h *Handler) ReviewCard(c echo.Context) error {
	userID, err := GetUserIDFromToken(c)
	if err != nil {
		return err
	}

	cardID := c.Param("id")
	if cardID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Card ID is required")
	}

	existingCard, err := h.db.GetCard(cardID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Card not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch card")
	}

	deck, err := h.db.GetDeck(existingCard.DeckID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to verify card ownership")
	}

	if deck.UserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	req := new(ReviewCardRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
	}

	if err := c.Validate(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := h.db.ReviewCard(userID, cardID, req.Rating, req.TimeSpentMs); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to process review: "+err.Error())
	}

	// Retrieve updated progress
	progress, err := h.db.GetCardProgress(userID, cardID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch updated progress")
	}

	return c.JSON(http.StatusOK, progress)
}

func (h *Handler) GetStats(c echo.Context) error {
	userID, err := GetUserIDFromToken(c)
	if err != nil {
		return err
	}

	deckID := c.QueryParam("deck_id")
	if deckID != "" {

		deck, err := h.db.GetDeck(deckID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "Deck not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch deck")
		}

		if deck.UserID != userID {
			return echo.NewHTTPError(http.StatusForbidden, "Access denied")
		}
	}

	dueCount, err := h.db.GetDueCardCount(userID, deckID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch statistics")
	}

	stats := map[string]interface{}{
		"due_cards": dueCount,
	}

	return c.JSON(http.StatusOK, stats)
}
