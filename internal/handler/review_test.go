package handler_test

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db" // Assuming db constants are accessible
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

// Helper function to make tests more readable for duration calculations
func daysToDuration(days float64) time.Duration {
	return time.Duration(days * 24 * float64(time.Hour))
}

func TestReviewCard_SequentialFlow(t *testing.T) {
	e := testutils.SetupHandlerDependencies(t) // Sets up router and db store

	// 1. Authenticate and Import Deck
	resp, err := testutils.AuthHelper(t, e, testutils.TelegramTestUserID, "mkkksim", "Maksim")
	require.NoError(t, err, "Failed to authenticate")

	reqBody := map[string]string{
		"name":        "Test Review Deck",
		"description": "Deck for testing review functionality",
		"file_name":   "vocab_n5.json", // Ensure this file exists and has cards
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

	// reviewAndCheck helper function
	// It captures 'now' just before the review to make 'NextReview' assertions more reliable.
	reviewAndCheck := func(
		t *testing.T,
		stepName string, // For better test failure messages
		currentCardID string,
		rating int,
		expectedState db.CardState,
		expectedBaseIntervalNoFuzz time.Duration, // The interval *before* any fuzzing
		expectedEase float64,
		expectedReviewCount int,
		expectedLapsCount int,
	) {
		t.Helper()
		reviewTime := time.Now() // Time closer to the actual review API call

		reviewData := map[string]interface{}{
			"card_id":       currentCardID, // Use the passed cardID
			"rating":        rating,
			"time_spent_ms": 3000,
		}
		reviewBody, _ := json.Marshal(reviewData)

		// Perform the review request
		endpoint := "/v1/cards/" + currentCardID + "/review"
		rec := testutils.PerformRequest(t, e, http.MethodPost, endpoint, string(reviewBody), resp.Token, http.StatusOK)
		progress := testutils.ParseResponse[db.CardProgress](t, rec)

		// Assertions
		assert.Equal(t, expectedState, progress.State, "[%s] Unexpected state", stepName)
		assert.InDelta(t, expectedEase, progress.Ease, 0.001, "[%s] Unexpected ease", stepName) // Use 0.001 for float comparison
		assert.Equal(t, expectedReviewCount, progress.ReviewCount, "[%s] Unexpected review count", stepName)
		assert.Equal(t, expectedLapsCount, progress.LapsCount, "[%s] Unexpected laps count", stepName)

		// Interval Assertion (accounting for fuzz)
		if expectedBaseIntervalNoFuzz > 24*time.Hour && db.FuzzPercentage > 0.0 {
			// Allowable deviation due to fuzz: base_interval * fuzz_percentage
			// We add a small epsilon (e.g., 1 second) for floating point comparisons.
			fuzzDeltaSeconds := expectedBaseIntervalNoFuzz.Seconds()*db.FuzzPercentage + 1.0
			assert.InDelta(t, expectedBaseIntervalNoFuzz.Seconds(), progress.Interval.Seconds(), fuzzDeltaSeconds,
				"[%s] Unexpected interval (expected %v, got %v, considering fuzz)", stepName, expectedBaseIntervalNoFuzz, progress.Interval)
		} else {
			assert.Equal(t, expectedBaseIntervalNoFuzz, progress.Interval, "[%s] Unexpected interval (no fuzz expected, expected %v, got %v)", stepName, expectedBaseIntervalNoFuzz, progress.Interval)
		}

		// NextReview Assertion
		require.NotNil(t, progress.NextReview, "[%s] Expected next review date to be set", stepName)

		// Expected next review time, calculated from the *actual progress.Interval* returned by the server.
		// This ensures the server's `now.Add(progress.Interval)` logic is consistent.
		// There will be a small diff due to request latency and server processing time.
		expectedNextReviewBasedOnActualInterval := reviewTime.Add(progress.Interval)
		assert.WithinDuration(t, expectedNextReviewBasedOnActualInterval, *progress.NextReview, 15*time.Second, // Allow some seconds for processing drift
			"[%s] NextReview (%v) not close to reviewTime + actual progress.Interval (%v)", stepName, *progress.NextReview, expectedNextReviewBasedOnActualInterval)

		// Additionally, check if the NextReview time is roughly what we expect based on the *unfuzzed* base interval.
		// This helps catch gross errors if fuzz calculation itself is way off.
		expectedNextReviewBasedOnUnfuzzed := reviewTime.Add(expectedBaseIntervalNoFuzz)
		maxDeviation := time.Minute // Default for short intervals
		if expectedBaseIntervalNoFuzz > 24*time.Hour && db.FuzzPercentage > 0.0 {
			maxDeviation = time.Duration(expectedBaseIntervalNoFuzz.Seconds()*db.FuzzPercentage*1.2) + time.Minute // 20% margin on fuzz + 1 min
		}
		assert.WithinDuration(t, expectedNextReviewBasedOnUnfuzzed, *progress.NextReview, maxDeviation,
			"[%s] NextReview (%v) too far from expected (unfuzzed) %v", stepName, *progress.NextReview, expectedNextReviewBasedOnUnfuzzed)
	}

	// --- Test Case Sequence using cardID (cards[0].ID) ---
	// Values based on the new algorithm (db.DefaultEase, db.EaseAdj*, etc.)

	// Initial state: New card, Ease: db.DefaultEase (2.5), ReviewCount: 0, LapsCount: 0

	// 1. New Card -> Good
	// Action: Rate 3 (Good)
	// Result: Learning, Step 1 advanced, Interval = LearningStep2 (10m)
	reviewAndCheck(t, "1. New->Good", cardID, 3,
		db.StateLearning, db.LearningStep2Duration, // 10m
		db.DefaultEase, // 2.5 (no change for Good: 2.5 + 0.0)
		1, 0)

	// 2. Learning (10m) -> Good
	// Action: Rate 3 (Good)
	// Result: Graduate to Review, Interval = 1 day
	// Current: StateLearning, Ease=2.5, RC=1, LC=0
	reviewAndCheck(t, "2. Learning->Good", cardID, 3,
		db.StateReview, daysToDuration(db.GraduateToReviewIntervalGoodDays), // 1 day (24h)
		db.DefaultEase, // 2.5 (no change)
		2, 0)

	// 3. Review (1 day) -> Good
	// Action: Rate 3 (Good)
	// Result: Interval = 1day * Ease(2.5) = 2.5 days. Fuzz applies.
	// Current: StateReview, Interval=1d, Ease=2.5, RC=2, LC=0
	reviewAndCheck(t, "3. Review(1d)->Good", cardID, 3,
		db.StateReview, daysToDuration(1.0*db.DefaultEase), // 2.5 days (60h)
		db.DefaultEase, // 2.5 (no change)
		3, 0)

	// 4. Review (2.5 days) -> Good
	// Action: Rate 3 (Good)
	// Result: Interval = 2.5days * Ease(2.5) = 6.25 days. Fuzz applies.
	// Current: StateReview, Interval=2.5d, Ease=2.5, RC=3, LC=0
	reviewAndCheck(t, "4. Review(2.5d)->Good", cardID, 3,
		db.StateReview, daysToDuration(2.5*db.DefaultEase), // 6.25 days (150h)
		db.DefaultEase, // 2.5 (no change)
		4, 0)

	// 5. Review (6.25 days) -> Again (Lapse)
	// Action: Rate 1 (Again)
	// Result: Relearning, Interval = LearningStep1 (1m), Ease penalized.
	// Current: StateReview, Interval=6.25d, Ease=2.5, RC=4, LC=0
	expectedEaseAfterLapse := math.Max(db.MinEase, db.DefaultEase+db.EaseAdjAgain) // 2.5 - 0.20 = 2.3
	reviewAndCheck(t, "5. Review(6.25d)->Again", cardID, 1,
		db.StateRelearning, db.LearningStep1Duration, // 1m
		expectedEaseAfterLapse, // 2.3
		5, 1)                   // LapsCount becomes 1

	// 6. Relearning (1m, LS=0) -> Good
	// Action: Rate 3 (Good)
	// Result: Stays Relearning, advances to LearningStep2 (10m)
	// Current: StateRelearning, Interval=1m, Ease=2.3, RC=5, LC=1
	reviewAndCheck(t, "6. Relearning(1m)->Good", cardID, 3,
		db.StateRelearning, db.LearningStep2Duration, // 10m
		expectedEaseAfterLapse, // 2.3 (no ease change on relearn step advance)
		6, 1)

	// 7. Relearning (10m, LS=1) -> Good (Graduates from Relearning)
	// Action: Rate 3 (Good)
	// Result: Graduates to Review, Interval = 1 day
	// Current: StateRelearning, Interval=10m, Ease=2.3, RC=6, LC=1
	reviewAndCheck(t, "7. Relearning(10m)->Good", cardID, 3,
		db.StateReview, daysToDuration(db.GraduateToReviewIntervalGoodDays), // 1 day (24h)
		expectedEaseAfterLapse, // 2.3 (ease not changed on graduation from relearning by 'Good')
		7, 1)

	// 8. Review (1 day) -> Easy
	// Action: Rate 4 (Easy)
	// Result: Interval = 1day * Ease(2.3) * EasyBonus(1.3). Ease increases. Fuzz applies.
	// Current: StateReview, Interval=1d, Ease=2.3, RC=7, LC=1
	newEase := math.Min(db.MaxEase, expectedEaseAfterLapse+db.EaseAdjEasy)                // 2.3 + 0.15 = 2.45
	expectedIntervalEasy := 1.0 * expectedEaseAfterLapse * db.EasyBonusIntervalMultiplier // 1.0 * 2.3 * 1.3 = 2.99 days
	reviewAndCheck(t, "8. Review(1d)->Easy", cardID, 4,
		db.StateReview, daysToDuration(expectedIntervalEasy), // 2.99 days (approx 71.76h)
		newEase, // 2.45
		8, 1)

	// --- Add more distinct scenarios using a different card if available ---
	if len(cards) > 1 {
		cardID2 := cards[1].ID
		t.Run("NewCardVariations", func(t *testing.T) {
			// Scenario: New Card -> Easy (Direct graduation)
			// Initial: New, Ease db.DefaultEase (2.5)
			easeAfterNewEasy := math.Min(db.MaxEase, db.DefaultEase+db.EaseAdjEasy) // 2.5 + 0.15 = 2.65, capped at MaxEase (2.5)
			reviewAndCheck(t, "NewCard->Easy", cardID2, 4,
				db.StateReview, daysToDuration(db.GraduateToReviewIntervalEasyDays), // 4 days
				easeAfterNewEasy, // Should be db.MaxEase if DefaultEase + AdjEasy > MaxEase
				1, 0)

			// Reset progress for cardID2 or use cards[2] if you have a reset API per card
			// For this test, we'll assume cardID2 is now in Review state from above.
			// Let's test a "Hard" on this card (currently 4d interval, ease 2.5)
			// Interval = 4d * HardMultiplier (1.2) = 4.8 days
			// Ease = 2.5 + EaseAdjHard (-0.15) = 2.35
			currentEase := easeAfterNewEasy
			expectedEaseAfterHard := math.Max(db.MinEase, currentEase+db.EaseAdjHard)
			expectedIntervalHard := 4.0 * db.HardIntervalMultiplier // 4.0 * 1.2 = 4.8 days
			reviewAndCheck(t, "Review(4d)->Hard", cardID2, 2,
				db.StateReview, daysToDuration(expectedIntervalHard),
				expectedEaseAfterHard,
				2, 0) // Review count for cardID2 is now 2
		})
	}

	// TODO: Add tests for:
	// - Max interval capping
	// - Ease hitting MinEase and MaxEase boundaries after multiple reviews
	// - New card -> Again/Hard
	// - Relearning -> Again/Hard/Easy variations
}
