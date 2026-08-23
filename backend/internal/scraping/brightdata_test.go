package scraping

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"clever-consumer/backend/internal/logging"
)

func TestBrightDataAPIDoesNotFallbackWhenAPIKeyMissing(t *testing.T) {
	scraper := NewBrightDataAPI("", "cli_unlocker", "", time.Second, 1, nil, logging.Nop())

	_, err := scraper.GenericScrape(context.Background(), "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN")
	if err == nil {
		t.Fatal("expected scrape to fail when Bright Data API key is unavailable")
	}
	if strings.Contains(err.Error(), "999") || strings.Contains(err.Error(), "local_fallback") {
		t.Fatalf("error should not expose old fake fallback behavior: %v", err)
	}
}

func TestBrightDataAPIRequiresUnlockerZoneForGenericScrape(t *testing.T) {
	scraper := NewBrightDataAPI("api_key", "", "", time.Second, 1, nil, logging.Nop())

	_, err := scraper.GenericScrape(context.Background(), "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN")
	if err == nil || !strings.Contains(err.Error(), "BRIGHTDATA_UNLOCKER_ZONE") {
		t.Fatalf("expected missing zone error, got %v", err)
	}
}

func TestBrightDataAPIRejectsMissingPrice(t *testing.T) {
	_, err := decodeProductResult([]byte(`{"name":"PREDATOR PRO Fold-Over Tongue Turf Football Shoes","currency":"INR"}`), "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN", "brightdata_generic_api")
	if err == nil || !strings.Contains(err.Error(), "valid product price") {
		t.Fatalf("expected missing price validation error, got %v", err)
	}
}

func TestBrightDataAPIReportsBrightDataErrorPayload(t *testing.T) {
	_, err := decodeProductResult([]byte(`{"status_code":502,"headers":{"x-brd-error":"response status was rejected: 403 status code"}}`), "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN", "brightdata_generic_api")
	if err == nil || !strings.Contains(err.Error(), "Bright Data scrape returned status 502") || !strings.Contains(err.Error(), "403 status code") {
		t.Fatalf("expected Bright Data payload error, got %v", err)
	}
}

func TestBrightDataAPIReportsNonProductJSONClearly(t *testing.T) {
	_, err := decodeProductResult([]byte(`{"status":"running","message":"Scraping https://www.amazon.in/dp/B0G4MLD5N8"}`), "https://www.amazon.in/dp/B0G4MLD5N8", "IN", "brightdata_generic_api")
	if err == nil || !strings.Contains(err.Error(), "does not contain product fields") {
		t.Fatalf("expected non-product JSON error, got %v", err)
	}
}

func TestBrightDataAPIParsesUnlockerProductEnvelope(t *testing.T) {
	var requestBody struct {
		Zone    string `json:"zone"`
		URL     string `json:"url"`
		Format  string `json:"format"`
		Country string `json:"country"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/request" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer api_key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status_code":200,"body":"{\"name\":\"PREDATOR PRO Fold-Over Tongue Turf Football Shoes\",\"price\":13999,\"currency\":\"INR\",\"availability\":\"in_stock\"}"}`))
	}))
	defer server.Close()

	scraper := NewBrightDataAPI("api_key", "cli_unlocker", server.URL, time.Second, 1, nil, logging.Nop())
	result, err := scraper.GenericScrape(context.Background(), "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN")
	if err != nil {
		t.Fatal(err)
	}
	if result.Method != "brightdata_generic_api" || result.CurrentPrice != 13999 || result.Currency != "INR" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if requestBody.Zone != "cli_unlocker" || requestBody.Format != "json" || requestBody.Country != "in" {
		t.Fatalf("unexpected unlocker request: %+v", requestBody)
	}
}

