package handler_test

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db"
	"atamagaii/internal/testutils"
	"encoding/json"
	_ "github.com/mattn/go-sqlite3"
	"net/http"
	"testing"
	"time"
)

func TestReviewCard(t *testing.T) {
	e := testutils.SetupHandlerDependencies(t)

	// Authenticate user
	resp, err := testutils.AuthHelper(t, e, testutils.TelegramTestUserID, "mkkksim", "Maksim")
	if err != nil {
		t.Fatalf("Failed to authenticate: %v", err)
	}

	// 1. Create a deck with cards
	reqBody := map[string]string{
		"name":        "Test Review Deck",
		"description": "Deck for testing review functionality",
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

	// 2. Get due cards
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
		t.Fatal("Expected at least one due card")
	}

	cardID := cards[0].ID

	// 3. Test review with different ratings

	// 3.1 Rate as "Again" (1)
	reviewData := map[string]interface{}{
		"card_id":      cardID,
		"rating":       1,
		"time_spent_ms": 3000,
	}
	reviewBody, _ := json.Marshal(reviewData)

	rec = testutils.PerformRequest(
		t,
		e,
		http.MethodPost,
		"/v1/cards/"+cardID+"/review",
		string(reviewBody),
		resp.Token,
		http.StatusOK,
	)

	progress := testutils.ParseResponse[db.CardProgress](t, rec)

	// Verify the progress was recorded correctly for rating 1 (Again)
	if progress.Interval != 1 {
		t.Errorf("Expected interval to be reset to 1 for 'Again' rating, got %d", progress.Interval)
	}
	if progress.ReviewCount != 1 {
		t.Errorf("Expected review count to be 1, got %d", progress.ReviewCount)
	}
	if progress.LapsCount != 1 {
		t.Errorf("Expected laps count to be 1 for 'Again' rating, got %d", progress.LapsCount)
	}
	if progress.NextReview == nil {
		t.Error("Expected next review date to be set")
	} else {
		expectedDate := time.Now().AddDate(0, 0, 1)
		// Allow a few minutes of difference
		timeDiff := expectedDate.Sub(*progress.NextReview)
		if timeDiff < -5*time.Minute || timeDiff > 5*time.Minute {
			t.Errorf("Expected next review to be around %v, got %v", expectedDate, *progress.NextReview)
		}
	}

	// 3.2 Rate as "Good" (3)
	reviewData = map[string]interface{}{
		"card_id":      cardID,
		"rating":       3,
		"time_spent_ms": 2000,
	}
	reviewBody, _ = json.Marshal(reviewData)

	rec = testutils.PerformRequest(
		t,
		e,
		http.MethodPost,
		"/v1/cards/"+cardID+"/review",
		string(reviewBody),
		resp.Token,
		http.StatusOK,
	)

	progress = testutils.ParseResponse[db.CardProgress](t, rec)

	// Verify the progress was updated correctly for rating 3 (Good)
	if progress.Interval <= 1 {
		t.Errorf("Expected interval to increase for 'Good' rating, got %d", progress.Interval)
	}
	if progress.ReviewCount != 2 {
		t.Errorf("Expected review count to be 2, got %d", progress.ReviewCount)
	}
	if progress.NextReview == nil {
		t.Error("Expected next review date to be set")
	} else {
		expectedDate := time.Now().AddDate(0, 0, progress.Interval)
		// Allow a few minutes of difference
		timeDiff := expectedDate.Sub(*progress.NextReview)
		if timeDiff < -5*time.Minute || timeDiff > 5*time.Minute {
			t.Errorf("Expected next review to be around %v, got %v", expectedDate, *progress.NextReview)
		}
	}

	// 4. Test invalid card ID
	rec = testutils.PerformRequest(
		t,
		e,
		http.MethodPost,
		"/v1/cards/nonexistent-card/review",
		string(reviewBody),
		resp.Token,
		http.StatusNotFound,
	)

	errorResp := testutils.ParseResponse[contract.ErrorResponse](t, rec)
	if errorResp.Error == "" {
		t.Error("Expected non-empty error message for non-existent card")
	}

	// 5. Test invalid rating
	invalidRatingData := map[string]interface{}{
		"card_id":      cardID,
		"rating":       0, // Invalid rating
		"time_spent_ms": 2000,
	}
	invalidRatingBody, _ := json.Marshal(invalidRatingData)

	rec = testutils.PerformRequest(
		t,
		e,
		http.MethodPost,
		"/v1/cards/"+cardID+"/review",
		string(invalidRatingBody),
		resp.Token,
		http.StatusBadRequest,
	)

	errorResp = testutils.ParseResponse[contract.ErrorResponse](t, rec)
	if errorResp.Error == "" {
		t.Error("Expected non-empty error message for invalid rating")
	}

	// 6. Test invalid request (missing required fields)
	invalidRequestData := map[string]interface{}{
		"card_id":      cardID,
		"time_spent_ms": 2000,
		// Missing rating
	}
	invalidRequestBody, _ := json.Marshal(invalidRequestData)

	rec = testutils.PerformRequest(
		t,
		e,
		http.MethodPost,
		"/v1/cards/"+cardID+"/review",
		string(invalidRequestBody),
		resp.Token,
		http.StatusBadRequest,
	)

	errorResp = testutils.ParseResponse[contract.ErrorResponse](t, rec)
	if errorResp.Error == "" {
		t.Error("Expected non-empty error message for invalid request")
	}
}