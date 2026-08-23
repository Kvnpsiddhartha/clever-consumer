package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"clever-consumer/backend/internal/config"
	"clever-consumer/backend/internal/domain"
	"go.uber.org/zap"
)

type Store interface {
	Close()
	CreateMagicLink(ctx context.Context, id, email, tokenHash string, expiresAt time.Time) error
	ConsumeMagicLink(ctx context.Context, tokenHash string, now time.Time) (string, error)
	UpsertUser(ctx context.Context, email, country, timezone string) (domain.User, error)
	CreateSession(ctx context.Context, id, userID string, expiresAt time.Time) error
	UserBySession(ctx context.Context, sessionID string, now time.Time) (domain.User, error)
	UpdateProfile(ctx context.Context, userID, country, timezone string) (domain.User, error)
	CreatePreview(ctx context.Context, preview domain.ProductPreview) error
	Preview(ctx context.Context, userID, previewID string) (domain.ProductPreview, error)
	ConfirmPreview(ctx context.Context, userID, previewID string, at time.Time) (domain.ProductPreview, error)
	CreateTracker(ctx context.Context, tracker domain.Tracker) error
	ListTrackers(ctx context.Context, userID string) ([]domain.Tracker, error)
	Tracker(ctx context.Context, userID, trackerID string) (domain.Tracker, error)
	DeleteTracker(ctx context.Context, userID, trackerID string) error
	AddObservation(ctx context.Context, obs domain.Observation) error
	Observations(ctx context.Context, userID, trackerID string) ([]domain.Observation, error)
	SetTrackerStatus(ctx context.Context, userID, trackerID, status string) error
	TouchTrackerRun(ctx context.Context, userID, trackerID string, price float64, availability string, observedAt, nextCheckAt time.Time) error
	DueTrackers(ctx context.Context, now time.Time, limit int) ([]domain.Tracker, error)
	ListCollectorOperations(ctx context.Context) ([]domain.CollectorOperationsProfile, error)
	CollectorHealingTarget(ctx context.Context, collectorID string) (domain.CollectorHealingTarget, error)
}

type Scraper interface {
	Scrape(ctx context.Context, targetURL, country string) (domain.ScrapeResult, error)
}

type CollectorHealer interface {
	HealCollector(ctx context.Context, collector domain.ScraperCollector, targetURL, country, reason string) error
}

type Application struct {
	store   Store
	scraper Scraper
	logger  *zap.SugaredLogger
	cfg     config.Config
}

const trackerRunTimeout = 2 * time.Minute
const previewScrapeTimeout = 2 * time.Minute

func NewApplication(store Store, scraper Scraper, logger *zap.SugaredLogger, cfg config.Config) *Application {
	return &Application{store: store, scraper: scraper, logger: logger, cfg: cfg}
}

func (a *Application) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		a.store.Close()
		close(done)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (a *Application) RequestMagicLink(ctx context.Context, email string) (domain.MagicLink, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") {
		return domain.MagicLink{}, errors.New("valid email is required")
	}
	token := randomToken()
	link := domain.MagicLink{
		ID:        newID("ml"),
		Email:     email,
		Token:     token,
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}
	if err := a.store.CreateMagicLink(ctx, link.ID, link.Email, hashToken(token), link.ExpiresAt); err != nil {
		return domain.MagicLink{}, err
	}
	return link, nil
}

func (a *Application) VerifyMagicLink(ctx context.Context, token string) (domain.User, string, time.Time, error) {
	email, err := a.store.ConsumeMagicLink(ctx, hashToken(strings.TrimSpace(token)), time.Now().UTC())
	if err != nil {
		return domain.User{}, "", time.Time{}, err
	}
	return a.CreateEmailSession(ctx, email)
}

func (a *Application) CreateEmailSession(ctx context.Context, email string) (domain.User, string, time.Time, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") {
		return domain.User{}, "", time.Time{}, errors.New("valid email is required")
	}
	user, err := a.store.UpsertUser(ctx, email, "IN", "Asia/Kolkata")
	if err != nil {
		return domain.User{}, "", time.Time{}, err
	}
	sessionID := newID("sess")
	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)
	if err := a.store.CreateSession(ctx, sessionID, user.ID, expiresAt); err != nil {
		return domain.User{}, "", time.Time{}, err
	}
	return user, sessionID, expiresAt, nil
}