func TestBrightDataUsesUnlockerWhenDatasetIsNotSeeded(t *testing.T) {
	var requestBody struct {
		Zone string `json:"zone"`
		URL  string `json:"url"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/request" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status_code":200,"body":"{\"title\":\"Marvel Spiderman Collectible\",\"price\":1499,\"currency\":\"INR\",\"availability\":\"In Stock\",\"main_image\":\"https://example.com/spiderman.jpg\",\"url\":\"https://www.amazon.in/dp/B0G4MLD5N8\"}"}`))
	}))
	defer server.Close()

	scraper := NewBrightDataAPI("api_key", "cli_unlocker", server.URL, time.Second, 1, nil, logging.Nop())
	result, err := scraper.GenericScrape(context.Background(), "https://www.amazon.in/Marvel-Spiderman-Collectible-Articulated-Superheroes/dp/B0G4MLD5N8", "IN")
	if err != nil {
		t.Fatal(err)
	}
	if requestBody.Zone != "cli_unlocker" {
		t.Fatalf("unexpected unlocker request: %+v", requestBody)
	}
	if result.Name != "Marvel Spiderman Collectible" || result.CurrentPrice != 1499 || result.Method != "brightdata_generic_api" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestBrightDataParsesGraphWrappedProductJSONLD(t *testing.T) {
	html := []byte(`<!doctype html>
<html>
	<head><title>Stridex Market</title></head>
	<body>
		<script type="application/ld+json">
			{
				"@context": "http://schema.org/",
				"@graph": [
					{"@type": "Organization", "name": "Stridex Market"},
					{"@type": "BreadcrumbList", "itemListElement": [{"@type": "ListItem", "name": "shoes"}]},
					{
						"@type": "Product",
						"name": "Trailblaze Trail Sneaker",
						"image": [
							"https://cdn.dummyjson.com/product-images/mens-shoes/sports-sneakers-off-white-red/1.webp",
							"https://cdn.dummyjson.com/product-images/mens-shoes/sports-sneakers-off-white-red/2.webp"
						],
						"offers": {
							"@type": "Offer",
							"priceCurrency": "INR",
							"price": 6499,
							"availability": "InStock"
						}
					}
				]
			}
		</script>
	</body>
</html>`)

	result, err := decodeProductResult(html, "https://demo-store-lq1o.onrender.com/product/trailblaze-hiking-boot", "IN", "brightdata_generic_api")
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "Trailblaze Trail Sneaker" || result.CurrentPrice != 6499 || result.Currency != "INR" || result.Availability != "in_stock" {
		t.Fatalf("unexpected graph product result: %+v", result)
	}
	if result.ImageURL == "" {
		t.Fatalf("expected graph product image, got %+v", result)
	}
}

func TestBrightDataUsesDatasetAPIWhenSeeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/datasets/v3/scrape" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("dataset_id"); got != "gd_adidas" {
			t.Fatalf("unexpected dataset id: %s", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer api_key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"product_name":"PREDATOR PRO Fold-Over Tongue Turf Football Shoes","final_price":"13,999","currency_code":"INR","stock_status":"in_stock"}]`))
	}))
	defer server.Close()

	scraper := NewBrightDataAPI("api_key", "", server.URL, time.Second, 1, map[string]string{"adidas.co.in": "gd_adidas"}, logging.Nop())
	result, err := scraper.GenericScrape(context.Background(), "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN")
	if err != nil {
		t.Fatal(err)
	}
	if result.Method != "brightdata_dataset" || result.CurrentPrice != 13999 || result.Currency != "INR" {
		t.Fatalf("unexpected dataset result: %+v", result)
	}
}

func TestBrightDataRunsCollectorThroughScraperStudioAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer api_key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		switch r.URL.Path {
		case "/dca/trigger":
			if got := r.URL.Query().Get("collector"); got != "c_adidas" {
				t.Fatalf("unexpected collector id: %s", got)
			}
			_, _ = w.Write([]byte(`{"collection_id":"j_123"}`))
		case "/dca/dataset":
			_, _ = w.Write([]byte(`[{"name":"PREDATOR PRO Fold-Over Tongue Turf Football Shoes","price":13999,"currency":"INR","availability":"in_stock"}]`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	scraper := NewBrightDataAPI("api_key", "", server.URL, time.Second, 1, nil, logging.Nop())
	result, err := scraper.RunCollector(context.Background(), "c_adidas", "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN")
	if err != nil {
		t.Fatal(err)
	}
	if result.Method != "brightdata_collector_api" || result.CurrentPrice != 13999 {
		t.Fatalf("unexpected collector result: %+v", result)
	}
}

func TestBrightDataCreatesCollectorThroughAIFlowAPI(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if got := r.Header.Get("Authorization"); got != "Bearer api_key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		switch r.URL.Path {
		case "/dca/collector":
			var body struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Name != "cc-adidas-co-in-default" {
				t.Fatalf("unexpected collector name: %s", body.Name)
			}
			_, _ = w.Write([]byte(`{"id":"c_adidas"}`))
		case "/dca/collectors/c_adidas/automate_template":
			_, _ = w.Write([]byte(`{"id":"ai_123","queued":false}`))
		case "/dca/collectors/c_adidas/automate_template/progress":
			_, _ = w.Write([]byte(`{"status":"done"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	scraper := NewBrightDataAPI("api_key", "", server.URL, time.Second, 1, nil, logging.Nop())
	collectorID, err := scraper.CreateCollector(context.Background(), "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN", "adidas.co.in", "default")
	if err != nil {
		t.Fatal(err)
	}
	if collectorID != "c_adidas" {
		t.Fatalf("unexpected collector id: %s", collectorID)
	}
	if strings.Join(calls, ",") != "POST /dca/collector,POST /dca/collectors/c_adidas/automate_template,GET /dca/collectors/c_adidas/automate_template/progress" {
		t.Fatalf("unexpected calls: %v", calls)
	}
}

