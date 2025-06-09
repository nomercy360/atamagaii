package handler

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

type GenerateCardRequest struct {
	CardID string `json:"card_id" validate:"required"`
	DeckID string `json:"deck_id" validate:"required"`
}

func (h *Handler) GenerateCard(c echo.Context) error {
	userID, err := GetUserIDFromToken(c)
	if err != nil {
		return err
	}

	req := new(GenerateCardRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request")
	}

	if err := c.Validate(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	card, err := h.db.GetCardByID(req.CardID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch card").WithInternal(err)
	}

	if card.UserID != userID || card.DeckID != req.DeckID {
		return echo.NewHTTPError(http.StatusForbidden, "You do not have permission to access this card")
	}

	_, err = h.generateCardContent(c.Request().Context(), card)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to generate card content").WithInternal(err)
	}

	updatedCard, err := h.db.GetCard(req.CardID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch updated card").WithInternal(err)
	}

	response, err := formatCardResponse(*updatedCard)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to format card response").WithInternal(err)
	}

	return c.JSON(http.StatusOK, response)
}

// generateCardContent handles the core logic for generating card content (AI + audio)
func (h *Handler) generateCardContent(ctx context.Context, card *db.Card) (*contract.CardFields, error) {
	deck, err := h.db.GetDeck(card.DeckID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deck: %w", err)
	}

	// Parse card fields
	var fields contract.CardFields
	if err := json.Unmarshal([]byte(card.Fields), &fields); err != nil {
		return nil, fmt.Errorf("failed to parse card fields: %w", err)
	}

	if fields.Term == "" {
		return nil, fmt.Errorf("card has no term field")
	}

	// Generate content using AI
	updatedFields, err := h.aiClient.GenerateCardContent(ctx, fields.Term, deck.LanguageCode)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	updatedFields.LanguageCode = deck.LanguageCode

	// Generate combined audio for word and example
	if updatedFields.AudioExample == "" && updatedFields.ExampleNative != "" {
		combinedAudioFileName := fmt.Sprintf("%s_combined.wav", card.ID)
		combinedText := fmt.Sprintf("%s. %s", updatedFields.Term, updatedFields.ExampleNative)
		tempFilePath, err := h.aiClient.GenerateAudio(ctx, combinedText, deck.LanguageCode)
		if err != nil {
			fmt.Printf("Error generating combined audio: %v\n", err)
		} else {
			tempFile, err := os.Open(tempFilePath)
			if err != nil {
				fmt.Printf("Error opening temp file: %v\n", err)
			} else {
				defer tempFile.Close()
				defer os.Remove(tempFilePath)

				audioURL, err := h.storageProvider.UploadFile(
					ctx,
					tempFile,
					combinedAudioFileName,
					"audio/wav",
				)
				if err != nil {
					fmt.Printf("Error uploading combined audio: %v\n", err)
				} else {
					updatedFields.AudioExample = audioURL
				}
			}
		}
	}

	// Update card with generated content
	updatedFieldsJSON, err := json.Marshal(updatedFields)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize updated fields: %w", err)
	}

	if err := h.db.UpdateCardFields(card.ID, string(updatedFieldsJSON)); err != nil {
		return nil, fmt.Errorf("failed to update card: %w", err)
	}

	return updatedFields, nil
}
