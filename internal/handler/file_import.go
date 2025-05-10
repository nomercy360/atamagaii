package handler

import (
	"atamagaii/internal/db"
	"atamagaii/internal/utils"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"os"
	"path/filepath"
)

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

	materialsDir, err := utils.FindDirUp("materials", 3)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to find materials directory")
	}

	filePath := filepath.Join(materialsDir, req.FileName)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("File %s does not exist (looked in %s)", req.FileName, materialsDir))
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to read file: %v", err))
	}

	var vocabularyItems []db.VocabularyItem
	if err := json.Unmarshal(fileData, &vocabularyItems); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to parse vocabulary data: %v", err))
	}

	var level string
	if len(req.FileName) >= 8 && req.FileName[:7] == "vocab_n" {
		level = req.FileName[6:8]
	} else {
		level = "Unknown"
	}

	// Default to Japanese for predefined vocabulary
	languageCode := "ja"
	transcriptionType := "furigana"

	deck, err := h.db.CreateDeck(userID, req.Name, req.Description, level, languageCode, transcriptionType)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to create deck: %v", err))
	}

	fieldsArray := make([]string, len(vocabularyItems))

	for i, item := range vocabularyItems {
		fieldsContent := map[string]interface{}{
			"term":                       item.Term,
			"transcription":              item.Transcription,
			"term_with_transcription":    item.TermWithTranscription,
			"meaning_en":                 item.MeaningEn,
			"meaning_ru":                 item.MeaningRu,
			"example_native":             item.ExampleNative,
			"example_en":                 item.ExampleEn,
			"example_ru":                 item.ExampleRu,
			"example_with_transcription": item.ExampleWithTranscription,
			"frequency":                  item.Frequency,
			"language_code":              languageCode,
			"transcription_type":         transcriptionType,
			"audio_word":                 item.AudioWord,
			"audio_example":              item.AudioExample,
			"image_url":                  item.ImageURL,
		}
		fieldsJSON, _ := json.Marshal(fieldsContent)
		fieldsArray[i] = string(fieldsJSON)
	}

	if err := h.db.AddCardsInBatch(userID, deck.ID, fieldsArray); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to add cards to deck: %v", err))
	}

	return c.JSON(http.StatusCreated, deck)
}
