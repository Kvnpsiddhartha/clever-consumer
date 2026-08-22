package scraping

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"clever-consumer/backend/internal/logging"
)

func TestBrightDataCLIDoesNotFallbackWhenCommandFails(t *testing.T) {
	scraper := NewBrightDataCLI("brightdata-command-that-does-not-exist", "", "", "", time.Second, 1, nil, logging.Nop())

	_, err := scraper.GenericScrape(context.Background(), "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN")
	if err == nil {
		t.Fatal("expected scrape to fail when Bright Data CLI is unavailable")
	}
	if strings.Contains(err.Error(), "999") || strings.Contains(err.Error(), "local_fallback") {
		t.Fatalf("error should not expose old fake fallback behavior: %v", err)
	}
}

func TestBrightDataCLIRejectsMissingPrice(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper script uses POSIX shell")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "brightdata")
	script := "#!/bin/sh\nprintf '%s' '{\"name\":\"PREDATOR PRO Fold-Over Tongue Turf Football Shoes\",\"currency\":\"INR\"}'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	scraper := NewBrightDataCLI(bin, "", "", "", time.Second, 1, nil, logging.Nop())

	_, err := scraper.GenericScrape(context.Background(), "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN")
	if err == nil || !strings.Contains(err.Error(), "valid product price") {
		t.Fatalf("expected missing price validation error, got %v", err)
	}
}

func TestBrightDataCLIReportsBrightDataErrorPayload(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper script uses POSIX shell")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "brightdata")
	script := "#!/bin/sh\nprintf '%s' '{\"status_code\":502,\"headers\":{\"x-brd-error\":\"response status was rejected: 403 status code\"}}'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	scraper := NewBrightDataCLI(bin, "", "", "", time.Second, 1, nil, logging.Nop())

	_, err := scraper.GenericScrape(context.Background(), "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN")
	if err == nil || !strings.Contains(err.Error(), "Bright Data scrape returned status 502") || !strings.Contains(err.Error(), "403 status code") {
		t.Fatalf("expected Bright Data payload error, got %v", err)
	}
}

