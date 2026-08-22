package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"clever-consumer/backend/internal/config"
	"clever-consumer/backend/internal/datasource/postgres"
	apphttp "clever-consumer/backend/internal/http"
	"clever-consumer/backend/internal/jobs"
	"clever-consumer/backend/internal/logging"
	"clever-consumer/backend/internal/scraping"
	"clever-consumer/backend/internal/services"
	"go.uber.org/zap"
)

func main() {
	logger, syncLogger, err := logging.New()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "create logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = syncLogger()
	}()

	cfg, err := config.Load()
	if err != nil {
		logger.Errorw("load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := openPostgresWithRetry(ctx, cfg.DatabaseURL, logger)
	if err != nil {
		logger.Errorw("connect postgres", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	brightData := scraping.NewBrightDataAPI(
		cfg.BrightDataAPIKey,
		cfg.BrightDataZone,
		cfg.BrightDataAPIBaseURL,
		cfg.BrightDataTimeout,
		cfg.BrightDataHealRetries,
		cfg.BrightDataDatasetSeeds,
		logger,
	)
	scraper := scraping.NewManager(store, brightData, scraping.CollectorThresholds{
		RequestThreshold: cfg.ScraperCollectorRequestThreshold,
		ProductThreshold: cfg.ScraperCollectorProductThreshold,
		MaxPerDomain:     cfg.ScraperCollectorMaxPerDomain,
	}, logger)
	app := services.NewApplication(store, scraper, logger, cfg)

	role := cfg.AppRole
	if role == "worker" || role == "all" {
		worker := jobs.NewWorker(app, logger)
		go worker.Run(ctx)
	}

	if role == "api" || role == "all" {
		server := apphttp.NewServer(cfg, app, logger)
		go func() {
			if err := server.ListenAndServe(); err != nil {
				logger.Errorw("http server stopped", "error", err)
				stop()
			}
		}()
	}

	<-ctx.Done()
	logger.Infow("shutdown started")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = app.Shutdown(shutdownCtx)
	logger.Infow("shutdown complete")
}

func openPostgresWithRetry(ctx context.Context, databaseURL string, logger *zap.SugaredLogger) (*postgres.Store, error) {
	var lastErr error
	for attempt := 1; attempt <= 10; attempt++ {
		store, err := postgres.Open(ctx, databaseURL)
		if err == nil {
			return store, nil
		}
		lastErr = err
		logger.Warnw("postgres connection failed; retrying", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}
	return nil, lastErr
}
