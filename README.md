# Clever Consumer

Production-minded MVP for tracking product prices from public product URLs.

## What Is Implemented

- Go backend under `backend/` with the requested `cmd/main/main.go`, HTTP controllers, service layer, Postgres datasource, scheduler worker, Bright Data CLI wrapper, and email boundary.
- PostgreSQL schema for users, sessions, previews, trackers, rules, observations, durable jobs, email outbox, scraper profiles, and healing attempts.
- React/TypeScript web app under `web/clever-consumer/` with magic-link auth, profile settings, URL preview, tracker creation, dashboard actions, and observation history.
- Chrome MV3 popup under `extension/chrome/` that captures the active tab URL and creates trackers through the backend API.
- OpenAPI draft at `docs/openapi.yaml`.

## Local Setup

Start the backend, Postgres, and Mailpit:

```sh
make up
```

Useful local commands:

```sh
make ps
make logs
make backend-logs
make down
```

Run the web app:

```sh
cd web/clever-consumer
pnpm install
pnpm dev
```

Build the Chrome extension:

```sh
cd extension/chrome
pnpm install
pnpm build
```

Load `extension/chrome/dist` as an unpacked extension in Chrome.

## Current Bright Data Behavior

By default, the backend discovers existing Bright Data Scraper Studio collectors through the Bright Data API, stores matched collectors in Postgres, and runs `c_...` collectors through the Scraper Studio API. Configure pre-built Scrapers Library datasets with:

```sh
BRIGHTDATA_DATASET_SEEDS=adidas.co.in:gd_xxxxxxxxxxxxxxxx,amazon.in:gd_yyyyyyyyyyyyyyyy
```

When no dataset or discovered collector matches, the backend calls Bright Data through a typed argument array:

```sh
brightdata scrape <url> --format json --country <country>
```

It does not create fallback prices. If the CLI is missing, authentication fails, or the scrape response lacks required product fields, preview creation fails with an explicit error.

The backend also maintains provider-agnostic domain scraper profiles. Each domain can have multiple collectors in `domain_scraper_collectors`. The first request for a new domain returns from the generic scrape path, while the backend provisions one collector in the background with:

```sh
brightdata scraper create <url> "<product extraction description>" --json
```

Later requests route to a healthy collector for that domain once provisioning completes, using the Scraper Studio API when the collector id starts with `c_`:

```sh
brightdata scraper run <collector-id> <url> --json
```

When domain traffic crosses `SCRAPER_COLLECTOR_REQUEST_THRESHOLD` or distinct product count crosses `SCRAPER_COLLECTOR_PRODUCT_THRESHOLD`, the backend provisions another collector for the same domain, up to `SCRAPER_COLLECTOR_MAX_PER_DOMAIN`. Provisioning is guarded in Postgres so concurrent requests do not create duplicate collectors.

If a collector run succeeds but returns invalid product data, the backend marks that collector as healing, calls `brightdata scraper heal` with `--auto-approve`, then retries the same collector. Authentication, missing CLI, zone, and other command failures do not trigger healing. Other requests can route to another active collector for the domain where one exists.

Install and authenticate the CLI:

```sh
npm install -g @brightdata/cli
brightdata login
brightdata config
brightdata budget
brightdata scrape https://www.adidas.co.in/predator-pro-fold-over-tongue-turf-football-shoes/JR7866.html --format json --country in
```

For non-interactive local runs, set these environment variables before starting the backend:

```sh
export BRIGHTDATA_API_KEY='<your-api-key>'
export BRIGHTDATA_UNLOCKER_ZONE='cli_unlocker'
export BRIGHTDATA_HEAL_RETRIES=1
export SCRAPER_COLLECTOR_REQUEST_THRESHOLD=500
export SCRAPER_COLLECTOR_PRODUCT_THRESHOLD=100
export SCRAPER_COLLECTOR_MAX_PER_DOMAIN=3
```

When running through Docker Compose, these variables can be exported in your shell before `make up` or placed in a local `.env` file.

## Verification

```sh
make test
cd web/clever-consumer && npm run typecheck && npm run build
cd extension/chrome && npm run typecheck && npm run build
```
