package main

import (
	"atamagaii/internal/db"
	"atamagaii/internal/handlers"
	"atamagaii/internal/middleware"
	"context"
	"fmt"
	"github.com/go-playground/validator/v10"
	telegram "github.com/go-telegram/bot"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"gopkg.in/yaml.v3"
	"log"
	"log/slog"
	"net/http"
	"os"
)

type Config struct {
	Host             string `yaml:"host"`
	Port             int    `yaml:"port"`
	DBPath           string `yaml:"db_path"`
	TelegramBotToken string `yaml:"telegram_bot_token"`
	OpenAIAPIKey     string `yaml:"openai_api_key"`
	GrokAPIKey       string `yaml:"grok_api_key"`
	ExternalURL      string `yaml:"external_url"`
	JWTSecretKey     string `yaml:"jwt_secret_key"`
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

	storage, err := db.ConnectDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	bot, err := telegram.New(cfg.TelegramBotToken)
	if err != nil {
		log.Fatal(err)
	}

	handler := handlers.NewHandler(bot, storage, cfg.JWTSecretKey, cfg.TelegramBotToken)

	log.Printf("Authorized on account %d", bot.ID())

	e := echo.New()

	logr := slog.New(slog.NewTextHandler(os.Stdout, nil))

	middleware.Setup(e, logr)

	webhookURL := fmt.Sprintf("%s/webhook", cfg.ExternalURL)
	if ok, err := bot.SetWebhook(context.Background(), &telegram.SetWebhookParams{
		DropPendingUpdates: true,
		URL:                webhookURL,
	}); err != nil {
		log.Fatalf("Failed to set webhook: %v", err)
	} else if !ok {
		log.Fatalf("Failed to set webhook: %v", err)
	}

	// Register all routes
	handler.RegisterRoutes(e)

	v1 := e.Group("/v1")
	authCfg := middleware.GetAuthConfig(cfg.JWTSecretKey)

	v1.Use(echojwt.WithConfig(authCfg))

	port := "8080"
	log.Printf("Starting server on port %s", port)
	if err := e.Start(":" + port); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
