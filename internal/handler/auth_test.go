package handler_test

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"atamagaii/internal/contract"
	"atamagaii/internal/handler"
	"atamagaii/internal/testutils"
)

func TestMain(m *testing.M) {
	testutils.InitTestDB()
	code := m.Run()
	testutils.CleanupTestDB()
	os.Exit(code)
}

func TestTelegramAuth_Success(t *testing.T) {
	e := testutils.SetupHandlerDependencies(t)

	resp, err := testutils.AuthHelper(t, e, testutils.TelegramTestUserID, "mkkksim", "Maksim")
	if err != nil {
		t.Fatalf("Failed to authenticate: %v", err)
	}

	if resp.Token == "" {
		t.Error("Expected non-empty JWT token")
	}

	if resp.User.TelegramID != testutils.TelegramTestUserID {
		t.Errorf("Expected ChatID %d, got %d", testutils.TelegramTestUserID, resp.User.TelegramID)
	}

	if resp.User.Username == nil || *resp.User.Username != "mkkksim" {
		t.Errorf("Expected username 'mkkksim', got '%v'", resp.User.Username)
	}

	if resp.User.Name == nil || *resp.User.Name != "Maksim" {
		t.Errorf("Expected Name 'Maksim', got '%v'", resp.User.Name)
	}
}

func TestTelegramAuth_InvalidInitData(t *testing.T) {
	e := testutils.SetupHandlerDependencies(t)

	reqBody := contract.AuthTelegramRequest{
		Query: "invalid-init-data",
	}
	body, _ := json.Marshal(reqBody)

	rec := testutils.PerformRequest(t, e, http.MethodPost, "/auth/telegram", string(body), "", http.StatusUnauthorized)

	resp := testutils.ParseResponse[contract.ErrorResponse](t, rec)
	if resp.Error != handler.ErrInvalidInitData {
		t.Errorf("Expected error '%s', got '%s'", handler.ErrInvalidInitData, resp.Error)
	}
}

func TestTelegramAuth_MissingQuery(t *testing.T) {
	e := testutils.SetupHandlerDependencies(t)

	reqBody := contract.AuthTelegramRequest{}
	body, _ := json.Marshal(reqBody)

	rec := testutils.PerformRequest(t, e, http.MethodPost, "/auth/telegram", string(body), "", http.StatusBadRequest)

	resp := testutils.ParseResponse[contract.ErrorResponse](t, rec)
	expectedError := "query cannot be empty"
	if resp.Error != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, resp.Error)
	}
}
