package handler_test

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db"
	"atamagaii/internal/testutils"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestReviewCard(t *testing.T) {
	e := testutils.SetupHandlerDependencies(t)

	resp, err := testutils.AuthHelper(t, e, testutils.TelegramTestUserID, "mkkksim", "Maksim")
	if err != nil {
		t.Fatalf("Failed to authenticate: %v", err)
	}

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
	now := time.Now()

	reviewAndCheck := func(t *testing.T, rating int, expectedState db.CardState, expectedInterval int, expectedEase float64, expectedNextReview time.Time, expectedReviewCount int, expectedLapsCount int) {
		reviewData := map[string]interface{}{
			"card_id":       cardID,
			"rating":        rating,
			"time_spent_ms": 3000,
		}
		reviewBody, _ := json.Marshal(reviewData)

		rec := testutils.PerformRequest(
			t,
			e,
			http.MethodPost,
			"/v1/cards/"+cardID+"/review",
			string(reviewBody),
			resp.Token,
			http.StatusOK,
		)

		progress := testutils.ParseResponse[db.CardProgress](t, rec)

		// Assertions
		assert.Equal(t, expectedState, progress.State, "Unexpected state")
		assert.Equal(t, expectedInterval, progress.Interval, "Unexpected interval")
		assert.InDelta(t, expectedEase, progress.Ease, 0.01, "Unexpected ease")
		assert.Equal(t, expectedReviewCount, progress.ReviewCount, "Unexpected review count")
		assert.Equal(t, expectedLapsCount, progress.LapsCount, "Unexpected laps count")
		if progress.NextReview == nil {
			t.Fatal("Expected next review date to be set")
		}
		assert.WithinDuration(t, expectedNextReview, *progress.NextReview, time.Second, "Unexpected next review date")
	}

	// 3. Test Anki-like review flow

	// Step 1: New card, Good (moves to Learning, step 1)
	reviewAndCheck(t, 3, db.StateLearning, 10, 2.5, now.Add(10*time.Minute), 1, 0)

	// Step 2: Learning step 2, Good (graduates to Review)
	now = now.Add(10 * time.Minute)
	reviewAndCheck(t, 3, db.StateReview, 1, 2.5, now.Add(24*time.Hour), 2, 0)

	// Step 3: Review, Good (interval = 4 days)
	now = now.Add(24 * time.Hour)
	reviewAndCheck(t, 3, db.StateReview, 4, 2.5, now.Add(4*24*time.Hour), 3, 0)

	// Step 4: Review, Good (interval = 4 × 2.5 = 10 days)
	now = now.Add(4 * 24 * time.Hour)
	reviewAndCheck(t, 3, db.StateReview, 10, 2.5, now.Add(10*24*time.Hour), 4, 0)

	// Step 5: Review, Again (moves to Relearning)
	now = now.Add(10 * 24 * time.Hour)
	reviewAndCheck(t, 1, db.StateRelearning, 10, 1.7, now.Add(10*time.Minute), 5, 1)

	// Step 6: Relearning, Good (graduates back to Review)
	now = now.Add(10 * time.Minute)
	reviewAndCheck(t, 3, db.StateReview, 1, 1.7, now.Add(24*time.Hour), 6, 1)

	// Step 7: Review, Easy (interval = 1 × 1.7 × 1.3 ≈ 2 days)
	now = now.Add(24 * time.Hour)
	reviewAndCheck(t, 4, db.StateReview, 2, 1.85, now.Add(2*24*time.Hour), 7, 1)
}
