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

	// Создание базы данных
	log.Println("Создание базы данных...")
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Не удалось создать базу данных: %v", err)
	}
	defer db.Close() // Закрыть соединение с БД при завершенииы

	// Создание клиента GigaChat
	log.Println("Создание клиента GigaChat...")
	gigachatClient := gigachat.NewClient(
		cfg.GigaChatClientID,
		cfg.GigaChatSecret,
		cfg.GigaChatScope,
	)

	// Создание бота
	log.Println("Создание бота...")
	telegramBot, err := bot.New(cfg.TelegramBotToken, db, gigachatClient)
	if err != nil {
		log.Fatalf("Не удалось создать бота: %v", err)
	}

	// Настройка корректного завершения работы
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Обработка системных сигналов (например, Ctrl+C) для безопасного завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Получен сигнал: %v. Выполняется безопасное завершение...", sig)
		cancel() // Отмена контекста, чтобы остановить бота
	}()
}