func TestBrightDataHealsCollectorThroughAPIAndAutoApproves(t *testing.T) {
	var calls []string
	var progressCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if got := r.Header.Get("Authorization"); got != "Bearer api_key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		switch r.URL.Path {
		case "/dca/collectors/c_adidas/refactor_template":
			_, _ = w.Write([]byte(`{"id":"heal_123"}`))
		case "/dca/collectors/c_adidas/refactor_template/progress":
			progressCalls++
			if progressCalls == 1 {
				_, _ = w.Write([]byte(`{"status":"pending_answer","step":"user_approval"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"done"}`))
		case "/dca/collectors/c_adidas/resume_automation_job":
			var body struct {
				Message  bool `json:"message"`
				AutoSave bool `json:"auto_save"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if !body.Message || !body.AutoSave {
				t.Fatalf("expected message and auto_save body, got %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"running"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	scraper := NewBrightDataAPI("api_key", "", server.URL, time.Second, 1, nil, logging.Nop())
	err := scraper.HealCollector(context.Background(), "c_adidas", "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN", DataQualityError{Err: http.ErrBodyNotAllowed})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != "POST /dca/collectors/c_adidas/refactor_template,GET /dca/collectors/c_adidas/refactor_template/progress,POST /dca/collectors/c_adidas/resume_automation_job,GET /dca/collectors/c_adidas/refactor_template/progress" {
		t.Fatalf("unexpected calls: %v", calls)
	}
}

func TestBrightDataParsesCollectorOutputWithObjectPriceAndBooleanAvailability(t *testing.T) {
	result, err := decodeProductResult([]byte(`[{
		"name":"PREDATOR PRO Fold-Over Tongue Turf Football Shoes",
		"price":{"value":13999,"currency":"INR"},
		"availability":true
	}]`), "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN", "brightdata_collector_api")
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentPrice != 13999 || result.Currency != "INR" || result.Availability != "in_stock" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestBrightDataParsesSchemaProductJSONLD(t *testing.T) {
	result, err := decodeProductResult([]byte(`{
		"@context":"http://schema.org/",
		"@type":"Product",
		"name":"Rainbow Prism Diamond Stud Earrings",
		"image":["https://www.tanishq.co.in/on/demandware.static/-/Sites-Tanishq-product-catalog/default/dwfecceec2/images/hi-res/50D5S2SLJADA09_1.jpg"],
		"offers":{
			"@type":"Offer",
			"priceCurrency":"INR",
			"price":47856,
			"availability":"http://schema.org/InStock"
		}
	}`), "https://www.tanishq.co.in/product/rainbow-prism-diamond-stud-earrings-50d5s2sljada09.html?lang=en_IN", "IN", "brightdata_generic_api")
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "Rainbow Prism Diamond Stud Earrings" || result.CurrentPrice != 47856 || result.Currency != "INR" || result.Availability != "in_stock" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !strings.Contains(result.ImageURL, "50D5S2SLJADA09_1.jpg") {
		t.Fatalf("expected Tanishq image URL, got %q", result.ImageURL)
	}
}

func TestBrightDataParsesUnlockerHTMLProductJSONLD(t *testing.T) {
	htmlBody := `<html><head><script type="application/ld+json">{
		"@context":"http://schema.org/",
		"@type":"Product",
		"name":"Rainbow Prism Diamond Stud Earrings",
		"image":["https://www.tanishq.co.in/on/demandware.static/-/Sites-Tanishq-product-catalog/default/dwfecceec2/images/hi-res/50D5S2SLJADA09_1.jpg"],
		"offers":{"@type":"Offer","priceCurrency":"INR","price":47856,"availability":"http://schema.org/InStock"}
	}</script></head><body>Rainbow Prism Diamond Stud Earrings</body></html>`
	envelope, err := json.Marshal(map[string]any{
		"status_code": 200,
		"body":        htmlBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := decodeProductResult(envelope, "https://www.tanishq.co.in/product/rainbow-prism-diamond-stud-earrings-50d5s2sljada09.html?lang=en_IN", "IN", "brightdata_generic_api")
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "Rainbow Prism Diamond Stud Earrings" || result.CurrentPrice != 47856 || result.Currency != "INR" || result.Availability != "in_stock" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestBrightDataParsesFirstCurrencyPriceFromLongText(t *testing.T) {
	result, err := decodeProductResult([]byte(`{
		"name":"Rainbow Prism Diamond Stud Earrings",
		"price":"INR 47 856 Incl. taxes and charges INR 49 842 20% off SKU ID : 50D5S2SLJADA092JA000057",
		"currency":"INR"
	}`), "https://www.tanishq.co.in/product/rainbow-prism-diamond-stud-earrings-50d5s2sljada09.html?lang=en_IN", "IN", "brightdata_generic_api")
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentPrice != 47856 {
		t.Fatalf("expected first visible price, got %+v", result)
	}
}

func TestBrightDataDoesNotParseSKUAsPrice(t *testing.T) {
	_, err := decodeProductResult([]byte(`{
		"name":"Rainbow Prism Diamond Stud Earrings",
		"price":"SKU ID : 50D5S2SLJADA092JA000057",
		"currency":"INR"
	}`), "https://www.tanishq.co.in/product/rainbow-prism-diamond-stud-earrings-50d5s2sljada09.html?lang=en_IN", "IN", "brightdata_generic_api")
	if err == nil || !strings.Contains(err.Error(), "valid product price") {
		t.Fatalf("expected invalid SKU price, got %v", err)
	}
}

func TestBrightDataRejectsUnsupportedLargePrice(t *testing.T) {
	_, err := decodeProductResult([]byte(`{
		"name":"Rainbow Prism Diamond Stud Earrings",
		"current_price":{"value":478564785649842,"currency":"INR"},
		"currency":"INR"
	}`), "https://www.tanishq.co.in/product/rainbow-prism-diamond-stud-earrings-50d5s2sljada09.html?lang=en_IN", "IN", "brightdata_collector_api")
	if err == nil || !IsDataQualityError(err) || !strings.Contains(err.Error(), "exceeds supported product price") {
		t.Fatalf("expected unsupported price data-quality error, got %v", err)
	}
}

func TestBrightDataDiscoversActiveCollectors(t *testing.T) {
	var searches []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dca/collectors_list" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		searches = append(searches, r.URL.Query().Get("search"))
		switch r.URL.Query().Get("search") {
		case "adidas.co.in":
			_, _ = w.Write([]byte(`{"data":[{"id":"c_inactive","name":"adidas.co.in old","active":false},{"id":"c_other","name":"example.com","active":true}]}`))
		case "adidas-co-in":
			_, _ = w.Write([]byte(`{"data":[{"id":"c_adidas","name":"cc-adidas-co-in-default","active":true}]}`))
		case "adidas":
			_, _ = w.Write([]byte(`{"data":[{"id":"c_adidas","name":"cc-adidas-co-in-default","active":true},{"id":"c_other_adidas","name":"adidas unrelated export","active":true}]}`))
		default:
			t.Fatalf("unexpected search: %s", r.URL.Query().Get("search"))
		}
	}))
	defer server.Close()

	scraper := NewBrightDataAPI("api_key", "", server.URL, time.Second, 1, nil, logging.Nop())
	collectors, err := scraper.DiscoverCollectors(context.Background(), "adidas.co.in")
	if err != nil {
		t.Fatal(err)
	}
	if len(collectors) != 1 || collectors[0].ExternalID != "c_adidas" {
		t.Fatalf("unexpected collectors: %+v", collectors)
	}
	if strings.Join(searches, ",") != "adidas.co.in,adidas-co-in,adidas" {
		t.Fatalf("unexpected searches: %v", searches)
	}
}
