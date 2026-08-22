package scraping

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"clever-consumer/backend/internal/domain"
	"go.uber.org/zap"
)

type BrightDataCLI struct {
	binary      string
	apiKey      string
	zone        string
	apiBaseURL  string
	timeout     time.Duration
	healRetries int
	httpClient  *http.Client
	datasets    map[string]string
	logger      *zap.SugaredLogger
}

func NewBrightDataCLI(binary, apiKey, zone, apiBaseURL string, timeout time.Duration, healRetries int, datasets map[string]string, logger *zap.SugaredLogger) *BrightDataCLI {
	return &BrightDataCLI{
		binary:      binary,
		apiKey:      apiKey,
		zone:        zone,
		apiBaseURL:  strings.TrimRight(apiBaseURL, "/"),
		timeout:     timeout,
		healRetries: healRetries,
		httpClient:  &http.Client{},
		datasets:    normalizeDomainMap(datasets),
		logger:      logger,
	}
}

func (b *BrightDataCLI) Name() string {
	return "brightdata"
}

func (b *BrightDataCLI) GenericScrape(ctx context.Context, targetURL, country string) (domain.ScrapeResult, error) {
	if datasetID, ok := b.DatasetID(domainFromURL(targetURL)); ok {
		return b.scrapeDataset(ctx, datasetID, targetURL, country)
	}
	args := []string{"scrape", targetURL, "--format", "json", "--country", strings.ToLower(country)}
	if b.zone != "" {
		args = append(args, "--zone", b.zone)
	}
	out, err := b.run(ctx, args)
	if err != nil {
		return domain.ScrapeResult{}, b.commandError(err, out)
	}
	return decodeProductResult(out, targetURL, country, "brightdata_generic")
}

