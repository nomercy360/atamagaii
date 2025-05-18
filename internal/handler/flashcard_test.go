package handler_test

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db"
	"atamagaii/internal/testutils"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"net/http"
	"testing"
	"time"
)

func daysToDuration(days float64) time.Duration {
	return time.Duration(days * 24 * float64(time.Hour))
}

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

func TestGetDueCards(t *testing.T) {
	e := testutils.SetupHandlerDependencies(t)

	resp, err := testutils.AuthHelper(t, e, testutils.TelegramTestUserID, "mkkksim", "Maksim")
	if err != nil {
		t.Fatalf("Failed to authenticate: %v", err)
	}

	reqBody := map[string]string{
		"name":        "Test Vocabulary",
		"description": "Test deck for due cards",
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

	rec = testutils.PerformRequest(
		t,
		e,
		http.MethodGet,
		"/v1/cards/due?deck_id="+deck.ID,
		"",
		resp.Token,
		http.StatusOK,
	)

	cards := testutils.ParseResponse[[]contract.CardResponse](t, rec)

	if len(cards) == 0 {
		t.Error("Expected at least one due card")
	}

	rec = testutils.PerformRequest(
		t,
		e,
		http.MethodGet,
		"/v1/cards/due?deck_id="+deck.ID+"&limit=5",
		"",
		resp.Token,
		http.StatusOK,
	)

	limitedCards := testutils.ParseResponse[[]contract.CardResponse](t, rec)

	if len(limitedCards) > 5 {
		t.Errorf("Expected at most 5 cards, got %d", len(limitedCards))
	}

	rec = testutils.PerformRequest(
		t,
		e,
		http.MethodGet,
		"/v1/cards/due",
		"",
		resp.Token,
		http.StatusBadRequest,
	)

	errorResp := testutils.ParseResponse[contract.ErrorResponse](t, rec)
	if errorResp.Error == "" {
		t.Error("Expected non-empty error message for missing deck_id")
	}

	rec = testutils.PerformRequest(
		t,
		e,
		http.MethodGet,
		"/v1/cards/due?deck_id=nonexistent",
		"",
		resp.Token,
		http.StatusNotFound,
	)

	errorResp = testutils.ParseResponse[contract.ErrorResponse](t, rec)
	if errorResp.Error == "" {
		t.Error("Expected non-empty error message for non-existent deck")
	}
}

func TestGetDecksWithDueCards(t *testing.T) {
	e := testutils.SetupHandlerDependencies(t)

	// Authenticate user
	resp, err := testutils.AuthHelper(t, e, testutils.TelegramTestUserID, "mkkksim", "Maksim")
	if err != nil {
		t.Fatalf("Failed to authenticate: %v", err)
	}

	// Import a deck
	reqBody := map[string]string{
		"name":        "Test Deck for Due Cards",
		"description": "Testing due cards in deck listing",
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

	// Get decks list
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

	found := false
	for _, d := range decks {
		if d.ID == deck.ID {
			found = true

			dueCardsURL := fmt.Sprintf("/v1/cards/due?deck_id=%s", d.ID)
			dueRec := testutils.PerformRequest(
				t,
				e,
				http.MethodGet,
				dueCardsURL,
				"",
				resp.Token,
				http.StatusOK,
			)

			dueCards := testutils.ParseResponse[[]contract.CardResponse](t, dueRec)

			expectedCount := len(dueCards)
			if expectedCount > d.NewCardsPerDay {
				expectedCount = d.NewCardsPerDay
			}
			break
		}
	}

	if !found {
		t.Error("Newly created deck not found in the deck list")
	}
}

func TestDeckMetrics(t *testing.T) {
	e := testutils.SetupHandlerDependencies(t)

	// Authenticate user
	resp, err := testutils.AuthHelper(t, e, testutils.TelegramTestUserID, "mkkksim", "Maksim")
	if err != nil {
		t.Fatalf("Failed to authenticate: %v", err)
	}

	// Import a deck
	reqBody := map[string]string{
		"name":        "Test Deck for Metrics",
		"description": "Testing card metrics in deck",
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

	importedDeck := testutils.ParseResponse[db.Deck](t, rec)

	// Get the deck to verify metrics are populated
	rec = testutils.PerformRequest(
		t,
		e,
		http.MethodGet,
		"/v1/decks/"+importedDeck.ID,
		"",
		resp.Token,
		http.StatusOK,
	)

	deck := testutils.ParseResponse[db.Deck](t, rec)

	if deck.Stats == nil {
		t.Errorf("Expected deck to have stats")
	} else {
		if deck.Stats.NewCards <= 0 {
			t.Errorf("Expected deck to have new cards, got %d", deck.Stats.NewCards)
		}

		if deck.Stats.LearningCards != 0 {
			t.Errorf("Expected learning cards to be 0 initially, got %d", deck.Stats.LearningCards)
		}

		if deck.Stats.ReviewCards != 0 {
			t.Errorf("Expected review cards to be 0 initially, got %d", deck.Stats.ReviewCards)
		}
	}

	// Get due cards
	rec = testutils.PerformRequest(
		t,
		e,
		http.MethodGet,
		"/v1/cards/due?deck_id="+deck.ID+"&limit=1",
		"",
		resp.Token,
		http.StatusOK,
	)

	dueCards := testutils.ParseResponse[[]contract.CardResponse](t, rec)
	if len(dueCards) == 0 {
		t.Fatal("Expected at least one due card")
	}

	// Review a card to move it to learning status
	reviewReq := map[string]interface{}{
		"card_id":       dueCards[0].ID,
		"rating":        2, // Hard - should move to learning
		"time_spent_ms": 3000,
	}
	reviewBody, _ := json.Marshal(reviewReq)

	testutils.PerformRequest(
		t,
		e,
		http.MethodPost,
		"/v1/cards/"+dueCards[0].ID+"/review",
		string(reviewBody),
		resp.Token,
		http.StatusOK,
	)

	// Get the deck again to check updated metrics
	rec = testutils.PerformRequest(
		t,
		e,
		http.MethodGet,
		"/v1/decks/"+deck.ID,
		"",
		resp.Token,
		http.StatusOK,
	)

	updatedDeck := testutils.ParseResponse[db.Deck](t, rec)

	if updatedDeck.Stats != nil && deck.Stats != nil {
		if updatedDeck.Stats.NewCards >= deck.Stats.NewCards {
			t.Errorf("Expected new cards to decrease after review, was %d, now %d",
				deck.Stats.NewCards, updatedDeck.Stats.NewCards)
		}
	} else {
		t.Errorf("Stats missing from deck or updated deck")
	}

	if updatedDeck.Stats != nil {
		if updatedDeck.Stats.LearningCards < 1 {
			t.Errorf("Expected learning cards to increase after review, got %d", updatedDeck.Stats.LearningCards)
		}
	} else {
		t.Errorf("Stats missing from updated deck")
	}
}

func strPtr(s string) *string {
	return &s
}
