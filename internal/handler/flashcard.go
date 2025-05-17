package handler

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"strconv"
)

type ReviewCardRequest struct {
	Rating      int `json:"rating" validate:"required,min=1,max=2"`
	TimeSpentMs int `json:"time_spent_ms" validate:"required"`
}

type CreateDeckFromFileRequest struct {
	Name     string `json:"name" validate:"required"`
	FileName string `json:"file_name" validate:"required"` // e.g., "vocab_n5.json"
}

type UpdateDeckSettingsRequest struct {
	NewCardsPerDay int    `json:"new_cards_per_day" validate:"required,min=1,max=500"`
	Name           string `json:"name" validate:"required"`
}

type UpdateCardRequest struct {
	Fields contract.CardFields `json:"fields" validate:"required"`
}

func (h *Handler) AddFlashcardRoutes(g *echo.Group) {
	g.GET("/decks", h.GetDecks)
	g.GET("/decks/:id", h.GetDeck)
	g.GET("/decks/available", h.GetAvailableDecks)
	g.POST("/decks/import", h.CreateDeckFromFile)
	g.PUT("/decks/:id/settings", h.UpdateDeckSettings)
	g.DELETE("/decks/:id", h.DeleteDeck)

	g.GET("/cards/due", h.GetDueCards)
	g.GET("/cards/:id", h.GetCard)
	g.PUT("/cards/:id", h.UpdateCard)
	g.POST("/cards/generate", h.GenerateCard)

	g.POST("/cards/:id/review", h.ReviewCard)
	g.GET("/stats", h.GetStats)
	g.GET("/stats/history", h.GetStudyHistory)
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

	if len(deckID) < 3 {
		return echo.NewHTTPError(http.StatusNotFound, "Deck not found")
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

func formatCardResponse(card db.Card) (contract.CardResponse, error) {
	response := contract.CardResponse{
		ID:              card.ID,
		DeckID:          card.DeckID,
		CreatedAt:       card.CreatedAt,
		UpdatedAt:       card.UpdatedAt,
		DeletedAt:       card.DeletedAt,
		NextReview:      card.NextReview,
		Interval:        card.Interval,
		Ease:            card.Ease,
		ReviewCount:     card.ReviewCount,
		LapsCount:       card.LapsCount,
		LastReviewedAt:  card.LastReviewedAt,
		FirstReviewedAt: card.FirstReviewedAt,
		State:           card.State,
		LearningStep:    card.LearningStep,
	}

	var fields contract.CardFields
	if err := json.Unmarshal([]byte(card.Fields), &fields); err != nil {
		return response, fmt.Errorf("error unmarshalling card fields: %w", err)
	}
	response.Fields = fields

	return response, nil
}

func parseIntQuery(c echo.Context, key string, defaultValue int) int {
	value, err := strconv.Atoi(c.QueryParam(key))
	if err != nil || value < 0 {
		return defaultValue
	}
	return value
}

func (h *Handler) GetDueCards(c echo.Context) error {
	userID, err := GetUserIDFromToken(c)
	if err != nil {
		return err
	}

	deckID := c.QueryParam("deck_id")
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

	limit := parseIntQuery(c, "limit", 3)

	cards, err := h.db.GetCardsForReview(userID, deckID, limit, deck.NewCardsPerDay)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch due cards").WithInternal(err)
	}

	responses := make([]contract.CardResponse, len(cards))
	for i, card := range cards {
		response, err := formatCardResponse(card)
		if err != nil {
			continue
		}

		intervalAgainVal := db.CalculatePreviewInterval(card, db.RatingAgain)
		intervalGoodVal := db.CalculatePreviewInterval(card, db.RatingGood)

		response.NextIntervals = contract.PotentialIntervalsForDisplay{
			Again: db.FormatSimpleDuration(intervalAgainVal),
			Good:  db.FormatSimpleDuration(intervalGoodVal),
		}

		responses[i] = response
	}

	return c.JSON(http.StatusOK, responses)
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

	card, err := h.db.GetCard(cardID, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Card not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch card")
	}

	deck, err := h.db.GetDeck(card.DeckID)
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

	if err := h.db.ReviewCard(card, req.Rating, req.TimeSpentMs); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to process review: "+err.Error())
	}

	stats, err := h.db.GetDeckStatistics(userID, deck.ID, deck.NewCardsPerDay)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch stats").WithInternal(err)
	}

	nextCards, err := h.db.GetCardsForReview(userID, deck.ID, 5, deck.NewCardsPerDay)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch next card").WithInternal(err)
	}

	respCards := make([]contract.CardResponse, 0, len(nextCards))

	for _, c := range nextCards {
		nextCardResp, err := formatCardResponse(c)
		if err == nil {
			intervalAgainVal := db.CalculatePreviewInterval(c, db.RatingAgain)
			intervalGoodVal := db.CalculatePreviewInterval(c, db.RatingGood)

			nextCardResp.NextIntervals = contract.PotentialIntervalsForDisplay{
				Again: db.FormatSimpleDuration(intervalAgainVal),
				Good:  db.FormatSimpleDuration(intervalGoodVal),
			}

			respCards = append(respCards, nextCardResp)
		}
	}

	resp := contract.ReviewCardResponse{
		Stats:     stats,
		NextCards: respCards,
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetStats(c echo.Context) error {
	userID, err := GetUserIDFromToken(c)
	if err != nil {
		return err
	}

	// Get due cards count
	dueCount, err := h.db.GetDueCardCount(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch statistics")
	}

	// Get study statistics
	studyStats, err := h.db.GetUserStudyStats(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch study statistics")
	}

	// Combine both statistics types in the response
	response := map[string]interface{}{
		"due_cards":   dueCount,
		"study_stats": studyStats,
	}

	return c.JSON(http.StatusOK, response)
}

func (h *Handler) GetStudyHistory(c echo.Context) error {
	userID, err := GetUserIDFromToken(c)
	if err != nil {
		return err
	}

	// Parse the days parameter, default to 100 days
	days := parseIntQuery(c, "days", 100)

	// Get study history
	history, err := h.db.GetUserStudyHistory(userID, days)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch study history").WithInternal(err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"history": history,
	})
}

