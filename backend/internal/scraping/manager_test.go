package scraping

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"clever-consumer/backend/internal/domain"
	"clever-consumer/backend/internal/logging"
)

func TestManagerHealsCollectorOnDataQualityFailure(t *testing.T) {
	store := &fakeCollectorStore{
		target: domain.ScrapeTarget{
			Collector: &domain.ScraperCollector{ID: "dsc_1", ExternalCollectorID: "collector_1"},
		},
	}
	provider := &fakeProvider{
		runErrors:  []error{DataQualityError{Err: errors.New("missing price")}},
		genericErr: errors.New("generic scrape failed"),
	}
	manager := NewManager(store, provider, domain.CollectorThresholds{MaxPerDomain: 3}, logging.Nop())

	result, err := manager.Scrape(context.Background(), "https://shop.example/products/1", "IN")
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentPrice != 42 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if provider.healCalls != 1 || store.startHealingCalls != 1 || store.finishHealingSuccess != 1 {
		t.Fatalf("expected one successful heal, provider=%d start=%d finish=%d", provider.healCalls, store.startHealingCalls, store.finishHealingSuccess)
	}
}

func TestManagerFallsBackToGenericOnCollectorDataQualityFailure(t *testing.T) {
	store := &fakeCollectorStore{
		target: domain.ScrapeTarget{
			Collector: &domain.ScraperCollector{ID: "dsc_1", ExternalCollectorID: "collector_1"},
		},
	}
	provider := &fakeProvider{
		runErrors: []error{DataQualityError{Err: errors.New("price exceeds supported product price")}},
	}
	manager := NewManager(store, provider, domain.CollectorThresholds{MaxPerDomain: 3}, logging.Nop())

	result, err := manager.Scrape(context.Background(), "https://shop.example/products/1", "IN")
	if err != nil {
		t.Fatal(err)
	}
	if result.Method != "fake_generic" {
		t.Fatalf("expected generic fallback result, got %+v", result)
	}
	waitUntil(t, time.Second, func() bool {
		return provider.healCalls == 1 && store.finishHealingSuccess == 1
	})
}

func TestManagerDoesNotHealCommandFailure(t *testing.T) {
	store := &fakeCollectorStore{
		target: domain.ScrapeTarget{
			Collector: &domain.ScraperCollector{ID: "dsc_1", ExternalCollectorID: "collector_1"},
		},
	}
	provider := &fakeProvider{
		runErrors: []error{errors.New("Bright Data API key is invalid")},
	}
	manager := NewManager(store, provider, domain.CollectorThresholds{MaxPerDomain: 3}, logging.Nop())

	_, err := manager.Scrape(context.Background(), "https://shop.example/products/1", "IN")
	if err == nil {
		t.Fatal("expected command failure")
	}
	if provider.healCalls != 0 || store.startHealingCalls != 0 {
		t.Fatalf("did not expect heal for command failure, provider=%d start=%d", provider.healCalls, store.startHealingCalls)
	}
}

