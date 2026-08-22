package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"clever-consumer/backend/internal/config"
	"clever-consumer/backend/internal/datasource/postgres"
	apphttp "clever-consumer/backend/internal/http"
	"clever-consumer/backend/internal/jobs"
	"clever-consumer/backend/internal/scraping"
	"clever-consumer/backend/internal/services"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	brightData := scraping.NewBrightDataCLI(cfg.BrightDataBinary, cfg.BrightDataTimeout, logger)
	app := services.NewApplication(store, brightData, logger, cfg)

	role := cfg.AppRole
	if role == "worker" || role == "all" {
		worker := jobs.NewWorker(app, logger)
		go worker.Run(ctx)
	}

	if role == "api" || role == "all" {
		server := apphttp.NewServer(cfg, app, logger)
		go func() {
			if err := server.ListenAndServe(); err != nil {
				logger.Error("http server stopped", "error", err)
				stop()
			}
		}()
	}

	<-ctx.Done()
	logger.Info("shutdown started")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = app.Shutdown(shutdownCtx)
	logger.Info("shutdown complete")
}
