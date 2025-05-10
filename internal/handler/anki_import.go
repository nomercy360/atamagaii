package handler

import (
	"atamagaii/internal/anki"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
)

type AnkiImportRequest struct {
	DeckName string `form:"deck_name"`
}

type AnkiImportResponse struct {
	DeckName      string   `json:"deck_name"`
	CardsAdded    int      `json:"cards_added"`
	MediaUploaded int      `json:"media_uploaded"`
	Errors        []string `json:"errors,omitempty"`
}

func (h *Handler) HandleAnkiImport(c echo.Context) error {
	userID, err := GetUserIDFromToken(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
	}

	var request AnkiImportRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request format")
	}

	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "No file provided")
	}

	if filepath.Ext(file.Filename) != ".zip" {
		return echo.NewHTTPError(http.StatusBadRequest, "File must be a .zip or .apkg file")
	}

	tempFile, err := os.CreateTemp("", "anki-import-*.zip")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error saving upload")
	}
	defer os.Remove(tempFile.Name())

	src, err := file.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error opening upload")
	}
	defer src.Close()

	if _, err = io.Copy(tempFile, src); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error copying upload")
	}
	tempFile.Close()

	processor := anki.NewProcessor(h.db, h.storageProvider)

	result, err := processor.ImportDeck(c.Request().Context(), userID, request.DeckName, tempFile.Name())
	if err != nil {
		if result == nil {
			result = &anki.ImportResult{
				Errors: []string{err.Error()},
			}
		} else {
			result.Errors = append(result.Errors, err.Error())
		}
	}

	return c.JSON(http.StatusOK, AnkiImportResponse{
		DeckName:      result.DeckName,
		CardsAdded:    result.CardsAdded,
		MediaUploaded: result.MediaUploaded,
		Errors:        result.Errors,
	})
}

func (h *Handler) AddAnkiImportRoutes(g *echo.Group) {
	g.POST("/decks/import/anki", h.HandleAnkiImport)
}