func TestManagerProvisionsCollectorInBackgroundAndReturnsGenericResult(t *testing.T) {
	store := &fakeCollectorStore{
		target: domain.ScrapeTarget{
			Provision: &domain.ScraperCollector{ID: "dsc_1"},
		},
	}
	createStarted := make(chan struct{})
	createRelease := make(chan struct{})
	provider := &fakeProvider{
		createStarted: createStarted,
		createRelease: createRelease,
	}
	manager := NewManager(store, provider, domain.CollectorThresholds{MaxPerDomain: 3}, logging.Nop())

	resultCh := make(chan domain.ScrapeResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := manager.Scrape(context.Background(), "https://shop.example/products/1", "IN")
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	select {
	case result := <-resultCh:
		if result.Method != "fake_generic" {
			t.Fatalf("expected generic result while collector provisions, got %+v", result)
		}
	case err := <-errCh:
		t.Fatal(err)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("scrape blocked on collector provisioning")
	}

	select {
	case <-createStarted:
	case <-time.After(time.Second):
		t.Fatal("expected background collector creation to start")
	}
	close(createRelease)
	waitUntil(t, time.Second, func() bool {
		return store.completeProvisioningCount() == 1
	})
}

func TestManagerReturnsGenericResultAndDiscoversCollectorInBackground(t *testing.T) {
	store := &fakeCollectorStore{
		target: domain.ScrapeTarget{
			Provision: &domain.ScraperCollector{ID: "dsc_pending"},
		},
	}
	provider := &fakeProvider{
		discoveredCollectors: []DiscoveredCollector{{ExternalID: "c_shop", Name: "shop.example"}},
	}
	manager := NewManager(store, provider, domain.CollectorThresholds{MaxPerDomain: 3}, logging.Nop())

	result, err := manager.Scrape(context.Background(), "https://shop.example/products/1", "IN")
	if err != nil {
		t.Fatal(err)
	}
	if result.Method != "fake_generic" {
		t.Fatalf("expected generic result while collector discovery runs, got %+v", result)
	}
	waitUntil(t, time.Second, func() bool {
		return store.activateDiscoveredCount() == 1
	})
	if externalID := store.activatedDiscoveredExternalID(); externalID != "c_shop" {
		t.Fatalf("expected discovered collector to be stored, calls=%d external=%s", store.activateDiscoveredCount(), externalID)
	}
	if provider.createCalls != 0 {
		t.Fatalf("did not expect background collector creation when discovery succeeds, got %d calls", provider.createCalls)
	}
}

func TestManagerRecordsProvisionFailureAfterRequestCancel(t *testing.T) {
	store := &fakeCollectorStore{
		target: domain.ScrapeTarget{
			Provision: &domain.ScraperCollector{ID: "dsc_1"},
		},
	}
	provider := &fakeProvider{
		createErr: errors.New("creation canceled"),
	}
	manager := NewManager(store, provider, domain.CollectorThresholds{MaxPerDomain: 3}, logging.Nop())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := manager.Scrape(ctx, "https://shop.example/products/1", "IN")
	if err != nil {
		t.Fatal(err)
	}
	waitUntil(t, time.Second, func() bool {
		return store.failProvisioningCount() == 1
	})
	if err := store.failProvisioningContextError(); err != nil {
		t.Fatalf("expected failure recording to use live context, got %v", err)
	}
}

type fakeProvider struct {
	runErrors            []error
	genericErr           error
	createErr            error
	createStarted        chan struct{}
	createRelease        chan struct{}
	discoveredCollectors []DiscoveredCollector
	lastCollectorID      string
	createCalls          int
	healCalls            int
}

func (f *fakeProvider) Name() string {
	return "fake"
}

func (f *fakeProvider) GenericScrape(ctx context.Context, targetURL, country string) (domain.ScrapeResult, error) {
	if f.genericErr != nil {
		return domain.ScrapeResult{}, f.genericErr
	}
	return fakeResult(country, "fake_generic"), nil
}

func (f *fakeProvider) CreateCollector(ctx context.Context, targetURL, country, domainName, purpose string) (string, error) {
	f.createCalls++
	if f.createStarted != nil {
		close(f.createStarted)
	}
	if f.createRelease != nil {
		<-f.createRelease
	}
	if f.createErr != nil {
		return "", f.createErr
	}
	return "collector_created", nil
}

func (f *fakeProvider) RunCollector(ctx context.Context, collectorID, targetURL, country string) (domain.ScrapeResult, error) {
	f.lastCollectorID = collectorID
	if len(f.runErrors) > 0 {
		err := f.runErrors[0]
		f.runErrors = f.runErrors[1:]
		return domain.ScrapeResult{}, err
	}
	return fakeResult(country, "fake_collector"), nil
}

func (f *fakeProvider) HealCollector(ctx context.Context, collectorID, targetURL, country string, scrapeErr error) error {
	f.healCalls++
	return nil
}

func (f *fakeProvider) DiscoverCollectors(ctx context.Context, domainName string) ([]DiscoveredCollector, error) {
	return f.discoveredCollectors, nil
}

type fakeCollectorStore struct {
	mu                         sync.Mutex
	target                     domain.ScrapeTarget
	startHealingCalls          int
	finishHealingSuccess       int
	activateDiscoveredCalls    int
	activatedExternalID        string
	completeProvisioningCalls  int
	failProvisioningCalls      int
	failProvisioningContextErr error
}

func (f *fakeCollectorStore) PrepareScrapeTarget(ctx context.Context, domainName, targetURL, provider string, now time.Time, thresholds domain.CollectorThresholds) (domain.ScrapeTarget, error) {
	return f.target, nil
}

func (f *fakeCollectorStore) CompleteCollectorProvisioning(ctx context.Context, collectorID, externalCollectorID string, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completeProvisioningCalls++
	return nil
}

func (f *fakeCollectorStore) ActivateDiscoveredCollector(ctx context.Context, profileID, provider, externalCollectorID, purpose string, now time.Time) (domain.ScraperCollector, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.activateDiscoveredCalls++
	f.activatedExternalID = externalCollectorID
	return domain.ScraperCollector{ID: "dsc_discovered", ProfileID: profileID, Provider: provider, ExternalCollectorID: externalCollectorID, Status: domain.ScraperCollectorActive}, nil
}

func (f *fakeCollectorStore) activateDiscoveredCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.activateDiscoveredCalls
}

func (f *fakeCollectorStore) activatedDiscoveredExternalID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.activatedExternalID
}

func (f *fakeCollectorStore) FailCollectorProvisioning(ctx context.Context, collectorID, message string, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failProvisioningCalls++
	f.failProvisioningContextErr = ctx.Err()
	return nil
}

func (f *fakeCollectorStore) completeProvisioningCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.completeProvisioningCalls
}

func (f *fakeCollectorStore) failProvisioningCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.failProvisioningCalls
}

func (f *fakeCollectorStore) failProvisioningContextError() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.failProvisioningContextErr
}

func (f *fakeCollectorStore) RecordCollectorSuccess(ctx context.Context, collectorID string, now time.Time) error {
	return nil
}

func (f *fakeCollectorStore) RecordCollectorFailure(ctx context.Context, collectorID, message string, structural bool, now time.Time) error {
	return nil
}

func (f *fakeCollectorStore) StartCollectorHealing(ctx context.Context, collectorID string, now time.Time) (bool, error) {
	f.startHealingCalls++
	return true, nil
}

func (f *fakeCollectorStore) FinishCollectorHealing(ctx context.Context, collectorID, message string, success bool, now time.Time) error {
	if success {
		f.finishHealingSuccess++
	}
	return nil
}

func fakeResult(country, method string) domain.ScrapeResult {
	return domain.ScrapeResult{
		Name:         "Product",
		CanonicalURL: "https://shop.example/products/1",
		CurrentPrice: 42,
		Currency:     "INR",
		Availability: domain.AvailabilityInStock,
		Country:      country,
		Method:       method,
		Confidence:   0.82,
		ObservedAt:   time.Now().UTC(),
	}
}

func waitUntil(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}
