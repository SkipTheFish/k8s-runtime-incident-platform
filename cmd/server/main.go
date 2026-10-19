package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"observability-platform/internal/bootstrap"
	"observability-platform/internal/config"
	"observability-platform/internal/telemetry"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		log.Printf("configuration error: %v", err)
		return
	}
	shutdownTelemetry, err := telemetry.Configure(ctx, cfg.OTLPEndpoint, cfg.OTELServiceName)
	if err != nil {
		log.Printf("telemetry configuration error: %v", err)
		return
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			log.Printf("telemetry shutdown error: %v", err)
		}
	}()
	app, err := bootstrap.New(cfg)
	if err != nil {
		log.Printf("bootstrap error: %v", err)
		return
	}
	if err := app.Run(ctx); err != nil {
		log.Printf("application stopped with error: %v", err)
	}
}