func (h *Handler) GetCard(c echo.Context) error {
	userID, err := GetUserIDFromToken(c)
	if err != nil {
		return err
	}

	cardID := c.Param("id")
	if cardID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Card ID is required")
	}

	card, err := h.db.GetCard(cardID, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Card not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch card").WithInternal(err)
	}

	response, err := formatCardResponse(*card)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to format card response").WithInternal(err)
	}

	return c.JSON(http.StatusOK, response)
}

func (h *Handler) UpdateDeckSettings(c echo.Context) error {
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

	req := new(UpdateDeckSettingsRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
	}

	if err := c.Validate(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	deck.NewCardsPerDay = req.NewCardsPerDay
	deck.Name = req.Name

	if err := h.db.UpdateDeckSettings(deckID, deck); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update deck settings: "+err.Error())
	}

	// Get updated deck to return to the client
	updatedDeck, err := h.db.GetDeck(deckID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch updated deck")
	}

	return c.JSON(http.StatusOK, updatedDeck)
}

func (h *Handler) DeleteDeck(c echo.Context) error {
	userID, err := GetUserIDFromToken(c)
	if err != nil {
		return err
	}

	deckID := c.Param("id")
	if deckID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Deck ID is required")
	}

	// Verify the deck exists and belongs to the user
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

	if err := h.db.DeleteDeck(deckID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete deck: "+err.Error())
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) UpdateCard(c echo.Context) error {
	userID, err := GetUserIDFromToken(c)
	if err != nil {
		return err
	}

	cardID := c.Param("id")
	if cardID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Card ID is required")
	}

	card, err := h.db.GetCard(cardID, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Card not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch card").WithInternal(err)
	}

	deck, err := h.db.GetDeck(card.DeckID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to verify card ownership")
	}

	if deck.UserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	req := new(UpdateCardRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
	}

	if err := c.Validate(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	fieldsJSON, err := json.Marshal(req.Fields)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to process card fields")
	}

	if err := h.db.UpdateCardFields(cardID, string(fieldsJSON)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update card")
	}

	updatedCard, err := h.db.GetCard(cardID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch updated card")
	}

	response, err := formatCardResponse(*updatedCard)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to format card response")
	}

	return c.JSON(http.StatusOK, response)
}