func (a *Application) UserBySession(ctx context.Context, sessionID string) (domain.User, error) {
	return a.store.UserBySession(ctx, sessionID, time.Now().UTC())
}

func (a *Application) UpdateProfile(ctx context.Context, userID, country, timezone string) (domain.User, error) {
	country = strings.ToUpper(strings.TrimSpace(country))
	timezone = strings.TrimSpace(timezone)
	if len(country) != 2 {
		return domain.User{}, errors.New("country must be a two-letter ISO code")
	}
	if timezone == "" {
		return domain.User{}, errors.New("timezone is required")
	}
	return a.store.UpdateProfile(ctx, userID, country, timezone)
}

func (a *Application) CreatePreview(ctx context.Context, user domain.User, rawURL, country string) (domain.ProductPreview, error) {
	normalizedURL, err := normalizePublicURL(rawURL)
	if err != nil {
		return domain.ProductPreview{}, err
	}
	if country == "" {
		country = user.Country
	}
	scrapeCtx, cancel := context.WithTimeout(ctx, previewScrapeTimeout)
	defer cancel()
	result, err := a.scraper.Scrape(scrapeCtx, normalizedURL, strings.ToUpper(country))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return domain.ProductPreview{}, fmt.Errorf("preview scrape timed out after %s", previewScrapeTimeout)
		}
		return domain.ProductPreview{}, err
	}
	now := time.Now().UTC()
	preview := domain.ProductPreview{
		ID:           newID("prv"),
		UserID:       user.ID,
		URL:          normalizedURL,
		Status:       domain.PreviewNeedsConfirmation,
		Name:         result.Name,
		ImageURL:     result.ImageURL,
		CurrentPrice: result.CurrentPrice,
		Currency:     result.Currency,
		Availability: result.Availability,
		Country:      result.Country,
		Confidence:   result.Confidence,
		ExpiresAt:    now.Add(24 * time.Hour),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := a.store.CreatePreview(ctx, preview); err != nil {
		return domain.ProductPreview{}, err
	}
	return preview, nil
}

func (a *Application) Preview(ctx context.Context, userID, previewID string) (domain.ProductPreview, error) {
	return a.store.Preview(ctx, userID, previewID)
}

func (a *Application) CreateTracker(ctx context.Context, userID, previewID string, intervalHours int, country string, rules []domain.AlertRule) (domain.Tracker, error) {
	if intervalHours < 1 || intervalHours > 168 {
		return domain.Tracker{}, errors.New("interval_hours must be between 1 and 168")
	}
	if len(rules) == 0 {
		return domain.Tracker{}, errors.New("at least one alert rule is required")
	}
	preview, err := a.store.ConfirmPreview(ctx, userID, previewID, time.Now().UTC())
	if err != nil {
		return domain.Tracker{}, err
	}
	if country == "" {
		country = preview.Country
	}
	now := time.Now().UTC()
	tracker := domain.Tracker{
		ID:            newID("trk"),
		UserID:        userID,
		PreviewID:     previewID,
		ProductURL:    preview.URL,
		Name:          preview.Name,
		ImageURL:      preview.ImageURL,
		Currency:      preview.Currency,
		Country:       strings.ToUpper(country),
		IntervalHours: intervalHours,
		Status:        domain.TrackerActive,
		CurrentPrice:  preview.CurrentPrice,
		Availability:  preview.Availability,
		NextCheckAt:   now.Add(time.Duration(intervalHours) * time.Hour),
		Rules:         rulesWithIDs(rules),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := a.store.CreateTracker(ctx, tracker); err != nil {
		return domain.Tracker{}, err
	}
	_ = a.store.AddObservation(ctx, domain.Observation{
		ID:           newID("obs"),
		TrackerID:    tracker.ID,
		Price:        tracker.CurrentPrice,
		Currency:     tracker.Currency,
		Availability: tracker.Availability,
		Method:       "confirmed_preview",
		Confidence:   preview.Confidence,
		ObservedAt:   now,
	})
	return tracker, nil
}

func (a *Application) ListTrackers(ctx context.Context, userID string) ([]domain.Tracker, error) {
	return a.store.ListTrackers(ctx, userID)
}

func (a *Application) Tracker(ctx context.Context, userID, trackerID string) (domain.Tracker, error) {
	return a.store.Tracker(ctx, userID, trackerID)
}

func (a *Application) DeleteTracker(ctx context.Context, userID, trackerID string) error {
	return a.store.DeleteTracker(ctx, userID, trackerID)
}

func (a *Application) Observations(ctx context.Context, userID, trackerID string) ([]domain.Observation, error) {
	return a.store.Observations(ctx, userID, trackerID)
}

func (a *Application) SetTrackerStatus(ctx context.Context, userID, trackerID, status string) error {
	return a.store.SetTrackerStatus(ctx, userID, trackerID, status)
}

func (a *Application) RunTrackerNow(ctx context.Context, userID, trackerID string) (domain.Observation, error) {
	tracker, err := a.store.Tracker(ctx, userID, trackerID)
	if err != nil {
		return domain.Observation{}, err
	}
	scrapeCtx, cancel := context.WithTimeout(ctx, trackerRunTimeout)
	defer cancel()
	result, err := a.scraper.Scrape(scrapeCtx, tracker.ProductURL, tracker.Country)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return domain.Observation{}, fmt.Errorf("scraper run timed out after %s", trackerRunTimeout)
		}
		return domain.Observation{}, err
	}
	now := time.Now().UTC()
	obs := domain.Observation{
		ID:           newID("obs"),
		TrackerID:    tracker.ID,
		Price:        result.CurrentPrice,
		Currency:     result.Currency,
		Availability: result.Availability,
		Method:       result.Method,
		Confidence:   result.Confidence,
		ObservedAt:   now,
	}
	if err := a.store.AddObservation(ctx, obs); err != nil {
		return domain.Observation{}, err
	}
	next := now.Add(time.Duration(tracker.IntervalHours) * time.Hour)
	if err := a.store.TouchTrackerRun(ctx, userID, trackerID, obs.Price, obs.Availability, now, next); err != nil {
		return domain.Observation{}, err
	}
	return obs, nil
}

