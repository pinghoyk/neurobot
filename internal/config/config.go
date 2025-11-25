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

