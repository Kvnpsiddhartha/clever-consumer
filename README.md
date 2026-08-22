# Clever Consumer

Production-minded MVP for tracking product prices from public product URLs.

## What Is Implemented

- Go backend under `backend/` with the requested `cmd/main/main.go`, HTTP controllers, service layer, Postgres datasource, scheduler worker, Bright Data CLI wrapper, and email boundary.
- PostgreSQL schema for users, sessions, previews, trackers, rules, observations, durable jobs, email outbox, scraper profiles, and healing attempts.
- React/TypeScript web app under `web/clever-consumer/` with magic-link auth, profile settings, URL preview, tracker creation, dashboard actions, and observation history.
- Chrome MV3 popup under `extension/chrome/` that captures the active tab URL and creates trackers through the backend API.
- OpenAPI draft at `docs/openapi.yaml`.

## Local Setup

Start Postgres and Mailpit:

```sh
docker compose up -d postgres mailpit
```

Run the backend:

```sh
cd backend
DATABASE_URL='postgres://clever:clever@localhost:55432/clever_consumer?sslmode=disable' go run ./cmd/main
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

The backend calls `brightdata scrape <url> --format json` through a typed argument array. If the CLI is not installed locally, it returns a low-confidence local fallback preview so the rest of the app can be exercised. Replace this fallback with strict provider failure once Bright Data credentials and CLI setup are finalized.

## Verification

```sh
cd backend && go test ./...
cd web/clever-consumer && npm run typecheck && npm run build
cd extension/chrome && npm run typecheck && npm run build
```