func (b *BrightDataCLI) CreateCollector(ctx context.Context, targetURL, country, domainName, purpose string) (string, error) {
	name := fmt.Sprintf("cc-%s-%s", sanitizeCollectorName(domainName), purpose)
	description := "Extract product title, current price, currency, image URL, canonical URL, and availability from this product detail page. Return structured JSON with fields name, price or current_price, currency, image_url, url, and availability."
	args := []string{
		"scraper", "create", targetURL, description,
		"--name", name,
		"--json",
		"--max-retries", strconv.Itoa(b.healRetries),
		"--timeout", strconv.Itoa(int(b.timeout.Seconds())),
	}
	out, err := b.run(ctx, args)
	if err != nil {
		return "", b.commandError(err, out)
	}
	payload, err := firstJSONValue(out)
	if err != nil {
		return "", fmt.Errorf("Bright Data scraper create returned invalid JSON: %w", err)
	}
	var envelope struct {
		CollectorID string `json:"collector_id"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return "", fmt.Errorf("Bright Data scraper create returned invalid JSON: %w", err)
	}
	if envelope.CollectorID == "" {
		if envelope.Error != "" {
			return "", fmt.Errorf("Bright Data scraper create failed: %s", envelope.Error)
		}
		return "", errors.New("Bright Data scraper create did not return a collector_id")
	}
	return envelope.CollectorID, nil
}

func (b *BrightDataCLI) RunCollector(ctx context.Context, collectorID, targetURL, country string) (domain.ScrapeResult, error) {
	if strings.HasPrefix(collectorID, "c_") && b.apiKey != "" {
		return b.runScraperStudioCollector(ctx, collectorID, targetURL, country)
	}
	args := []string{"scraper", "run", collectorID, targetURL, "--json", "--timeout", strconv.Itoa(int(b.timeout.Seconds()))}
	out, err := b.run(ctx, args)
	if err != nil {
		return domain.ScrapeResult{}, b.commandError(err, out)
	}
	return decodeProductResult(out, targetURL, country, "brightdata_collector")
}

func (b *BrightDataCLI) DatasetID(domainName string) (string, bool) {
	return lookupDomainMap(b.datasets, domainName)
}

func (b *BrightDataCLI) DiscoverCollectors(ctx context.Context, domainName string) ([]DiscoveredCollector, error) {
	domainName = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domainName)), "www.")
	if domainName == "" {
		return nil, nil
	}
	discovered := make([]DiscoveredCollector, 0)
	seen := make(map[string]struct{})
	for _, search := range collectorDiscoverySearches(domainName) {
		endpoint := b.apiURL("/dca/collectors_list", map[string]string{"search": search})
		out, err := b.apiRequest(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		var envelope struct {
			Data []struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Active bool   `json:"active"`
			} `json:"data"`
		}
		if err := json.Unmarshal(out, &envelope); err != nil {
			return nil, fmt.Errorf("Bright Data collectors_list returned invalid JSON: %w", err)
		}
		for _, item := range envelope.Data {
			if !item.Active || !strings.HasPrefix(item.ID, "c_") || !collectorNameMatchesDomain(item.Name, domainName) {
				continue
			}
			if _, ok := seen[item.ID]; ok {
				continue
			}
			seen[item.ID] = struct{}{}
			discovered = append(discovered, DiscoveredCollector{ExternalID: item.ID, Name: item.Name})
		}
	}
	return discovered, nil
}

func (b *BrightDataCLI) HealCollector(ctx context.Context, collectorID, targetURL, country string, scrapeErr error) error {
	args := []string{
		"scraper", "heal", collectorID, healingPrompt(targetURL, country, scrapeErr),
		"--url", targetURL,
		"--auto-approve",
		"--json",
		"--max-retries", strconv.Itoa(b.healRetries),
		"--timeout", strconv.Itoa(int(b.timeout.Seconds())),
	}
	out, err := b.run(ctx, args)
	if err != nil {
		return b.commandError(err, out)
	}
	return nil
}

func (b *BrightDataCLI) run(ctx context.Context, args []string) ([]byte, error) {
	runCtx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, b.binary, args...)
	cmd.Env = os.Environ()
	if b.apiKey != "" {
		cmd.Env = append(cmd.Env, "BRIGHTDATA_API_KEY="+b.apiKey)
	}
	if b.zone != "" {
		cmd.Env = append(cmd.Env, "BRIGHTDATA_UNLOCKER_ZONE="+b.zone)
	}
	out, err := cmd.CombinedOutput()
	if runCtx.Err() != nil {
		return out, runCtx.Err()
	}
	return out, err
}

func (b *BrightDataCLI) commandError(err error, out []byte) error {
	b.logger.Warnw("bright data command failed", "error", err, "output_preview", outputPreview(out))
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return fmt.Errorf("Bright Data CLI is not installed or not on PATH; install @brightdata/cli or set BRIGHTDATA_BIN")
	}
	output := strings.ToLower(string(out))
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("Bright Data command timed out after %s; increase BRIGHTDATA_TIMEOUT_SECONDS for scraper creation", b.timeout)
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("Bright Data command was canceled before completion")
	case strings.Contains(output, "invalid") && strings.Contains(output, "api"):
		return fmt.Errorf("Bright Data API key is invalid or expired; refresh BRIGHTDATA_API_KEY or run brightdata login again")
	case strings.Contains(output, "expired") && strings.Contains(output, "api"):
		return fmt.Errorf("Bright Data API key is invalid or expired; refresh BRIGHTDATA_API_KEY or run brightdata login again")
	case strings.Contains(output, "access denied"):
		return fmt.Errorf("Bright Data access denied; check API key permissions and Web Unlocker zone access")
	case strings.Contains(output, "no web unlocker zone"):
		return fmt.Errorf("Bright Data Web Unlocker zone is not configured; run brightdata login or set BRIGHTDATA_UNLOCKER_ZONE")
	case strings.Contains(output, "rate limit"):
		return fmt.Errorf("Bright Data rate limit exceeded; retry later or increase the zone limit")
	default:
		return fmt.Errorf("Bright Data command failed; verify CLI authentication, API key, and Web Unlocker zone")
	}
}

func (b *BrightDataCLI) scrapeDataset(ctx context.Context, datasetID, targetURL, country string) (domain.ScrapeResult, error) {
	payload := []map[string]string{{"url": targetURL}}
	endpoint := b.apiURL("/datasets/v3/scrape", map[string]string{
		"dataset_id": datasetID,
		"format":     "json",
	})
	out, err := b.apiRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return domain.ScrapeResult{}, err
	}
	if result, err := decodeProductResult(out, targetURL, country, "brightdata_dataset"); err == nil {
		return result, nil
	}
	snapshotID := snapshotIDFromBody(out)
	if snapshotID == "" {
		return decodeProductResult(out, targetURL, country, "brightdata_dataset")
	}
	return b.pollDatasetSnapshot(ctx, snapshotID, targetURL, country)
}

func (b *BrightDataCLI) pollDatasetSnapshot(ctx context.Context, snapshotID, targetURL, country string) (domain.ScrapeResult, error) {
	progressURL := b.apiURL("/datasets/v3/progress/"+url.PathEscape(snapshotID), nil)
	snapshotURL := b.apiURL("/datasets/v3/snapshot/"+url.PathEscape(snapshotID), map[string]string{"format": "json"})
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	deadline := time.NewTimer(b.timeout)
	defer deadline.Stop()
	for {
		progress, err := b.apiRequest(ctx, http.MethodGet, progressURL, nil)
		if err != nil {
			return domain.ScrapeResult{}, err
		}
		status := statusFromBody(progress)
		switch status {
		case "ready", "done":
			out, err := b.apiRequest(ctx, http.MethodGet, snapshotURL, nil)
			if err != nil {
				return domain.ScrapeResult{}, err
			}
			return decodeProductResult(out, targetURL, country, "brightdata_dataset")
		case "failed", "error", "canceled", "cancelled":
			return domain.ScrapeResult{}, fmt.Errorf("Bright Data dataset snapshot %s failed with status %q", snapshotID, status)
		}
		select {
		case <-ctx.Done():
			return domain.ScrapeResult{}, ctx.Err()
		case <-deadline.C:
			return domain.ScrapeResult{}, fmt.Errorf("Bright Data dataset snapshot %s timed out after %s", snapshotID, b.timeout)
		case <-ticker.C:
		}
	}
}

func (b *BrightDataCLI) runScraperStudioCollector(ctx context.Context, collectorID, targetURL, country string) (domain.ScrapeResult, error) {
	triggerURL := b.apiURL("/dca/trigger", map[string]string{
		"collector":  collectorID,
		"queue_next": "1",
	})
	payload := []map[string]string{{"url": targetURL}}
	out, err := b.apiRequest(ctx, http.MethodPost, triggerURL, payload)
	if err != nil {
		return domain.ScrapeResult{}, err
	}
	collectionID := collectionIDFromBody(out)
	if collectionID == "" {
		return domain.ScrapeResult{}, fmt.Errorf("Bright Data Scraper Studio trigger did not return a collection_id")
	}
	return b.pollScraperStudioDataset(ctx, collectionID, targetURL, country)
}

func (b *BrightDataCLI) pollScraperStudioDataset(ctx context.Context, collectionID, targetURL, country string) (domain.ScrapeResult, error) {
	datasetURL := b.apiURL("/dca/dataset", map[string]string{"id": collectionID})
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	deadline := time.NewTimer(b.timeout)
	defer deadline.Stop()
	for {
		out, err := b.apiRequest(ctx, http.MethodGet, datasetURL, nil)
		if err != nil {
			return domain.ScrapeResult{}, err
		}
		if bytes.HasPrefix(bytes.TrimSpace(out), []byte("[")) {
			return decodeProductResult(out, targetURL, country, "brightdata_collector_api")
		}
		status := statusFromBody(out)
		if status == "failed" || status == "error" || status == "canceled" || status == "cancelled" {
			return domain.ScrapeResult{}, fmt.Errorf("Bright Data Scraper Studio collection %s failed with status %q", collectionID, status)
		}
		select {
		case <-ctx.Done():
			return domain.ScrapeResult{}, ctx.Err()
		case <-deadline.C:
			return domain.ScrapeResult{}, fmt.Errorf("Bright Data Scraper Studio collection %s timed out after %s", collectionID, b.timeout)
		case <-ticker.C:
		}
	}
}

func (b *BrightDataCLI) apiRequest(ctx context.Context, method, endpoint string, body any) ([]byte, error) {
	if b.apiKey == "" {
		return nil, errors.New("BRIGHTDATA_API_KEY is required for Bright Data API requests")
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+b.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Bright Data API %s %s returned %d: %s", method, endpoint, resp.StatusCode, outputPreview(out))
	}
	return out, nil
}

func (b *BrightDataCLI) apiURL(path string, query map[string]string) string {
	base := b.apiBaseURL
	if base == "" {
		base = "https://api.brightdata.com"
	}
	values := url.Values{}
	for key, value := range query {
		values.Set(key, value)
	}
	if encoded := values.Encode(); encoded != "" {
		return base + path + "?" + encoded
	}
	return base + path
}

type genericScrapeOutput struct {
	Name         string `json:"name"`
	Title        string `json:"title"`
	ProductName  string `json:"product_name"`
	URL          string `json:"url"`
	ProductURL   string `json:"product_url"`
	Image        string `json:"image"`
	ImageURL     string `json:"image_url"`
	MainImage    string `json:"main_image"`
	Price        any    `json:"price"`
	CurrentPrice any    `json:"current_price"`
	FinalPrice   any    `json:"final_price"`
	Currency     any    `json:"currency"`
	CurrencyCode any    `json:"currency_code"`
	Availability any    `json:"availability"`
	StockStatus  any    `json:"stock_status"`
}

func decodeProductResult(out []byte, targetURL, country, method string) (domain.ScrapeResult, error) {
	parsed, err := decodeGenericScrape(out)
	if err != nil {
		return domain.ScrapeResult{}, DataQualityError{Err: fmt.Errorf("scrape result could not be decoded from Bright Data output %q: %w", outputPreview(out), err)}
	}
	price := numberValue(parsed.CurrentPrice)
	if price == 0 {
		price = numberValue(parsed.Price)
	}
	if price == 0 {
		price = numberValue(parsed.FinalPrice)
	}
	name := parsed.Name
	if name == "" {
		name = parsed.Title
	}
	if name == "" {
		name = parsed.ProductName
	}
	image := parsed.ImageURL
	if image == "" {
		image = parsed.Image
	}
	if image == "" {
		image = parsed.MainImage
	}
	currency := coalesce(
		stringValue(parsed.Currency),
		stringValue(parsed.CurrencyCode),
		currencyFromPrice(parsed.CurrentPrice),
		currencyFromPrice(parsed.Price),
		currencyFromPrice(parsed.FinalPrice),
	)
	if name == "" || currency == "" {
		return domain.ScrapeResult{}, DataQualityError{Err: errors.New("scrape result missing required product fields")}
	}
	if price <= 0 {
		return domain.ScrapeResult{}, DataQualityError{Err: errors.New("scrape result missing a valid product price")}
	}
	availability := coalesce(availabilityValue(parsed.Availability), availabilityValue(parsed.StockStatus))
	if availability == "" {
		availability = domain.AvailabilityUnknown
	}
	return domain.ScrapeResult{
		Name:         name,
		CanonicalURL: coalesce(parsed.URL, parsed.ProductURL, targetURL),
		ImageURL:     image,
		CurrentPrice: price,
		Currency:     strings.ToUpper(currency),
		Availability: availability,
		Country:      strings.ToUpper(country),
		Method:       method,
		Confidence:   0.82,
		ObservedAt:   time.Now().UTC(),
	}, nil
}

func healingPrompt(targetURL, country string, scrapeErr error) string {
	prompt := fmt.Sprintf("Product scrape for %s returned invalid product data: %v. Update the scraper so it returns name or title, current_price or price, currency, image_url or image, canonical url, and availability as structured JSON. Country: %s.", targetURL, scrapeErr, strings.ToUpper(country))
	if len(prompt) <= 1000 {
		return prompt
	}
	return prompt[:1000]
}

func decodeGenericScrape(out []byte) (genericScrapeOutput, error) {
	payload, err := firstJSONValue(out)
	if err != nil {
		return genericScrapeOutput{}, err
	}
	if err := brightDataPayloadError(payload); err != nil {
		return genericScrapeOutput{}, err
	}

	var parsed genericScrapeOutput
	if err := json.Unmarshal(payload, &parsed); err == nil && hasProductFields(parsed) {
		return parsed, nil
	}
	var items []genericScrapeOutput
	if err := json.Unmarshal(payload, &items); err != nil {
		return genericScrapeOutput{}, err
	}
	if len(items) == 0 {
		return genericScrapeOutput{}, errors.New("scrape result is empty")
	}
	return items[0], nil
}

func brightDataPayloadError(payload []byte) error {
	var envelope struct {
		StatusCode int               `json:"status_code"`
		Error      string            `json:"error"`
		Headers    map[string]string `json:"headers"`
		Body       string            `json:"body"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil
	}
	message := coalesce(envelope.Error, envelope.Headers["x-brd-error"], envelope.Body)
	if envelope.StatusCode == 0 && message == "" {
		return nil
	}
	if message == "" {
		message = "request failed"
	}
	if envelope.StatusCode > 0 {
		return fmt.Errorf("Bright Data scrape returned status %d: %s", envelope.StatusCode, message)
	}
	return fmt.Errorf("Bright Data scrape failed: %s", message)
}

