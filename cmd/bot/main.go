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