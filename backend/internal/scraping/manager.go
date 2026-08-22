package scraping

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"clever-consumer/backend/internal/domain"
	"go.uber.org/zap"
)

type Provider interface {
	Name() string
	GenericScrape(ctx context.Context, targetURL, country string) (domain.ScrapeResult, error)
	CreateCollector(ctx context.Context, targetURL, country, domainName, purpose string) (string, error)
	RunCollector(ctx context.Context, collectorID, targetURL, country string) (domain.ScrapeResult, error)
	HealCollector(ctx context.Context, collectorID, targetURL, country string, scrapeErr error) error
}

type DiscoveredCollector struct {
	ExternalID string
	Name       string
}

type CollectorDiscoveryProvider interface {
	DiscoverCollectors(ctx context.Context, domainName string) ([]DiscoveredCollector, error)
}

type CollectorStore interface {
	PrepareScrapeTarget(ctx context.Context, domainName, targetURL, provider string, now time.Time, thresholds domain.CollectorThresholds) (domain.ScrapeTarget, error)
	ActivateDiscoveredCollector(ctx context.Context, profileID, provider, externalCollectorID, purpose string, now time.Time) (domain.ScraperCollector, error)
	CompleteCollectorProvisioning(ctx context.Context, collectorID, externalCollectorID string, now time.Time) error
	FailCollectorProvisioning(ctx context.Context, collectorID, message string, now time.Time) error
	RecordCollectorSuccess(ctx context.Context, collectorID string, now time.Time) error
	RecordCollectorFailure(ctx context.Context, collectorID, message string, structural bool, now time.Time) error
	StartCollectorHealing(ctx context.Context, collectorID string, now time.Time) (bool, error)
	FinishCollectorHealing(ctx context.Context, collectorID, message string, success bool, now time.Time) error
}

type CollectorThresholds = domain.CollectorThresholds

type Manager struct {
	store      CollectorStore
	provider   Provider
	thresholds domain.CollectorThresholds
	logger     *zap.SugaredLogger
}

func NewManager(store CollectorStore, provider Provider, thresholds domain.CollectorThresholds, logger *zap.SugaredLogger) *Manager {
	return &Manager{store: store, provider: provider, thresholds: thresholds, logger: logger}
}

func (m *Manager) Scrape(ctx context.Context, targetURL, country string) (domain.ScrapeResult, error) {
	domainName := domainFromURL(targetURL)
	if domainName == "" {
		return m.provider.GenericScrape(ctx, targetURL, country)
	}

	target, err := m.store.PrepareScrapeTarget(ctx, domainName, targetURL, m.provider.Name(), time.Now().UTC(), m.thresholds)
	if err != nil {
		return domain.ScrapeResult{}, err
	}

	if target.Collector == nil {
		if collector, ok := m.discoverCollector(ctx, target, domainName); ok {
			result, err := m.provider.RunCollector(ctx, collector.ExternalCollectorID, targetURL, country)
			if err == nil {
				_ = m.store.RecordCollectorSuccess(ctx, collector.ID, time.Now().UTC())
				return result, nil
			}
			_ = m.store.RecordCollectorFailure(ctx, collector.ID, err.Error(), IsDataQualityError(err), time.Now().UTC())
			m.logger.Warnw("discovered collector failed; falling back to generic scrape", "domain", domainName, "collector_id", collector.ExternalCollectorID, "error", err)
		}
		if target.Provision != nil {
			m.provisionCollectorInBackground(*target.Provision, targetURL, country, domainName)
		}
		return m.provider.GenericScrape(ctx, targetURL, country)
	}

	result, err := m.provider.RunCollector(ctx, target.Collector.ExternalCollectorID, targetURL, country)
	if err == nil {
		_ = m.store.RecordCollectorSuccess(ctx, target.Collector.ID, time.Now().UTC())
		if target.Provision != nil {
			go func() {
				_, _ = m.provisionCollector(context.Background(), *target.Provision, targetURL, country, domainName)
			}()
		}
		return result, nil
	}

	structural := IsDataQualityError(err)
	_ = m.store.RecordCollectorFailure(ctx, target.Collector.ID, err.Error(), structural, time.Now().UTC())
	if !structural {
		return domain.ScrapeResult{}, err
	}

	healingStarted, startErr := m.store.StartCollectorHealing(ctx, target.Collector.ID, time.Now().UTC())
	if startErr != nil {
		return domain.ScrapeResult{}, startErr
	}
	if !healingStarted {
		return domain.ScrapeResult{}, err
	}

	if healErr := m.provider.HealCollector(ctx, target.Collector.ExternalCollectorID, targetURL, country, err); healErr != nil {
		_ = m.store.FinishCollectorHealing(ctx, target.Collector.ID, healErr.Error(), false, time.Now().UTC())
		return domain.ScrapeResult{}, errors.Join(err, healErr)
	}
	_ = m.store.FinishCollectorHealing(ctx, target.Collector.ID, "", true, time.Now().UTC())

	result, err = m.provider.RunCollector(ctx, target.Collector.ExternalCollectorID, targetURL, country)
	if err != nil {
		_ = m.store.RecordCollectorFailure(ctx, target.Collector.ID, err.Error(), IsDataQualityError(err), time.Now().UTC())
		return domain.ScrapeResult{}, err
	}
	_ = m.store.RecordCollectorSuccess(ctx, target.Collector.ID, time.Now().UTC())
	return result, nil
}