func hasProductFields(value genericScrapeOutput) bool {
	return coalesce(value.Name, value.Title, value.ProductName, stringValue(value.Currency), stringValue(value.CurrencyCode), availabilityValue(value.Availability), availabilityValue(value.StockStatus)) != "" ||
		numberValue(value.Price) > 0 || numberValue(value.CurrentPrice) > 0 || numberValue(value.FinalPrice) > 0
}

func collectionIDFromBody(out []byte) string {
	var envelope struct {
		CollectionID string `json:"collection_id"`
		SnapshotID   string `json:"snapshot_id"`
		ID           string `json:"id"`
	}
	_ = json.Unmarshal(bytes.TrimSpace(out), &envelope)
	return coalesce(envelope.CollectionID, envelope.SnapshotID, envelope.ID)
}

func snapshotIDFromBody(out []byte) string {
	var envelope struct {
		SnapshotID string `json:"snapshot_id"`
		ID         string `json:"id"`
	}
	_ = json.Unmarshal(bytes.TrimSpace(out), &envelope)
	return coalesce(envelope.SnapshotID, envelope.ID)
}

func statusFromBody(out []byte) string {
	var envelope struct {
		Status string `json:"status"`
		State  string `json:"state"`
	}
	_ = json.Unmarshal(bytes.TrimSpace(out), &envelope)
	return strings.ToLower(coalesce(envelope.Status, envelope.State))
}

