package handler

import (
	"atamagaii/internal/anki"
	"atamagaii/internal/contract"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
)

// AnkiImportRequest represents a request to import an Anki deck
type AnkiImportRequest struct {
	DeckName string `form:"deck_name"`
}

// AnkiImportResponse represents the response after importing an Anki deck
type AnkiImportResponse struct {
	DeckName      string   `json:"deck_name"`
	CardsAdded    int      `json:"cards_added"`
	MediaUploaded int      `json:"media_uploaded"`
	Errors        []string `json:"errors,omitempty"`
}

// HandleAnkiImport handles the import of an Anki deck
func (h *Handler) HandleAnkiImport(c echo.Context) error {
	// Get user ID from token
	userID, err := GetUserIDFromToken(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
	}

	// Parse form
	var request AnkiImportRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request format")
	}

	// Get file from form
	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "No file provided")
	}

	// Check if file is a zip file
	if filepath.Ext(file.Filename) != ".zip" && filepath.Ext(file.Filename) != ".apkg" {
		return echo.NewHTTPError(http.StatusBadRequest, "File must be a .zip or .apkg file")
	}

	// Save the uploaded file temporarily
	tempFile, err := os.CreateTemp("", "anki-import-*.zip")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error saving upload")
	}
	defer os.Remove(tempFile.Name()) // Clean up

	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error opening upload")
	}
	defer src.Close()

	// Copy to temp file
	if _, err = io.Copy(tempFile, src); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error copying upload")
	}
	tempFile.Close()

	// Create Anki processor
	processor := anki.NewProcessor(h.db, h.storageProvider)

	// Process the import
	result, err := processor.ImportDeck(c.Request().Context(), userID, request.DeckName, tempFile.Name())
	if err != nil {
		// Add the main error to the result errors
		if result == nil {
			result = &anki.ImportResult{
				Errors: []string{err.Error()},
			}
		} else {
			result.Errors = append(result.Errors, err.Error())
		}
	}

	// Return success even with partial errors
	return c.JSON(http.StatusOK, AnkiImportResponse{
		DeckName:      result.DeckName,
		CardsAdded:    result.CardsAdded,
		MediaUploaded: result.MediaUploaded,
		Errors:        result.Errors,
	})
}

// AddAnkiImportRoutes adds the Anki import routes to the given group
func (h *Handler) AddAnkiImportRoutes(g *echo.Group) {
	g.POST("/decks/import/anki", h.HandleAnkiImport)
}
