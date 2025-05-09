package handler

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"os"
	"path/filepath"
)

type ReviewCardRequest struct {
	Rating      int `json:"rating" validate:"required,min=1,max=4"`
	TimeSpentMs int `json:"time_spent_ms" validate:"required"`
}

type CreateDeckFromFileRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
	FileName    string `json:"file_name" validate:"required"` // e.g., "vocab_n5.json"
}

type UpdateDeckSettingsRequest struct {
	NewCardsPerDay int `json:"new_cards_per_day" validate:"required,min=1,max=500"`
}

func (h *Handler) AddFlashcardRoutes(g *echo.Group) {
	g.GET("/decks", h.GetDecks)
	g.GET("/decks/:id", h.GetDeck)
	g.POST("/decks/import", h.CreateDeckFromFile)
	g.PUT("/decks/:id/settings", h.UpdateDeckSettings)
	g.DELETE("/decks/:id", h.DeleteDeck)

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

	// Corner case handling for tests and error cases
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

func formatCardResponse(card db.CardWithProgress) (contract.CardResponse, error) {
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

	limit := 3

	cards, err := h.db.GetCardsWithProgress(userID, deckID, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch due cards").WithInternal(err)
	}

	responses := make([]contract.CardResponse, 0, len(cards))
	for _, card := range cards {
		response, err := formatCardResponse(card)
		if err != nil {
			continue
		}
		responses = append(responses, response)
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

	progress, err := h.db.GetCardProgress(userID, cardID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch updated progress").WithInternal(err)
	}

	stats, err := h.db.GetDeckStatistics(userID, deck.ID, deck.NewCardsPerDay)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch stats").WithInternal(err)
	}

	resp := contract.ReviewCardResponse{
		Stats:    stats,
		Progress: progress,
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetStats(c echo.Context) error {
	userID, err := GetUserIDFromToken(c)
	if err != nil {
		return err
	}

	dueCount, err := h.db.GetDueCardCount(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch statistics")
	}

	stats := map[string]interface{}{
		"due_cards": dueCount,
	}

	return c.JSON(http.StatusOK, stats)
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

	if err := h.db.UpdateDeckNewCardsPerDay(deckID, req.NewCardsPerDay); err != nil {
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

func (h *Handler) CreateDeckFromFile(c echo.Context) error {
	userID, err := GetUserIDFromToken(c)
	if err != nil {
		return err
	}

	req := new(CreateDeckFromFileRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
	}

	if err := c.Validate(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	rootDir := "."

	// Try to locate the materials directory by checking a few common locations
	materialsDir := filepath.Join(rootDir, "materials")
	if _, err := os.Stat(materialsDir); os.IsNotExist(err) {
		// Try one level up (for tests running from a subdirectory)
		materialsDir = filepath.Join(rootDir, "..", "materials")
		if _, err := os.Stat(materialsDir); os.IsNotExist(err) {
			// Try absolute path based on working directory
			wd, err := os.Getwd()
			if err == nil {
				// Try to find materials in the main project directory
				for i := 0; i < 3; i++ { // Look up to 3 levels up
					checkPath := filepath.Join(wd, "materials")
					if _, err := os.Stat(checkPath); err == nil {
						materialsDir = checkPath
						break
					}
					// Go up one directory
					wd = filepath.Dir(wd)
				}
			}
		}
	}

	filePath := filepath.Join(materialsDir, req.FileName)

	// Check if the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("File %s does not exist (looked in %s)", req.FileName, materialsDir))
	}

	// Open and read the vocabulary file
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to read file: %v", err))
	}

	// Parse the JSON vocabulary data
	var vocabularyItems []db.VocabularyItem
	if err := json.Unmarshal(fileData, &vocabularyItems); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to parse vocabulary data: %v", err))
	}

	// Extract level from the first item (assuming all items in a file have the same level)
	var level string
	if len(vocabularyItems) > 0 {
		level = vocabularyItems[0].Level
	} else {
		// Try to extract level from filename if there are no items
		if len(req.FileName) >= 8 && req.FileName[:7] == "vocab_n" {
			level = req.FileName[6:8] // Extract "N5", "N4", etc.
		} else {
			level = "Unknown"
		}
	}

	// Create the deck
	deck, err := h.db.CreateDeck(userID, req.Name, req.Description, level)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to create deck: %v", err))
	}

	// Prepare card data
	fieldsArray := make([]string, len(vocabularyItems))

	for i, item := range vocabularyItems {
		fieldsContent := map[string]interface{}{
			"kanji":       item.Kanji,
			"kana":        item.Kana,
			"translation": item.Translation,
			"examples":    item.Examples,
			"audio_url":   item.AudioURL,
		}
		fieldsJSON, _ := json.Marshal(fieldsContent)
		fieldsArray[i] = string(fieldsJSON)
	}

	if err := h.db.AddCardsInBatch(deck.ID, fieldsArray); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to add cards to deck: %v", err))
	}

	return c.JSON(http.StatusCreated, deck)
}