func (m *Manager) provisionCollectorInBackground(collector domain.ScraperCollector, targetURL, country, domainName string) {
	go func() {
		if _, err := m.provisionCollector(context.Background(), collector, targetURL, country, domainName); err != nil {
			m.logger.Warnw("collector provisioning failed", "domain", domainName, "collector_id", collector.ID, "error", err)
		}
	}()
}

func (m *Manager) discoverCollector(ctx context.Context, target domain.ScrapeTarget, domainName string) (domain.ScraperCollector, bool) {
	provider, ok := m.provider.(CollectorDiscoveryProvider)
	if !ok {
		return domain.ScraperCollector{}, false
	}
	discovered, err := provider.DiscoverCollectors(ctx, domainName)
	if err != nil {
		m.logger.Warnw("collector discovery failed", "domain", domainName, "error", err)
		return domain.ScraperCollector{}, false
	}
	if len(discovered) == 0 {
		return domain.ScraperCollector{}, false
	}
	storeCtx, cancel := collectorStoreContext()
	defer cancel()
	collector, err := m.store.ActivateDiscoveredCollector(storeCtx, target.Profile.ID, m.provider.Name(), discovered[0].ExternalID, domain.ScraperCollectorPurposeDefault, time.Now().UTC())
	if err != nil {
		m.logger.Warnw("store discovered collector failed", "domain", domainName, "collector_id", discovered[0].ExternalID, "error", err)
		return domain.ScraperCollector{}, false
	}
	return collector, true
}

func (m *Manager) provisionAndRun(ctx context.Context, collector domain.ScraperCollector, targetURL, country, domainName string) (domain.ScrapeResult, error) {
	externalID, err := m.provisionCollector(ctx, collector, targetURL, country, domainName)
	if err != nil {
		return domain.ScrapeResult{}, err
	}
	result, err := m.provider.RunCollector(ctx, externalID, targetURL, country)
	if err != nil {
		_ = m.store.RecordCollectorFailure(ctx, collector.ID, err.Error(), IsDataQualityError(err), time.Now().UTC())
		return domain.ScrapeResult{}, err
	}
	_ = m.store.RecordCollectorSuccess(ctx, collector.ID, time.Now().UTC())
	return result, nil
}

func (m *Manager) provisionCollector(ctx context.Context, collector domain.ScraperCollector, targetURL, country, domainName string) (string, error) {
	externalID, err := m.provider.CreateCollector(ctx, targetURL, country, domainName, collector.Purpose)
	if err != nil {
		storeCtx, cancel := collectorStoreContext()
		defer cancel()
		_ = m.store.FailCollectorProvisioning(storeCtx, collector.ID, err.Error(), time.Now().UTC())
		return "", err
	}
	storeCtx, cancel := collectorStoreContext()
	defer cancel()
	if err := m.store.CompleteCollectorProvisioning(storeCtx, collector.ID, externalID, time.Now().UTC()); err != nil {
		return "", err
	}
	return externalID, nil
}

func collectorStoreContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

type DataQualityError struct {
	Err error
}

func (e DataQualityError) Error() string {
	return e.Err.Error()
}

func (e DataQualityError) Unwrap() error {
	return e.Err
}

func IsDataQualityError(err error) bool {
	var dataErr DataQualityError
	return errors.As(err, &dataErr)
}

func domainFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
}
