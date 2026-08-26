package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"observability-platform/internal/bootstrap"
	"observability-platform/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		log.Printf("configuration error: %v", err)
		return
	}
	app, err := bootstrap.New(cfg)
	if err != nil {
		log.Printf("bootstrap error: %v", err)
		return
	}
	if err := app.Run(ctx); err != nil {
		log.Printf("application stopped with error: %v", err)
	}
}
