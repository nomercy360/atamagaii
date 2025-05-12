package handler

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db"
	"encoding/json"
	"errors"
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

	card, err := h.db.GetCard(req.CardID, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Card not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch card").WithInternal(err)
	}

	deck, err := h.db.GetDeck(req.DeckID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Deck not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch deck").WithInternal(err)
	}

	if card.UserID != userID || card.DeckID != req.DeckID {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	var fields contract.CardFields
	if err := json.Unmarshal([]byte(card.Fields), &fields); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to parse card fields").WithInternal(err)
	}

	if fields.Term == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Card must have a term field")
	}

	updatedFields, err := h.openaiClient.GenerateCardContent(c.Request().Context(), fields.Term, deck.LanguageCode)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to generate card content").WithInternal(err)
	}

	if updatedFields.AudioWord == "" {
		termAudioFileName := fmt.Sprintf("%s_term.m4a", req.CardID)
		tempFilePath, err := h.openaiClient.GenerateAudio(c.Request().Context(), updatedFields.Term, deck.LanguageCode)
		if err != nil {

			fmt.Printf("Error generating term audio: %v\n", err)
		} else {

			tempFile, err := os.Open(tempFilePath)
			if err != nil {
				fmt.Printf("Error opening temp file: %v\n", err)
			} else {
				defer tempFile.Close()
				defer os.Remove(tempFilePath)

				audioURL, err := h.storageProvider.UploadFile(
					c.Request().Context(),
					tempFile,
					termAudioFileName,
					"audio/aac",
				)
				if err != nil {
					fmt.Printf("Error uploading term audio: %v\n", err)
				} else {
					updatedFields.AudioWord = audioURL
				}
			}
		}
	}

	if updatedFields.AudioExample == "" && updatedFields.ExampleNative != "" {
		exampleAudioFileName := fmt.Sprintf("%s_example.m4a", req.CardID)
		tempFilePath, err := h.openaiClient.GenerateAudio(c.Request().Context(), updatedFields.ExampleNative, deck.LanguageCode)
		if err != nil {
			fmt.Printf("Error generating example audio: %v\n", err)
		} else {
			tempFile, err := os.Open(tempFilePath)
			if err != nil {
				fmt.Printf("Error opening temp file: %v\n", err)
			} else {
				defer tempFile.Close()
				defer os.Remove(tempFilePath)

				audioURL, err := h.storageProvider.UploadFile(
					c.Request().Context(),
					tempFile,
					exampleAudioFileName,
					"audio/aac",
				)
				if err != nil {
					fmt.Printf("Error uploading example audio: %v\n", err)
				} else {
					updatedFields.AudioExample = audioURL
				}
			}
		}
	}

	updatedFieldsJSON, err := json.Marshal(updatedFields)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to serialize card fields").WithInternal(err)
	}

	if err := h.db.UpdateCardFields(req.CardID, string(updatedFieldsJSON)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update card").WithInternal(err)
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
