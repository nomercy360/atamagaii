package handler_test

import (
	"atamagaii/internal/handler"
	"atamagaii/internal/testutils"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleAnkiImport(t *testing.T) {
	e := testutils.SetupHandlerDependencies(t)

	resp, err := testutils.AuthHelper(t, e, testutils.TelegramTestUserID, "mkkksim", "Maksim")
	if err != nil {
		t.Fatalf("Failed to authenticate: %v", err)
	}

	require.NotEmpty(t, resp.Token, "Expected non-empty JWT token")

	t.Run("SuccessfulImport", func(t *testing.T) {
		body, contentType := createMultipartFormWithFile(t, "test_deck.zip", "Test Deck")

		req := httptest.NewRequest(http.MethodPost, "/v1/decks/import/anki", bytes.NewReader(body.Bytes()))
		req.Header.Set(echo.HeaderContentType, contentType)
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+resp.Token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "Expected status OK")

		var importResp handler.AnkiImportResponse
		err := json.NewDecoder(rec.Body).Decode(&importResp)
		require.NoError(t, err, "Failed to parse response")

		require.Equal(t, "Test Deck", importResp.DeckName, "Deck name doesn't match")
		require.Equal(t, importResp.CardsAdded, 2, "Expected 2 cards to be added")
		require.Equal(t, importResp.MediaUploaded, 3, "Expected 3 media files to be uploaded")
		require.Empty(t, importResp.Errors, "Expected no errors")
	})

	t.Run("Unauthorized", func(t *testing.T) {
		body, contentType := createMultipartFormWithFile(t, "test_deck.zip", "Test Deck")

		req := httptest.NewRequest(http.MethodPost, "/v1/decks/import/anki", bytes.NewReader(body.Bytes()))
		req.Header.Set(echo.HeaderContentType, contentType)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code, "Expected unauthorized status")
	})

	t.Run("InvalidFileFormat", func(t *testing.T) {
		body, contentType := createMultipartFormWithInvalidFile(t, "Test Deck")

		req := httptest.NewRequest(http.MethodPost, "/v1/decks/import/anki", bytes.NewReader(body.Bytes()))
		req.Header.Set(echo.HeaderContentType, contentType)
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+resp.Token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code, "Expected bad request for invalid file format")
	})

	t.Run("NoFileProvided", func(t *testing.T) {

		body, contentType := createMultipartFormWithoutFile(t, "Test Deck")

		req := httptest.NewRequest(http.MethodPost, "/v1/decks/import/anki", bytes.NewReader(body.Bytes()))
		req.Header.Set(echo.HeaderContentType, contentType)
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+resp.Token)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code, "Expected bad request for no file")
	})
}

func createMultipartFormWithFile(t *testing.T, filename, deckName string) (bytes.Buffer, string) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	err := writer.WriteField("deck_name", deckName)
	require.NoError(t, err, "Failed to write deck_name field")

	err = writer.WriteField("language_code", deckName)
	require.NoError(t, err, "Failed to write language_code field")

	testFilePath := filepath.Join("../../testdata", filename)
	fileContents, err := os.ReadFile(testFilePath)
	require.NoError(t, err, fmt.Sprintf("Failed to read test file %s", testFilePath))

	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err, "Failed to create form file")

	_, err = io.Copy(part, bytes.NewReader(fileContents))
	require.NoError(t, err, "Failed to copy file contents")

	err = writer.Close()
	require.NoError(t, err, "Failed to close writer")

	return body, writer.FormDataContentType()
}

func createMultipartFormWithInvalidFile(t *testing.T, deckName string) (bytes.Buffer, string) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	err := writer.WriteField("deck_name", deckName)
	require.NoError(t, err, "Failed to write deck_name field")

	part, err := writer.CreateFormFile("file", "invalid.txt")
	require.NoError(t, err, "Failed to create form file")

	_, err = io.Copy(part, strings.NewReader("This is not a zip file"))
	require.NoError(t, err, "Failed to copy file contents")

	err = writer.Close()
	require.NoError(t, err, "Failed to close writer")

	return body, writer.FormDataContentType()
}

func createMultipartFormWithoutFile(t *testing.T, deckName string) (bytes.Buffer, string) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	err := writer.WriteField("deck_name", deckName)
	require.NoError(t, err, "Failed to write deck_name field")

	err = writer.Close()
	require.NoError(t, err, "Failed to close writer")

	return body, writer.FormDataContentType()
}
