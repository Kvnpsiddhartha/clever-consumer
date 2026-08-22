package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppRole                          string
	HTTPAddr                         string
	DatabaseURL                      string
	PublicBaseURL                    string
	SessionCookieName                string
	GoogleClientID                   string
	GoogleClientSecret               string
	GoogleRedirectURL                string
	BrightDataAPIBaseURL             string
	BrightDataAPIKey                 string
	BrightDataZone                   string
	BrightDataTimeout                time.Duration
	BrightDataHealRetries            int
	BrightDataDatasetSeeds           map[string]string
	ScraperCollectorRequestThreshold int
	ScraperCollectorProductThreshold int
	ScraperCollectorMaxPerDomain     int
	SMTPFrom                         string
}

func Load() (Config, error) {
	cfg := Config{
		AppRole:                          env("APP_ROLE", "all"),
		HTTPAddr:                         env("HTTP_ADDR", ":18080"),
		DatabaseURL:                      os.Getenv("DATABASE_URL"),
		PublicBaseURL:                    env("PUBLIC_BASE_URL", "http://localhost:5173"),
		SessionCookieName:                env("SESSION_COOKIE_NAME", "cc_session"),
		GoogleClientID:                   os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:               os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:                env("GOOGLE_REDIRECT_URL", "http://localhost:18080/v1/auth/google/callback"),
		BrightDataAPIBaseURL:             env("BRIGHTDATA_API_BASE_URL", "https://api.brightdata.com"),
		BrightDataAPIKey:                 os.Getenv("BRIGHTDATA_API_KEY"),
		BrightDataZone:                   os.Getenv("BRIGHTDATA_UNLOCKER_ZONE"),
		BrightDataTimeout:                secondsEnv("BRIGHTDATA_TIMEOUT_SECONDS", 1800),
		BrightDataHealRetries:            intEnv("BRIGHTDATA_HEAL_RETRIES", 1),
		BrightDataDatasetSeeds:           domainMapEnv("BRIGHTDATA_DATASET_SEEDS"),
		ScraperCollectorRequestThreshold: intEnv("SCRAPER_COLLECTOR_REQUEST_THRESHOLD", 500),
		ScraperCollectorProductThreshold: intEnv("SCRAPER_COLLECTOR_PRODUCT_THRESHOLD", 100),
		ScraperCollectorMaxPerDomain:     intEnv("SCRAPER_COLLECTOR_MAX_PER_DOMAIN", 3),
		SMTPFrom:                         env("SMTP_FROM", "alerts@clever-consumer.local"),
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

func intEnv(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func domainMapEnv(key string) map[string]string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	values := make(map[string]string)
	for _, entry := range strings.Split(raw, ",") {
		parts := strings.SplitN(strings.TrimSpace(entry), ":", 2)
		if len(parts) != 2 {
			continue
		}
		domain := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(parts[0])), "www.")
		id := strings.TrimSpace(parts[1])
		if domain != "" && id != "" {
			values[domain] = id
		}
	}
	return values
}
