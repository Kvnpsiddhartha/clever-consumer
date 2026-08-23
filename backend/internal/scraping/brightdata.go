package scraping

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"clever-consumer/backend/internal/domain"
	"go.uber.org/zap"
)

type BrightDataAPI struct {
	apiKey      string
	zone        string
	apiBaseURL  string
	timeout     time.Duration
	healRetries int
	httpClient  *http.Client
	datasets    map[string]string
	logger      *zap.SugaredLogger
}

func NewBrightDataAPI(apiKey, zone, apiBaseURL string, timeout time.Duration, healRetries int, datasets map[string]string, logger *zap.SugaredLogger) *BrightDataAPI {
	return &BrightDataAPI{
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

func (b *BrightDataAPI) Name() string {
	return "brightdata"
}

func (b *BrightDataAPI) GenericScrape(ctx context.Context, targetURL, country string) (domain.ScrapeResult, error) {
	if datasetID, ok := b.DatasetID(domainFromURL(targetURL)); ok {
		return b.scrapeDataset(ctx, datasetID, targetURL, country)
	}
	return b.scrapeUnlocker(ctx, targetURL, country)
}

func (b *BrightDataAPI) CreateCollector(ctx context.Context, targetURL, country, domainName, purpose string) (string, error) {
	name := fmt.Sprintf("cc-%s-%s", sanitizeCollectorName(domainName), purpose)
	description := scraperDescription()
	collectorID, err := b.createScraperTemplate(ctx, name)
	if err != nil {
		return "", err
	}
	endpoint := b.apiURL("/dca/collectors/"+url.PathEscape(collectorID)+"/automate_template", nil)
	_, err = b.apiRequestWithRetry(ctx, http.MethodPost, endpoint, map[string]any{
		"description": truncateString(description, 500),
		"urls":        []string{targetURL},
	})
	if err != nil {
		return "", err
	}
	return collectorID, b.pollAIFlow(ctx, collectorID, "automate_template")
}

func (b *BrightDataAPI) RunCollector(ctx context.Context, collectorID, targetURL, country string) (domain.ScrapeResult, error) {
	return b.runScraperStudioCollector(ctx, collectorID, targetURL, country)
}

func (b *BrightDataAPI) DatasetID(domainName string) (string, bool) {
	if datasetID, ok := lookupDomainMap(b.datasets, domainName); ok {
		return datasetID, true
	}
	return defaultDatasetID(domainName)
}

func (b *BrightDataAPI) DiscoverCollectors(ctx context.Context, domainName string) ([]DiscoveredCollector, error) {
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

func (b *BrightDataAPI) HealCollector(ctx context.Context, collectorID, targetURL, country string, scrapeErr error) error {
	endpoint := b.apiURL("/dca/collectors/"+url.PathEscape(collectorID)+"/refactor_template", nil)
	_, err := b.apiRequestWithRetry(ctx, http.MethodPost, endpoint, map[string]any{
		"prompt":       healingPrompt(targetURL, country, scrapeErr),
		"custom_input": []map[string]string{{"url": targetURL}},
	})
	if err != nil {
		return err
	}
	status, err := b.pollAIFlowStatus(ctx, collectorID, "refactor_template")
	if err != nil {
		return err
	}
	if status == "awaiting_approval" || status == "pending_answer" || status == "user_approval" {
		approveURL := b.apiURL("/dca/collectors/"+url.PathEscape(collectorID)+"/resume_automation_job", nil)
		if _, err := b.apiRequest(ctx, http.MethodPost, approveURL, map[string]any{"message": true, "auto_save": true}); err != nil {
			return err
		}
		_, err = b.pollAIFlowStatus(ctx, collectorID, "refactor_template")
		return err
	}
	return nil
}

func (b *BrightDataAPI) scrapeDataset(ctx context.Context, datasetID, targetURL, country string) (domain.ScrapeResult, error) {
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

func (b *BrightDataAPI) scrapeUnlocker(ctx context.Context, targetURL, country string) (domain.ScrapeResult, error) {
	if b.zone == "" {
		return domain.ScrapeResult{}, errors.New("BRIGHTDATA_UNLOCKER_ZONE is required for Bright Data Unlocker API requests")
	}
	out, err := b.apiRequest(ctx, http.MethodPost, b.apiURL("/request", nil), map[string]any{
		"zone":    b.zone,
		"url":     targetURL,
		"format":  "json",
		"country": strings.ToLower(country),
	})
	if err != nil {
		return domain.ScrapeResult{}, err
	}
	return decodeProductResult(out, targetURL, country, "brightdata_generic_api")
}

func (b *BrightDataAPI) createScraperTemplate(ctx context.Context, name string) (string, error) {
	out, err := b.apiRequest(ctx, http.MethodPost, b.apiURL("/dca/collector", nil), map[string]any{
		"name": name,
		"deliver": map[string]any{
			"type":          "webhook",
			"endpoint":      "https://example.com/webhook",
			"flatten_csv":   false,
			"delivery_type": "deliver_results",
		},
	})
	if err != nil {
		return "", err
	}
	collectorID := collectorIDFromTemplateBody(out)
	if collectorID == "" {
		return "", errors.New("Bright Data scraper create did not return a collector id")
	}
	return collectorID, nil
}

func (b *BrightDataAPI) pollDatasetSnapshot(ctx context.Context, snapshotID, targetURL, country string) (domain.ScrapeResult, error) {
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

func (b *BrightDataAPI) runScraperStudioCollector(ctx context.Context, collectorID, targetURL, country string) (domain.ScrapeResult, error) {
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

func (b *BrightDataAPI) pollScraperStudioDataset(ctx context.Context, collectionID, targetURL, country string) (domain.ScrapeResult, error) {
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

func (b *BrightDataAPI) pollAIFlow(ctx context.Context, collectorID, flow string) error {
	status, err := b.pollAIFlowStatus(ctx, collectorID, flow)
	if err != nil {
		return err
	}
	if status != "done" && status != "ready" && status != "completed" && status != "success" && status != "finished" {
		return fmt.Errorf("Bright Data %s for collector %s stopped with status %q", flow, collectorID, status)
	}
	return nil
}

func (b *BrightDataAPI) pollAIFlowStatus(ctx context.Context, collectorID, flow string) (string, error) {
	progressURL := b.apiURL("/dca/collectors/"+url.PathEscape(collectorID)+"/"+flow+"/progress", nil)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	deadline := time.NewTimer(b.timeout)
	defer deadline.Stop()
	for {
		out, err := b.apiRequest(ctx, http.MethodGet, progressURL, nil)
		if err != nil {
			return "", err
		}
		status := statusFromBody(out)
		step := stepFromBody(out)
		switch {
		case isDoneStatus(status):
			return status, nil
		case isAwaitingApproval(status, step):
			if step == "user_approval" {
				return step, nil
			}
			return status, nil
		case isFailedStatus(status):
			return "", fmt.Errorf("Bright Data %s for collector %s failed with status %q", flow, collectorID, status)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline.C:
			return "", fmt.Errorf("Bright Data %s for collector %s timed out after %s", flow, collectorID, b.timeout)
		case <-ticker.C:
		}
	}
}

func (b *BrightDataAPI) apiRequestWithRetry(ctx context.Context, method, endpoint string, body any) ([]byte, error) {
	var out []byte
	var err error
	for attempt := 0; attempt <= b.healRetries; attempt++ {
		out, err = b.apiRequest(ctx, method, endpoint, body)
		var apiErr BrightDataAPIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests || attempt == b.healRetries {
			return out, err
		}
		wait := time.Duration(1<<attempt) * time.Second
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	return out, err
}

func (b *BrightDataAPI) apiRequest(ctx context.Context, method, endpoint string, body any) ([]byte, error) {
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
		return nil, BrightDataAPIError{Method: method, Endpoint: endpoint, StatusCode: resp.StatusCode, Body: outputPreview(out)}
	}
	return out, nil
}

type BrightDataAPIError struct {
	Method     string
	Endpoint   string
	StatusCode int
	Body       string
}

func (e BrightDataAPIError) Error() string {
	return fmt.Sprintf("Bright Data API %s %s returned %d: %s", e.Method, e.Endpoint, e.StatusCode, e.Body)
}

func (b *BrightDataAPI) apiURL(path string, query map[string]string) string {
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
	Name          any `json:"name"`
	Title         any `json:"title"`
	ProductName   any `json:"product_name"`
	URL           any `json:"url"`
	ProductURL    any `json:"product_url"`
	Image         any `json:"image"`
	ImageURL      any `json:"image_url"`
	MainImage     any `json:"main_image"`
	Price         any `json:"price"`
	CurrentPrice  any `json:"current_price"`
	FinalPrice    any `json:"final_price"`
	Currency      any `json:"currency"`
	CurrencyCode  any `json:"currency_code"`
	PriceCurrency any `json:"priceCurrency"`
	Availability  any `json:"availability"`
	StockStatus   any `json:"stock_status"`
	Offers        any `json:"offers"`
}

const maxSupportedProductPrice = 9_999_999_999.99

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
	if price == 0 {
		price = numberValue(offerField(parsed.Offers, "price", "current_price", "final_price", "amount"))
	}
	name := firstStringValue(parsed.Name)
	if name == "" {
		name = firstStringValue(parsed.Title)
	}
	if name == "" {
		name = firstStringValue(parsed.ProductName)
	}
	image := firstStringValue(parsed.ImageURL)
	if image == "" {
		image = firstStringValue(parsed.Image)
	}
	if image == "" {
		image = firstStringValue(parsed.MainImage)
	}
	currency := coalesce(
		firstStringValue(parsed.Currency),
		firstStringValue(parsed.CurrencyCode),
		firstStringValue(parsed.PriceCurrency),
		firstStringValue(offerField(parsed.Offers, "priceCurrency", "price_currency", "currency", "currency_code")),
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
	if price > maxSupportedProductPrice {
		return domain.ScrapeResult{}, DataQualityError{Err: fmt.Errorf("scrape result price %.2f exceeds supported product price", price)}
	}
	availability := coalesce(
		availabilityValue(parsed.Availability),
		availabilityValue(parsed.StockStatus),
		availabilityValue(offerField(parsed.Offers, "availability", "stock_status")),
	)
	if availability == "" {
		availability = domain.AvailabilityUnknown
	}
	return domain.ScrapeResult{
		Name:         name,
		CanonicalURL: coalesce(firstStringValue(parsed.URL), firstStringValue(parsed.ProductURL), firstStringValue(offerField(parsed.Offers, "url")), targetURL),
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
	if bytes.HasPrefix(bytes.TrimSpace(out), []byte("<")) {
		return decodeProductHTML(out)
	}
	payload, err := firstJSONValue(out)
	if err != nil {
		return genericScrapeOutput{}, err
	}
	if err := brightDataPayloadError(payload); err != nil {
		return genericScrapeOutput{}, err
	}
	if unwrapped, ok := unlockerBodyPayload(payload); ok {
		payload = unwrapped
		if err := brightDataPayloadError(payload); err != nil {
			return genericScrapeOutput{}, err
		}
	}

	parsed, jsonErr := decodeProductJSON(payload)
	if jsonErr == nil {
		return parsed, nil
	}
	if parsed, htmlErr := decodeProductHTML(payload); htmlErr == nil {
		return parsed, nil
	}
	return genericScrapeOutput{}, jsonErr
}

func decodeProductJSON(payload []byte) (genericScrapeOutput, error) {
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(payload)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return genericScrapeOutput{}, errors.New("scrape result is JSON but does not contain product fields")
	}
	if parsed, ok := productOutputFromValue(value); ok {
		return parsed, nil
	}
	if items, ok := value.([]any); ok && len(items) == 0 {
		return genericScrapeOutput{}, errors.New("scrape result is empty")
	}
	return genericScrapeOutput{}, errors.New("scrape result is JSON but does not contain product fields")
}

var jsonLDScriptPattern = regexp.MustCompile(`(?is)<script[^>]+type=["']application/ld\+json["'][^>]*>(.*?)</script>`)

func decodeProductHTML(payload []byte) (genericScrapeOutput, error) {
	for _, match := range jsonLDScriptPattern.FindAllSubmatch(payload, -1) {
		if len(match) < 2 {
			continue
		}
		script := strings.TrimSpace(html.UnescapeString(string(match[1])))
		if script == "" {
			continue
		}
		if parsed, err := decodeProductJSON([]byte(script)); err == nil {
			return parsed, nil
		}
	}
	return genericScrapeOutput{}, errors.New("scrape result HTML does not contain product JSON-LD")
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
	if envelope.StatusCode >= 200 && envelope.StatusCode < 300 {
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

func unlockerBodyPayload(payload []byte) ([]byte, bool) {
	var envelope struct {
		StatusCode int `json:"status_code"`
		Body       any `json:"body"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil || envelope.StatusCode < 200 || envelope.StatusCode >= 300 || envelope.Body == nil {
		return nil, false
	}
	switch body := envelope.Body.(type) {
	case string:
		trimmed := strings.TrimSpace(body)
		if trimmed == "" {
			return nil, false
		}
		return []byte(trimmed), true
	default:
		out, err := json.Marshal(body)
		return out, err == nil
	}
}

func hasProductFields(value genericScrapeOutput) bool {
	return coalesce(
		firstStringValue(value.Name),
		firstStringValue(value.Title),
		firstStringValue(value.ProductName),
		firstStringValue(value.Currency),
		firstStringValue(value.CurrencyCode),
		firstStringValue(value.PriceCurrency),
		firstStringValue(offerField(value.Offers, "priceCurrency", "price_currency", "currency", "currency_code")),
		availabilityValue(value.Availability),
		availabilityValue(value.StockStatus),
		availabilityValue(offerField(value.Offers, "availability", "stock_status")),
	) != "" || numberValue(value.Price) > 0 || numberValue(value.CurrentPrice) > 0 || numberValue(value.FinalPrice) > 0 || numberValue(offerField(value.Offers, "price", "current_price", "final_price", "amount")) > 0
}

func scraperDescription() string {
	return "Extract product title, current price, currency, image URL, canonical URL, and availability from this product detail page. Return structured JSON with fields name, price or current_price, currency, image_url, url, and availability."
}

func truncateString(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
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

func collectorIDFromTemplateBody(out []byte) string {
	var envelope struct {
		ID          string `json:"id"`
		CollectorID string `json:"collector_id"`
		Data        struct {
			ID          string `json:"id"`
			CollectorID string `json:"collector_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(bytes.TrimSpace(out), &envelope)
	return coalesce(envelope.ID, envelope.CollectorID, envelope.Data.ID, envelope.Data.CollectorID)
}

func statusFromBody(out []byte) string {
	var envelope struct {
		Status string `json:"status"`
		State  string `json:"state"`
	}
	_ = json.Unmarshal(bytes.TrimSpace(out), &envelope)
	return strings.ToLower(coalesce(envelope.Status, envelope.State))
}

func stepFromBody(out []byte) string {
	var envelope struct {
		Step string `json:"step"`
	}
	_ = json.Unmarshal(bytes.TrimSpace(out), &envelope)
	return strings.ToLower(envelope.Step)
}

func isDoneStatus(status string) bool {
	switch status {
	case "done", "ready", "completed", "success", "finished":
		return true
	default:
		return false
	}
}

func isFailedStatus(status string) bool {
	switch status {
	case "failed", "error", "canceled", "cancelled", "rejected":
		return true
	default:
		return false
	}
}

func isAwaitingApproval(status, step string) bool {
	switch status {
	case "awaiting_approval", "pending_answer", "user_approval":
		return true
	}
	return step == "user_approval"
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
		return priceFromString(typed)
	default:
		return 0
	}
}

var (
	currencyPricePattern = regexp.MustCompile(`(?i)(?:inr|rs\.?|\p{Sc})\s*([0-9][0-9,\s]*(?:\.[0-9]+)?)`)
	plainPricePattern    = regexp.MustCompile(`(?:^|[^A-Za-z0-9])([0-9]{1,3}(?:[,\s][0-9]{3})+|[0-9]{1,3}(?:,[0-9]{2})+(?:,[0-9]{3})?|[0-9]+(?:\.[0-9]+)?)(?:[^A-Za-z0-9]|$)`)
)

func priceFromString(value string) float64 {
	for _, pattern := range []*regexp.Regexp{currencyPricePattern, plainPricePattern} {
		match := pattern.FindStringSubmatch(value)
		if len(match) < 2 {
			continue
		}
		cleaned := strings.NewReplacer(",", "", " ", "").Replace(strings.TrimSpace(match[1]))
		out, _ := strconv.ParseFloat(cleaned, 64)
		if out > 0 {
			return out
		}
	}
	return 0
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
		return normalizeAvailability(typed)
	default:
		return normalizeAvailability(firstStringValue(typed))
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

func firstStringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case []any:
		for _, item := range typed {
			if out := firstStringValue(item); out != "" {
				return out
			}
		}
		return ""
	case map[string]any:
		for _, key := range []string{"url", "href", "src", "value", "text", "name"} {
			if out := firstStringValue(typed[key]); out != "" {
				return out
			}
		}
		return ""
	default:
		return stringValue(typed)
	}
}

func normalizeAvailability(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	lowered := strings.ToLower(trimmed)
	if idx := strings.LastIndexAny(lowered, "/#"); idx >= 0 && idx < len(lowered)-1 {
		lowered = lowered[idx+1:]
	}
	compact := strings.NewReplacer(" ", "", "_", "", "-", "").Replace(lowered)
	switch {
	case strings.Contains(compact, "instock") || strings.Contains(compact, "available"):
		return domain.AvailabilityInStock
	case strings.Contains(compact, "outofstock") || strings.Contains(compact, "soldout") || strings.Contains(compact, "unavailable"):
		return domain.AvailabilityOutOfStock
	case strings.Contains(compact, "preorder") || strings.Contains(compact, "presale"):
		return domain.AvailabilityPreorder
	default:
		return trimmed
	}
}

func offerField(offers any, keys ...string) any {
	switch typed := offers.(type) {
	case []any:
		for _, item := range typed {
			if out := offerField(item, keys...); out != nil {
				return out
			}
		}
	case map[string]any:
		for _, key := range keys {
			if value, ok := typed[key]; ok {
				return value
			}
		}
	}
	return nil
}

func productOutputFromValue(value any) (genericScrapeOutput, bool) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if parsed, ok := productOutputFromValue(item); ok {
				return parsed, true
			}
		}
	case map[string]any:
		out, err := json.Marshal(typed)
		if err == nil {
			var parsed genericScrapeOutput
			if err := json.Unmarshal(out, &parsed); err == nil && hasProductFields(parsed) {
				return parsed, true
			}
		}
		for _, key := range []string{"product", "data", "item", "@graph", "graph", "items", "products", "results"} {
			if parsed, ok := productOutputFromValue(typed[key]); ok {
				return parsed, true
			}
		}
	}
	return genericScrapeOutput{}, false
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

func defaultDatasetID(domainName string) (string, bool) {
	domainName = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domainName)), "www.")
	if domainName == "" {
		return "", false
	}
	for _, suffix := range []string{
		"amazon.ae",
		"amazon.ca",
		"amazon.co.jp",
		"amazon.co.uk",
		"amazon.com",
		"amazon.com.au",
		"amazon.com.br",
		"amazon.com.mx",
		"amazon.de",
		"amazon.es",
		"amazon.fr",
		"amazon.in",
		"amazon.it",
		"amazon.nl",
		"amazon.sa",
		"amazon.sg",
	} {
		if domainName == suffix || strings.HasSuffix(domainName, "."+suffix) {
			return "gd_l7q7dkf244hwjntr0", true
		}
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
