package handler_test

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db" // Assuming db constants are accessible (and updated for 2-button system)
	"atamagaii/internal/testutils"
	"encoding/json"
	"math"
	"net/http"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewCard_SequentialFlow_TwoButton(t *testing.T) {
	e := testutils.SetupHandlerDependencies(t) // Sets up router and db store

	// 1. Authenticate and Import Deck
	resp, err := testutils.AuthHelper(t, e, testutils.TelegramTestUserID, "mkkksim", "Maksim")
	require.NoError(t, err, "Failed to authenticate")

	reqBody := map[string]string{
		"name":        "Test Review Deck 2-Button",
		"description": "Deck for testing 2-button review functionality",
		"file_name":   "vocab_n5.json", // Ensure this file exists and has at least 2 cards
	}
	body, _ := json.Marshal(reqBody)

	rec := testutils.PerformRequest(
		t, e, http.MethodPost, "/v1/decks/import", string(body), resp.Token, http.StatusCreated,
	)
	deck := testutils.ParseResponse[db.Deck](t, rec)

	// 2. Get a due card
	rec = testutils.PerformRequest(
		t, e, http.MethodGet, "/v1/cards/due?deck_id="+deck.ID, "", resp.Token, http.StatusOK,
	)
	cards := testutils.ParseResponse[[]contract.CardResponse](t, rec)
	require.NotEmpty(t, cards, "Expected at least one due card from imported deck")
	cardID := cards[0].ID

	// reviewAndCheck helper function (remains largely the same, but rating input will be db.RatingAgain or db.RatingGood)
	reviewAndCheck := func(
		t *testing.T,
		stepName string,
		currentCardID string,
		rating int, // This will be db.RatingAgain (1) or db.RatingGood (2)
		expectedState db.CardState,
		expectedBaseIntervalNoFuzz time.Duration,
		expectedEase float64,
		expectedReviewCount int,
		expectedLapsCount int,
	) {
		t.Helper()
		reviewTime := time.Now()

		reviewData := map[string]interface{}{
			// "card_id" is now part of the path, not body for this endpoint structure
			"rating":        rating,
			"time_spent_ms": 3000,
		}
		reviewBody, _ := json.Marshal(reviewData)

		endpoint := "/v1/cards/" + currentCardID + "/review"
		rec := testutils.PerformRequest(t, e, http.MethodPost, endpoint, string(reviewBody), resp.Token, http.StatusOK)
		progress := testutils.ParseResponse[db.CardProgress](t, rec)

		assert.Equal(t, expectedState, progress.State, "[%s] Unexpected state", stepName)
		assert.InDelta(t, expectedEase, progress.Ease, 0.001, "[%s] Unexpected ease", stepName)
		assert.Equal(t, expectedReviewCount, progress.ReviewCount, "[%s] Unexpected review count", stepName)
		assert.Equal(t, expectedLapsCount, progress.LapsCount, "[%s] Unexpected laps count", stepName)

		if expectedBaseIntervalNoFuzz > 24*time.Hour && db.FuzzPercentage > 0.0 {
			fuzzDeltaSeconds := expectedBaseIntervalNoFuzz.Seconds()*db.FuzzPercentage + 1.0
			assert.InDelta(t, expectedBaseIntervalNoFuzz.Seconds(), progress.Interval.Seconds(), fuzzDeltaSeconds,
				"[%s] Unexpected interval (expected %v, got %v, considering fuzz)", stepName, expectedBaseIntervalNoFuzz, progress.Interval)
		} else {
			// For very short intervals (like learning steps), direct comparison is fine.
			// Also for 1-day intervals where fuzz might not apply or be negligible.
			assert.Equal(t, expectedBaseIntervalNoFuzz.Round(time.Second), progress.Interval.Round(time.Second),
				"[%s] Unexpected interval (expected %v, got %v)", stepName, expectedBaseIntervalNoFuzz, progress.Interval)
		}

		require.NotNil(t, progress.NextReview, "[%s] Expected next review date to be set", stepName)
		expectedNextReviewBasedOnActualInterval := reviewTime.Add(progress.Interval)
		assert.WithinDuration(t, expectedNextReviewBasedOnActualInterval, *progress.NextReview, 15*time.Second,
			"[%s] NextReview (%v) not close to reviewTime + actual progress.Interval (%v)", stepName, *progress.NextReview, expectedNextReviewBasedOnActualInterval)

		expectedNextReviewBasedOnUnfuzzed := reviewTime.Add(expectedBaseIntervalNoFuzz)
		maxDeviation := time.Minute
		if expectedBaseIntervalNoFuzz > 24*time.Hour && db.FuzzPercentage > 0.0 {
			maxDeviation = time.Duration(expectedBaseIntervalNoFuzz.Seconds()*db.FuzzPercentage*1.2) + time.Minute
		} else if expectedBaseIntervalNoFuzz > 0 { // For learning steps, ensure it's reasonably close
			maxDeviation = 10 * time.Second
		}
		assert.WithinDuration(t, expectedNextReviewBasedOnUnfuzzed, *progress.NextReview, maxDeviation,
			"[%s] NextReview (%v) too far from expected (unfuzzed) %v", stepName, *progress.NextReview, expectedNextReviewBasedOnUnfuzzed)
	}

	// --- Test Case Sequence for cardID (cards[0].ID) ---
	// Initial state: New card, Ease: db.DefaultEase (e.g., 2.5), ReviewCount: 0, LapsCount: 0

	currentEase := db.DefaultEase // Will be 2.5 if db.DefaultEase is 2.5

	// 1. New Card -> "I Know"
	// Action: Rate db.RatingGood (2)
	// Result: Learning, Step 1 advanced (from 0 to 1), Interval = LearningStep2Duration
	// Ease: db.DefaultEase (2.5 + db.EaseAdjIKnow (0.0) = 2.5)
	reviewAndCheck(t, "1. New->IKnow", cardID, db.RatingGood,
		db.StateLearning, db.LearningStep2Duration, // e.g., 10m
		currentEase, // 2.5
		1, 0)

	// 2. Learning (Step 1, e.g. 10m) -> "I Know"
	// Action: Rate db.RatingGood (2)
	// Result: Graduate to Review, Interval = GraduateToReviewIntervalDays (e.g. 1 day)
	// Ease: db.DefaultEase (2.5, no change)
	reviewAndCheck(t, "2. Learning->IKnow (Graduate)", cardID, db.RatingGood,
		db.StateReview, daysToDuration(db.GraduateToReviewIntervalDays), // 1 day
		currentEase, // 2.5
		2, 0)

	// 3. Review (1 day) -> "I Know"
	// Action: Rate db.RatingGood (2)
	// Result: Interval = 1day * Ease(2.5) = 2.5 days. Fuzz applies.
	// Ease: db.DefaultEase (2.5, no change)
	expectedIntervalStep3 := daysToDuration(1.0 * currentEase) // 2.5 days
	reviewAndCheck(t, "3. Review(1d)->IKnow", cardID, db.RatingGood,
		db.StateReview, expectedIntervalStep3,
		currentEase, // 2.5
		3, 0)

	// 4. Review (2.5 days) -> "I Know"
	// Action: Rate db.RatingGood (2)
	// Result: Interval = 2.5days * Ease(2.5) = 6.25 days. Fuzz applies.
	// Ease: db.DefaultEase (2.5, no change)
	expectedIntervalStep4 := daysToDuration(2.5 * currentEase) // 6.25 days
	reviewAndCheck(t, "4. Review(2.5d)->IKnow", cardID, db.RatingGood,
		db.StateReview, expectedIntervalStep4,
		currentEase, // 2.5
		4, 0)

	// 5. Review (6.25 days) -> "I Don't Know" (Lapse)
	// Action: Rate db.RatingAgain (1)
	// Result: Relearning, Interval = LearningStep1Duration (e.g. 1m), Ease penalized.
	// Ease: currentEase (2.5) + db.EaseAdjIDontKnow (-0.20) = 2.3
	currentEase = math.Max(db.MinEase, currentEase+db.EaseAdjIDontKnow) // 2.3
	reviewAndCheck(t, "5. Review(6.25d)->IDontKnow (Lapse)", cardID, db.RatingAgain,
		db.StateRelearning, db.LearningStep1Duration, // 1m
		currentEase, // 2.3
		5, 1)        // LapsCount becomes 1

	// 6. Relearning (Step 0, e.g. 1m) -> "I Know"
	// Action: Rate db.RatingGood (2)
	// Result: Stays Relearning, advances to LearningStep 1, Interval = LearningStep2Duration (e.g. 10m)
	// Ease: 2.3 (no ease change on relearn step advance with "IKnow")
	reviewAndCheck(t, "6. Relearning(1m)->IKnow", cardID, db.RatingGood,
		db.StateRelearning, db.LearningStep2Duration, // 10m
		currentEase, // 2.3
		6, 1)

	// 7. Relearning (Step 1, e.g. 10m) -> "I Know" (Graduates from Relearning)
	// Action: Rate db.RatingGood (2)
	// Result: Graduates to Review, Interval = GraduateToReviewIntervalDays (e.g. 1 day)
	// Ease: 2.3 (ease not changed on graduation from relearning by "IKnow")
	reviewAndCheck(t, "7. Relearning(10m)->IKnow (Graduate)", cardID, db.RatingGood,
		db.StateReview, daysToDuration(db.GraduateToReviewIntervalDays), // 1 day
		currentEase, // 2.3
		7, 1)

	// 8. Review (1 day, after relearning, Ease 2.3) -> "I Know"
	// Action: Rate db.RatingGood (2)
	// Result: Interval = 1day * Ease(2.3) = 2.3 days. Fuzz applies.
	// Ease: 2.3 (no change)
	expectedIntervalStep8 := daysToDuration(1.0 * currentEase) // 2.3 days
	reviewAndCheck(t, "8. Review(1d post-relearn)->IKnow", cardID, db.RatingGood,
		db.StateReview, expectedIntervalStep8,
		currentEase, // 2.3
		8, 1)

	// --- Scenarios with a different card (cardID2) ---
	if len(cards) > 1 {
		cardID2 := cards[1].ID
		t.Run("NewCardVariations_TwoButton", func(t *testing.T) {
			// Initial state for cardID2: New, Ease: db.DefaultEase (e.g., 2.5)
			card2Ease := db.DefaultEase

			// Scenario 1: New Card -> "I Don't Know"
			// Result: Learning, Step 0, Interval = LearningStep1Duration, Ease penalized
			card2Ease = math.Max(db.MinEase, card2Ease+db.EaseAdjIDontKnow) // 2.5 - 0.2 = 2.3
			reviewAndCheck(t, "Card2: New->IDontKnow", cardID2, db.RatingAgain,
				db.StateLearning, db.LearningStep1Duration, // 1m
				card2Ease, // 2.3
				1, 0)      // RC=1 for cardID2

			// Scenario 2: Learning (Step 0, 1m, Ease 2.3) -> "I Know"
			// Result: Learning, Step 1, Interval = LearningStep2Duration, Ease unchanged
			reviewAndCheck(t, "Card2: Learning(1m)->IKnow", cardID2, db.RatingGood,
				db.StateLearning, db.LearningStep2Duration, // 10m
				card2Ease, // 2.3 (no change)
				2, 0)      // RC=2 for cardID2

			// Scenario 3: Learning (Step 1, 10m, Ease 2.3) -> "I Know" (Graduates)
			// Result: Review, Interval = GraduateToReviewIntervalDays, Ease unchanged
			reviewAndCheck(t, "Card2: Learning(10m)->IKnow (Graduate)", cardID2, db.RatingGood,
				db.StateReview, daysToDuration(db.GraduateToReviewIntervalDays), // 1 day
				card2Ease, // 2.3 (no change)
				3, 0)      // RC=3 for cardID2

			// Scenario 4: Review (1 day, Ease 2.3) -> "I Don't Know" (Lapse)
			// Result: Relearning, Step 0, Interval = LearningStep1Duration, Ease penalized
			card2Ease = math.Max(db.MinEase, card2Ease+db.EaseAdjIDontKnow) // 2.3 - 0.2 = 2.1
			reviewAndCheck(t, "Card2: Review(1d)->IDontKnow (Lapse)", cardID2, db.RatingAgain,
				db.StateRelearning, db.LearningStep1Duration,
				card2Ease, // 2.1
				4, 1)      // RC=4, Laps=1 for cardID2
		})
	}

	// TODO: Add tests for:
	// - Max interval capping (ensure db.MaxReviewIntervalDays is respected)
	// - Ease hitting db.MinEase boundary after multiple "I Don't Know" reviews
	// - (No MaxEase to hit with current logic as ease only decreases or stays same)
}
