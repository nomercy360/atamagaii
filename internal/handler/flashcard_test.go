package handler_test

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db"
	"atamagaii/internal/testutils"
	"encoding/json"
	_ "github.com/mattn/go-sqlite3"
	"net/http"
	"testing"
)

func TestImportDeckFromFile(t *testing.T) {
	e := testutils.SetupHandlerDependencies(t)

	resp, err := testutils.AuthHelper(t, e, testutils.TelegramTestUserID, "mkkksim", "Maksim")
	if err != nil {
		t.Fatalf("Failed to authenticate: %v", err)
	}

	if resp.Token == "" {
		t.Error("Expected non-empty JWT token")
	}

	reqBody := map[string]string{
		"name":        "N5 Vocabulary",
		"description": "Basic Japanese vocabulary for JLPT N5 level",
		"file_name":   "vocab_n5.json",
	}
	body, _ := json.Marshal(reqBody)

	rec := testutils.PerformRequest(
		t,
		e,
		http.MethodPost,
		"/v1/decks/import",
		string(body),
		resp.Token,
		http.StatusCreated,
	)

	deck := testutils.ParseResponse[db.Deck](t, rec)

	if deck.ID == "" {
		t.Error("Expected non-empty deck ID")
	}

	if deck.Name != "N5 Vocabulary" {
		t.Errorf("Expected deck name 'N5 Vocabulary', got '%s'", deck.Name)
	}

	if deck.Description != "Basic Japanese vocabulary for JLPT N5 level" {
		t.Errorf("Expected deck description 'Basic Japanese vocabulary for JLPT N5 level', got '%s'", deck.Description)
	}

	if deck.Level != "N5" {
		t.Errorf("Expected deck level 'N5', got '%s'", deck.Level)
	}

	if deck.UserID != resp.User.ID {
		t.Errorf("Expected deck user ID '%s', got '%s'", resp.User.ID, deck.UserID)
	}

	rec = testutils.PerformRequest(
		t,
		e,
		http.MethodGet,
		"/v1/decks",
		"",
		resp.Token,
		http.StatusOK,
	)

	decks := testutils.ParseResponse[[]db.Deck](t, rec)

	if len(decks) == 0 {
		t.Error("Expected at least one deck in the list")
	}

	found := false
	for _, d := range decks {
		if d.ID == deck.ID {
			found = true
			break
		}
	}

	if !found {
		t.Error("Newly created deck not found in the deck list")
	}

	rec = testutils.PerformRequest(
		t,
		e,
		http.MethodGet,
		"/v1/decks/"+deck.ID,
		"",
		resp.Token,
		http.StatusOK,
	)

	retrievedDeck := testutils.ParseResponse[db.Deck](t, rec)

	if retrievedDeck.ID != deck.ID {
		t.Errorf("Expected deck ID '%s', got '%s'", deck.ID, retrievedDeck.ID)
	}

	if retrievedDeck.Name != deck.Name {
		t.Errorf("Expected deck name '%s', got '%s'", deck.Name, retrievedDeck.Name)
	}

	rec = testutils.PerformRequest(
		t,
		e,
		http.MethodGet,
		"/v1/stats?deck_id="+deck.ID,
		"",
		resp.Token,
		http.StatusOK,
	)

	stats := testutils.ParseResponse[map[string]interface{}](t, rec)

	_, exists := stats["due_cards"]
	if !exists {
		t.Error("Expected 'due_cards' in stats")
	}
}

func TestImportDeckFromFile_InvalidFilename(t *testing.T) {
	e := testutils.SetupHandlerDependencies(t)

	resp, err := testutils.AuthHelper(t, e, testutils.TelegramTestUserID, "mkkksim", "Maksim")
	if err != nil {
		t.Fatalf("Failed to authenticate: %v", err)
	}

	reqBody := map[string]string{
		"name":        "Non-existent Deck",
		"description": "This should fail",
		"file_name":   "nonexistent_file.json",
	}
	body, _ := json.Marshal(reqBody)

	rec := testutils.PerformRequest(
		t,
		e,
		http.MethodPost,
		"/v1/decks/import",
		string(body),
		resp.Token,
		http.StatusBadRequest,
	)

	errorResp := testutils.ParseResponse[contract.ErrorResponse](t, rec)

	if errorResp.Error == "" {
		t.Error("Expected non-empty error message")
	}
}

func strPtr(s string) *string {
	return &s
}
