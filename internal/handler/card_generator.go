package handler

import (
	"atamagaii/internal/ai"
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

	content, err := h.openaiClient.GenerateCardContent(c.Request().Context(), fields.Term, deck.LanguageCode)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to generate card content").WithInternal(err)
	}

	var updatedFields contract.CardFields
	if card, ok := content["card"].(ai.VocabularyCard); ok {

		updatedFields = convertVocabularyCardToFields(card, fields)
	} else {

		var err error
		updatedFields, err = parseCardContent(content["raw_response"].(string), fields)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to parse generated content").WithInternal(err)
		}
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

func convertVocabularyCardToFields(card ai.VocabularyCard, existingFields contract.CardFields) contract.CardFields {
	return contract.CardFields{
		Term:                     card.Term,
		Transcription:            card.Transcription,
		TermWithTranscription:    card.TermWithTranscription,
		MeaningEn:                card.MeaningEn,
		MeaningRu:                card.MeaningRu,
		ExampleNative:            card.ExampleNative,
		ExampleEn:                card.ExampleEn,
		ExampleRu:                card.ExampleRu,
		ExampleWithTranscription: card.ExampleWithTranscription,
		Frequency:                card.Frequency,
		AudioWord:                existingFields.AudioWord,
		AudioExample:             existingFields.AudioExample,
		ImageURL:                 existingFields.ImageURL,
	}
}

func parseCardContent(contentStr string, existingFields contract.CardFields) (contract.CardFields, error) {

	var generatedContent map[string]interface{}
	if err := json.Unmarshal([]byte(contentStr), &generatedContent); err != nil {
		return existingFields, fmt.Errorf("error unmarshaling generated content: %w", err)
	}

	generatedJSON, err := json.Marshal(generatedContent)
	if err != nil {
		return existingFields, fmt.Errorf("error marshaling generated content: %w", err)
	}

	var newFields contract.CardFields
	if err := json.Unmarshal(generatedJSON, &newFields); err != nil {
		return existingFields, fmt.Errorf("error unmarshaling to card fields: %w", err)
	}

	if existingFields.AudioWord != "" {
		newFields.AudioWord = existingFields.AudioWord
	}
	if existingFields.AudioExample != "" {
		newFields.AudioExample = existingFields.AudioExample
	}
	if existingFields.ImageURL != "" {
		newFields.ImageURL = existingFields.ImageURL
	}

	if newFields.Term == "" {
		newFields.Term = existingFields.Term
	}

	return newFields, nil
}