func (a *Application) ProcessDueTrackers(ctx context.Context) error {
	trackers, err := a.store.DueTrackers(ctx, time.Now().UTC(), 50)
	if err != nil {
		return err
	}
	for _, tracker := range trackers {
		if _, err := a.RunTrackerNow(ctx, tracker.UserID, tracker.ID); err != nil {
			a.logger.Warnw("scheduled tracker run failed", "tracker_id", tracker.ID, "error", err)
		}
	}
	return nil
}

func (a *Application) ListCollectorOperations(ctx context.Context) ([]domain.CollectorOperationsProfile, error) {
	return a.store.ListCollectorOperations(ctx)
}

func (a *Application) HealCollector(ctx context.Context, collectorID, reason string) error {
	target, err := a.store.CollectorHealingTarget(ctx, strings.TrimSpace(collectorID))
	if err != nil {
		return err
	}
	if target.TargetURL == "" {
		return errors.New("collector has no observed product URL to use for healing")
	}
	healer, ok := a.scraper.(CollectorHealer)
	if !ok {
		return errors.New("scraper does not support collector healing")
	}
	go func() {
		healCtx, cancel := context.WithTimeout(context.Background(), a.cfg.BrightDataTimeout)
		defer cancel()
		if err := healer.HealCollector(healCtx, target.Collector, target.TargetURL, target.Country, reason); err != nil {
			a.logger.Warnw("manual collector healing failed", "collector_id", target.Collector.ID, "external_collector_id", target.Collector.ExternalCollectorID, "domain", target.Profile.Domain, "error", err)
		}
	}()
	return nil
}

func rulesWithIDs(rules []domain.AlertRule) []domain.AlertRule {
	out := make([]domain.AlertRule, 0, len(rules))
	for _, rule := range rules {
		if rule.ID == "" {
			rule.ID = newID("rule")
		}
		rule.Enabled = true
		out = append(out, rule)
	}
	return out
}

func normalizePublicURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("valid absolute URL is required")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("only http and https URLs are allowed")
	}
	if parsed.User != nil {
		return "", errors.New("URLs with embedded credentials are not allowed")
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return "", errors.New("localhost URLs are not allowed")
	}
	ips, _ := net.LookupIP(host)
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() {
			return "", errors.New("private or reserved destinations are not allowed")
		}
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}

func randomToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Errorf("random token: %w", err))
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func newID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Errorf("new id: %w", err))
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(b[:])
}
