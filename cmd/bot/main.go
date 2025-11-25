package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pinghoyk/neurobot/internal/bot"
	"github.com/pinghoyk/neurobot/internal/config"
	"github.com/pinghoyk/neurobot/internal/database"
	"github.com/pinghoyk/neurobot/internal/gigachat"
)

func main() {
	// Загрузка конфигурации из переменных окружения
	log.Println("Загрузка конфигурации...")
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Не удалось загрузить конфигурацию: %v", err)
	}
}