func TestBrightDataCLIParsesProductResult(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper script uses POSIX shell")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "brightdata")
	script := "#!/bin/sh\nprintf '%s' '{\"name\":\"PREDATOR PRO Fold-Over Tongue Turf Football Shoes\",\"price\":13999,\"currency\":\"INR\",\"availability\":\"in_stock\"}'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	scraper := NewBrightDataCLI(bin, "", "", "", time.Second, 1, nil, logging.Nop())

	result, err := scraper.GenericScrape(context.Background(), "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN")
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "PREDATOR PRO Fold-Over Tongue Turf Football Shoes" || result.CurrentPrice != 13999 || result.Currency != "INR" {
		t.Fatalf("unexpected result: %+v", result)
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

	scraper := NewBrightDataCLI("brightdata-command-that-should-not-run", "api_key", "", server.URL, time.Second, 1, map[string]string{"adidas.co.in": "gd_adidas"}, logging.Nop())
	result, err := scraper.GenericScrape(context.Background(), "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN")
	if err != nil {
		t.Fatal(err)
	}
	if result.Method != "brightdata_dataset" || result.CurrentPrice != 13999 || result.Currency != "INR" {
		t.Fatalf("unexpected dataset result: %+v", result)
	}
}

func TestBrightDataRunsCollectorThroughScraperStudioAPI(t *testing.T) {
	var datasetPolls int
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
			datasetPolls++
			if datasetPolls == 1 {
				_, _ = w.Write([]byte(`{"status":"running"}`))
				return
			}
			_, _ = w.Write([]byte(`[{"name":"PREDATOR PRO Fold-Over Tongue Turf Football Shoes","price":13999,"currency":"INR","availability":"in_stock"}]`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	scraper := NewBrightDataCLI("brightdata-command-that-should-not-run", "api_key", "", server.URL, 10*time.Second, 1, nil, logging.Nop())
	result, err := scraper.RunCollector(context.Background(), "c_adidas", "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN")
	if err != nil {
		t.Fatal(err)
	}
	if result.Method != "brightdata_collector_api" || result.CurrentPrice != 13999 {
		t.Fatalf("unexpected collector result: %+v", result)
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

	scraper := NewBrightDataCLI("brightdata-command-that-should-not-run", "api_key", "", server.URL, time.Second, 1, nil, logging.Nop())
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

func TestBrightDataCLIRunCollectorAndHealUseCollectorCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper script uses POSIX shell")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "brightdata")
	calls := filepath.Join(dir, "calls")
	t.Setenv("CALLS_FILE", calls)

	script := `#!/bin/sh
printf '%s\n' "$*" >> "$CALLS_FILE"
if [ "$1" = "scraper" ] && [ "$2" = "heal" ]; then
	printf '%s' '{"status":"done"}'
	exit 0
fi
if [ "$1" = "scraper" ] && [ "$2" = "run" ]; then
	printf '%s' '{"name":"PREDATOR PRO Fold-Over Tongue Turf Football Shoes","price":13999,"currency":"INR","availability":"in_stock"}'
	exit 0
fi
exit 1
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	scraper := NewBrightDataCLI(bin, "", "", "", time.Second, 1, nil, logging.Nop())

	result, err := scraper.RunCollector(context.Background(), "collector_123", "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN")
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentPrice != 13999 || result.Method != "brightdata_collector" {
		t.Fatalf("unexpected collector result: %+v", result)
	}
	if err := scraper.HealCollector(context.Background(), "collector_123", "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN", DataQualityError{Err: os.ErrInvalid}); err != nil {
		t.Fatal(err)
	}

	callLog, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	callsText := string(callLog)
	if !strings.Contains(callsText, "scraper heal collector_123") {
		t.Fatalf("expected heal call, got:\n%s", callsText)
	}
	if !strings.Contains(callsText, "--auto-approve") || !strings.Contains(callsText, "--max-retries 1") {
		t.Fatalf("expected autonomous heal with configured retries, got:\n%s", callsText)
	}
	if strings.Count(callsText, "scraper run collector_123") != 1 {
		t.Fatalf("expected one scraper run, got:\n%s", callsText)
	}
}

func TestBrightDataCLIRunCollectorParsesJSONAfterStatusOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper script uses POSIX shell")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "brightdata")

	script := `#!/bin/sh
if [ "$1" = "scraper" ] && [ "$2" = "run" ]; then
	printf '%s\n' 'Success: collector run completed'
	printf '%s' '{"name":"PREDATOR PRO Fold-Over Tongue Turf Football Shoes","price":13999,"currency":"INR","availability":"in_stock"}'
	exit 0
fi
exit 1
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	scraper := NewBrightDataCLI(bin, "", "", "", time.Second, 1, nil, logging.Nop())

	result, err := scraper.RunCollector(context.Background(), "collector_123", "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN")
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentPrice != 13999 || result.Method != "brightdata_collector" {
		t.Fatalf("unexpected collector result: %+v", result)
	}
}

func TestBrightDataCLICreateCollectorParsesCollectorID(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper script uses POSIX shell")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "brightdata")
	calls := filepath.Join(dir, "calls")
	t.Setenv("CALLS_FILE", calls)

	script := `#!/bin/sh
printf '%s\n' "$*" >> "$CALLS_FILE"
if [ "$1" = "scraper" ] && [ "$2" = "create" ]; then
	printf '%s' '{"collector_id":"collector_456","status":"done"}'
	exit 0
fi
exit 1
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	scraper := NewBrightDataCLI(bin, "", "", "", time.Second, 1, nil, logging.Nop())

	collectorID, err := scraper.CreateCollector(context.Background(), "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN", "adidas.co.in", "default")
	if err != nil {
		t.Fatal(err)
	}
	if collectorID != "collector_456" {
		t.Fatalf("unexpected collector id: %s", collectorID)
	}

	callLog, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	callsText := string(callLog)
	if !strings.Contains(callsText, "scraper create") || !strings.Contains(callsText, "--name cc-adidas-co-in-default") {
		t.Fatalf("expected named scraper create command, got:\n%s", callsText)
	}
}

func TestBrightDataCLICreateCollectorParsesJSONAfterStatusOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test helper script uses POSIX shell")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "brightdata")

	script := `#!/bin/sh
if [ "$1" = "scraper" ] && [ "$2" = "create" ]; then
	printf '%s\n' 'Success: scraper collector created'
	printf '%s' '{"collector_id":"collector_456","status":"done"}'
	exit 0
fi
exit 1
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	scraper := NewBrightDataCLI(bin, "", "", "", time.Second, 1, nil, logging.Nop())

	collectorID, err := scraper.CreateCollector(context.Background(), "https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html", "IN", "adidas.co.in", "default")
	if err != nil {
		t.Fatal(err)
	}
	if collectorID != "collector_456" {
		t.Fatalf("unexpected collector id: %s", collectorID)
	}
}
