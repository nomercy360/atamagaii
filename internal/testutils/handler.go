package testutils

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db"
	"atamagaii/internal/handler"
	"atamagaii/internal/middleware"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	telegram "github.com/go-telegram/bot"
	"github.com/labstack/echo/v4"
	initdata "github.com/telegram-mini-apps/init-data-golang"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	dbStorage *db.Storage
)

// CustomValidator implements the echo.Validator interface
type CustomValidator struct {
	validator *validator.Validate
}

// Validate validates the provided struct
func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

const (
	TestBotToken       = "test-bot-token"
	TelegramTestUserID = 927635965
	TestDBPath         = ":memory:" // Use in-memory SQLite for tests
)

func InitTestDB() {
	ctx := context.Background()
	var err error
	dbStorage, _, err = setupTestDB(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize test database: %v", err))
	}
}

func CleanupTestDB() {
	if dbStorage != nil {
		if err := dbStorage.Close(); err != nil {
			fmt.Printf("Warning: Failed to close test database: %v\n", err)
		}
		dbStorage = nil
	}
}

func setupTestDB(ctx context.Context) (*db.Storage, func(), error) {
	storage, err := db.ConnectDB(TestDBPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to test database: %w", err)
	}

	cleanup := func() {
		if err := storage.Close(); err != nil {
			fmt.Printf("Warning: Failed to close test database: %v\n", err)
		}
	}

	return storage, cleanup, nil
}

func SetupHandlerDependencies(t *testing.T) *echo.Echo {
	var bot *telegram.Bot

	// Initialize DB for tests
	ctx := context.Background()
	var err error
	if dbStorage == nil {
		dbStorage, _, err = setupTestDB(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize test database: %v", err)
		}

		// Initialize schema
		if err := dbStorage.UpdateSchema(); err != nil {
			t.Fatalf("Failed to update schema: %v", err)
		}
	}

	h := handler.New(bot, dbStorage, "hello-world", TestBotToken)

	e := echo.New()

	logr := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	middleware.Setup(e, logr)

	// Add validator to Echo
	e.Validator = &CustomValidator{validator: validator.New()}

	h.RegisterRoutes(e)

	return e
}

func PerformRequest(t *testing.T, e *echo.Echo, method, path, body, token string, expectedStatus int) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != expectedStatus {
		t.Errorf("Expected status %d, got %d, body: %s", expectedStatus, rec.Code, rec.Body.String())
	}
	return rec
}

func ParseResponse[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	var result T
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	return result
}

func AuthHelper(t *testing.T, e *echo.Echo, telegramID int64, username, firstName string) (contract.AuthTelegramResponse, error) {
	userJSON := fmt.Sprintf(
		`{"id":%d,"first_name":"%s","last_name":"","username":"%s","language_code":"ru","is_premium":true,"allows_write_to_pm":true,"photo_url":"https://t.me/i/userpic/320/test.svg"}`,
		telegramID, firstName, username,
	)

	initData := map[string]string{
		"query_id":  "AAH9mUo3AAAAAP2ZSjdVL00J",
		"user":      userJSON,
		"auth_date": fmt.Sprintf("%d", time.Now().Unix()),
		"signature": "W_7-jDZLl7iwW8Qr2IZARpIsseV6jJDU_6eQ3ti-XY5Nm58N1_9dkXuFf9xidDZ0aoY_Pv0kq2-clrbHeLMQBA",
	}

	sign := initdata.Sign(initData, TestBotToken, time.Now())
	initData["hash"] = sign

	var query string
	for k, v := range initData {
		query += fmt.Sprintf("%s=%s&", k, v)
	}

	reqBody := contract.AuthTelegramRequest{
		Query: query,
	}

	body, _ := json.Marshal(reqBody)

	rec := PerformRequest(t, e, http.MethodPost, "/auth/telegram", string(body), "", http.StatusOK)

	resp := ParseResponse[contract.AuthTelegramResponse](t, rec)

	return resp, nil
}