func numberValue(value any) float64 {
	switch typed := value.(type) {
	case nil:
		return 0
	case float64:
		return typed
	case json.Number:
		out, _ := typed.Float64()
		return out
	case map[string]any:
		for _, key := range []string{"value", "amount", "price", "current_price", "final_price"} {
			if out := numberValue(typed[key]); out > 0 {
				return out
			}
		}
		return 0
	case []any:
		for _, item := range typed {
			if out := numberValue(item); out > 0 {
				return out
			}
		}
		return 0
	case string:
		cleaned := strings.Map(func(r rune) rune {
			switch {
			case r >= '0' && r <= '9':
				return r
			case r == '.':
				return r
			default:
				return -1
			}
		}, typed)
		out, _ := strconv.ParseFloat(cleaned, 64)
		return out
	default:
		return 0
	}
}

func currencyFromPrice(value any) string {
	fields, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	return coalesce(stringValue(fields["currency"]), stringValue(fields["currency_code"]))
}

func availabilityValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case bool:
		if typed {
			return domain.AvailabilityInStock
		}
		return domain.AvailabilityOutOfStock
	case string:
		return strings.TrimSpace(typed)
	default:
		return stringValue(typed)
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}

func firstJSONValue(out []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(out)
	for i, b := range trimmed {
		if b != '{' && b != '[' {
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(trimmed[i:]))
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err == nil {
			return raw, nil
		}
	}
	return nil, errors.New("no JSON object or array found in Bright Data output")
}

func normalizeDomainMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	normalized := make(map[string]string, len(values))
	for key, value := range values {
		domainName := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(key)), "www.")
		if domainName != "" && strings.TrimSpace(value) != "" {
			normalized[domainName] = strings.TrimSpace(value)
		}
	}
	return normalized
}

func lookupDomainMap(values map[string]string, domainName string) (string, bool) {
	domainName = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domainName)), "www.")
	for domainName != "" {
		if value, ok := values[domainName]; ok && value != "" {
			return value, true
		}
		_, rest, ok := strings.Cut(domainName, ".")
		if !ok {
			break
		}
		domainName = rest
	}
	return "", false
}

func outputPreview(out []byte) string {
	preview := strings.Join(strings.Fields(string(out)), " ")
	if len(preview) <= 240 {
		return preview
	}
	return preview[:240]
}

func sanitizeCollectorName(value string) string {
	value = strings.ToLower(value)
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}
	name := strings.Trim(builder.String(), "-")
	if name == "" {
		return "domain"
	}
	return name
}

func collectorDiscoverySearches(domainName string) []string {
	candidates := []string{
		domainName,
		sanitizeCollectorName(domainName),
		firstDomainLabel(domainName),
	}
	searches := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		searches = append(searches, candidate)
	}
	return searches
}

func collectorNameMatchesDomain(name, domainName string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	normalizedName := sanitizeCollectorName(name)
	normalizedDomain := sanitizeCollectorName(domainName)
	return strings.Contains(name, domainName) ||
		strings.Contains(normalizedName, normalizedDomain)
}

func firstDomainLabel(domainName string) string {
	label, _, ok := strings.Cut(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domainName)), "www."), ".")
	if !ok {
		return strings.TrimSpace(domainName)
	}
	return label
}

func coalesce(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
