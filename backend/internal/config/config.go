package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppRole           string
	HTTPAddr          string
	DatabaseURL       string
	PublicBaseURL     string
	SessionCookieName string
	BrightDataBinary  string
	BrightDataTimeout time.Duration
	SMTPFrom          string
}

func Load() (Config, error) {
	cfg := Config{
		AppRole:           env("APP_ROLE", "all"),
		HTTPAddr:          env("HTTP_ADDR", ":18080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		PublicBaseURL:     env("PUBLIC_BASE_URL", "http://localhost:5173"),
		SessionCookieName: env("SESSION_COOKIE_NAME", "cc_session"),
		BrightDataBinary:  env("BRIGHTDATA_BIN", "brightdata"),
		BrightDataTimeout: secondsEnv("BRIGHTDATA_TIMEOUT_SECONDS", 90),
		SMTPFrom:          env("SMTP_FROM", "alerts@clever-consumer.local"),
	}
	if cfg.AppRole != "all" && cfg.AppRole != "api" && cfg.AppRole != "worker" {
		return Config{}, errors.New("APP_ROLE must be all, api, or worker")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func secondsEnv(key string, fallback int) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return time.Duration(fallback) * time.Second
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return time.Duration(fallback) * time.Second
	}
	return time.Duration(value) * time.Second
}
