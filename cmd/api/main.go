package main

import (
	"atamagaii/internal/ai"
	"atamagaii/internal/db"
	"atamagaii/internal/handler"
	"atamagaii/internal/job"
	"atamagaii/internal/middleware"
	"atamagaii/internal/storage"
	"context"
	"fmt"
	"github.com/go-playground/validator/v10"
	telegram "github.com/go-telegram/bot"
	"github.com/labstack/echo/v4"
	"gopkg.in/yaml.v3"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Config struct {
	Host             string           `yaml:"host"`
	Port             int              `yaml:"port"`
	DBPath           string           `yaml:"db_path"`
	TelegramBotToken string           `yaml:"telegram_bot_token"`
	OpenAIAPIKey     string           `yaml:"openai_api_key"`
	GrokAPIKey       string           `yaml:"grok_api_key"`
	ExternalURL      string           `yaml:"external_url"`
	JWTSecretKey     string           `yaml:"jwt_secret_key"`
	S3Storage        storage.S3Config `yaml:"s3_storage"`
	TelegramWebApp   string           `yaml:"telegram_webapp_url"`
}

func ReadConfig(filePath string) (*Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var cfg Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	return &cfg, nil
}

func ValidateConfig(cfg *Config) error {
	validate := validator.New()
	return validate.Struct(cfg)
}

type CustomValidator struct {
	validator *validator.Validate
}

func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

func main() {
	configFilePath := "config.yml"
	configFilePathEnv := os.Getenv("CONFIG_FILE_PATH")
	if configFilePathEnv != "" {
		configFilePath = configFilePathEnv
	}

	cfg, err := ReadConfig(configFilePath)
	if err != nil {
		log.Fatalf("error reading configuration: %v", err)
	}

	if err := ValidateConfig(cfg); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	dbStorage, err := db.ConnectDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	bot, err := telegram.New(cfg.TelegramBotToken)
	if err != nil {
		log.Fatal(err)
	}

	var storageProvider storage.Provider
	storageProvider, err = storage.NewS3Provider(cfg.S3Storage)
	if err != nil {
		log.Printf("Warning: Failed to initialize S3 storage: %v", err)
		storageProvider = nil
	}

	openaiClient, err := ai.NewOpenAIClient(cfg.OpenAIAPIKey)
	if err != nil {
		log.Fatalf("Failed to create OpenAI client: %v", err)
	}

	h := handler.New(bot, dbStorage, cfg.JWTSecretKey, cfg.TelegramBotToken, cfg.TelegramWebApp, storageProvider, openaiClient)

	log.Printf("Authorized on account %d", bot.ID())

	e := echo.New()

	logr := slog.New(slog.NewTextHandler(os.Stdout, nil))

	middleware.Setup(e, logr)

	e.Validator = &CustomValidator{validator: validator.New()}

	webhookURL := fmt.Sprintf("%s/webhook", cfg.ExternalURL)
	if ok, err := bot.SetWebhook(context.Background(), &telegram.SetWebhookParams{
		DropPendingUpdates: true,
		URL:                webhookURL,
	}); err != nil {
		log.Fatalf("Failed to set webhook: %v", err)
	} else if !ok {
		log.Fatalf("Failed to set webhook: %v", err)
	}

	// Start task generation job
	taskGenerator := job.NewTaskGenerator(dbStorage, openaiClient)
	go taskGenerator.Start()
	log.Println("Task generation job started")

	h.RegisterRoutes(e)

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		port := "8080"
		log.Printf("Starting server on port %s", port)
		if err := e.Start(":" + port); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Wait for shutdown signal
	<-quit
	log.Println("Shutting down server...")

	// Stop the task generator
	taskGenerator.Stop()

	// Shutdown Echo server with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}

	log.Println("Server gracefully stopped")
}
