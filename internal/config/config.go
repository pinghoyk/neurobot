package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken string
	GigaChatClientID string
	GigaChatSecret   string
	GigaChatScope    string
	DatabasePath     string
}

// Загружаем конфиг и ищем файл .env
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		TelegramBotToken: os.Getenv("TG_BOT_TOKEN"),
		GigaChatClientID: os.Getenv("GIGACHAT_CLIENT_ID"),
		GigaChatSecret:   os.Getenv("GIGACHAT_SECRET"),
		GigaChatScope:    os.Getenv("GIGACHAT_SCOPE"),
		DatabasePath:     getEnvOrDefault("DATABASE_PATH", "bot.db"),
	}

	if cfg.TelegramBotToken == "" {
		return nil, fmt.Errorf("Требуется - TG_BOT_TOKEN")
	}

	if cfg.GigaChatClientID == "" {
		return nil, fmt.Errorf("Требуется - GIGACHAT_CLIENT_ID")
	}

	if cfg.GigaChatSecret == "" {
		return nil, fmt.Errorf("Требуется - GIGACHAT_SECRET")
	}

	if cfg.GigaChatScope == "" {
		cfg.GigaChatScope = "GIGACHAT_API_CORP"
	}

	return cfg, nil
}