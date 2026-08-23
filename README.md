# Clever Consumer
## A Provider Agnostic Product Price Tracker
If you are someone who wants to track products and price changes, Clever Consumer is just the right tool for you. 

Clever Consumer is a tool for tracking product prices from public product URLs. It combines a Go API, PostgreSQL-backed tracking state, a React web app, and a Chrome extension so a user can preview a product page, create a tracker, review price observations, and receive alert emails when rules match.

The project is organized around three local surfaces:

- Backend API: Go service on `http://localhost:18080`.
- Web app: Vite React app on `http://localhost:5173`.
- Chrome extension: MV3 popup built from `extension/chrome/`.

Local Docker services also include Postgres on `localhost:55432` and Mailpit at `http://localhost:8025` for inspecting email.

A fourth, fully independent piece lives in `demo-store/`: a fake e-commerce site used to demo the scraper live — its product pages rotate DOM/selector/layout structure every few loads while keeping the underlying product data valid, to show the scraper isn't brittle to layout changes. See `demo-store/README.md`.

## What Is Implemented

- Go backend under `backend/` with the requested `cmd/main/main.go`, HTTP controllers, service layer, Postgres datasource, scheduler worker, Bright Data API provider, and email boundary.
- PostgreSQL schema for users, sessions, previews, trackers, rules, observations, durable jobs, email outbox, scraper profiles, and healing attempts.
- React/TypeScript web app under `web/clever-consumer/` with magic-link auth, profile settings, URL preview, tracker creation, dashboard actions, and observation history.
- Chrome MV3 popup under `extension/chrome/` that captures the active tab URL and creates trackers through the backend API.
- OpenAPI draft at `docs/openapi.yaml`.

## Run Locally

Prerequisites:

- Docker with Docker Compose.
- `pnpm` for the web app and Chrome extension.
- Go only if you plan to run backend tests outside Docker.

### 1. Start Backend Services

Start the Go backend, Postgres, and Mailpit:

```sh
make up
```

Check service status and logs:

```sh
make ps
make backend-logs
```

The backend health endpoint should respond at:

```sh
http://localhost:18080/healthz
```

Mailpit is available at:

```sh
http://localhost:8025
```

Useful local commands:

```sh
make ps
make logs
make backend-logs
make down
```

### 2. Run The Web App

Run the web app:

```sh
cd web/clever-consumer
pnpm install
pnpm dev
```

Open the app at:

```sh
http://localhost:5173
```

### 3. Optional Google Sign-In

Magic-link auth works with the local backend and Mailpit. Google sign-in uses the same backend session cookie as magic links. Configure a Google OAuth client with this local redirect URI:

```sh
http://localhost:18080/v1/auth/google/callback
```

Set these environment variables in your shell before starting or restarting the backend:

```sh
export GOOGLE_CLIENT_ID='<your-client-id>'
export GOOGLE_CLIENT_SECRET='<your-client-secret>'
export GOOGLE_REDIRECT_URL='http://localhost:18080/v1/auth/google/callback'
```

Apply the new values to Docker Compose:

```sh
make restart
```

### 4. Optional Chrome Extension

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

When no dataset or discovered collector matches, the backend calls the Bright Data Unlocker API directly:

```sh
POST /request
```

It does not create fallback prices. If authentication fails, the Unlocker zone is missing, or the scrape response lacks required product fields, preview creation fails with an explicit error.

The backend also maintains provider-agnostic domain scraper profiles. Each domain can have multiple collectors in `domain_scraper_collectors`. The first request for a new domain returns from the generic scrape path, while the backend provisions one collector in the background with:

```sh
POST /dca/collector
POST /dca/collectors/<collector-id>/automate_template
GET /dca/collectors/<collector-id>/automate_template/progress
```

Later requests route to a healthy collector for that domain once provisioning completes, using the Scraper Studio API:

```sh
POST /dca/trigger?collector=<collector-id>&queue_next=1
GET /dca/dataset?id=<collection-id>
```

When domain traffic crosses `SCRAPER_COLLECTOR_REQUEST_THRESHOLD` or distinct product count crosses `SCRAPER_COLLECTOR_PRODUCT_THRESHOLD`, the backend provisions another collector for the same domain, up to `SCRAPER_COLLECTOR_MAX_PER_DOMAIN`. Provisioning is guarded in Postgres so concurrent requests do not create duplicate collectors.

If a collector run succeeds but returns invalid product data, the backend marks that collector as healing, calls the Scraper Studio self-healing API, auto-approves the fix when Bright Data returns an approval gate, then retries the same collector. Authentication, zone, and other command failures do not trigger healing. Other requests can route to another active collector for the domain where one exists.

```sh
POST /dca/collectors/<collector-id>/refactor_template
GET /dca/collectors/<collector-id>/refactor_template/progress
POST /dca/collectors/<collector-id>/resume_automation_job
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

## Deployment

Everything deploys free on **Render**, split into two Render Projects, plus a separately-hosted Postgres (Neon or Supabase both work; Render's own free Postgres is deleted 30 days after creation):

| Piece | Where | Type |
|---|---|---|
| Go backend (`APP_ROLE=all`) | Render, Project `clever-consumer` | Web Service, Docker (`backend/Dockerfile`) |
| React web app (`web/clever-consumer`) | Render, Project `clever-consumer` | Static Site (`pnpm build`, publish `dist`) |
| `demo-store/` | Render, Project `demo-store` (kept separate so redeploys never touch the real app) | Web Service, Node (`pnpm build` / `pnpm start`) |
| Postgres | Neon or Supabase | shared by both the main app and `demo-store`'s own `demo_store` schema |

Blueprints are checked in: `render.yaml` (backend + frontend) and `demo-store/render.yaml` (demo store) — in the Render dashboard use **New → Blueprint** and point at this repo for each.

> **If using Supabase:** use the **connection pooler** string (Project → Connect → Session pooler, `aws-0-<region>.pooler.supabase.com`, username `postgres.<project-ref>`), not the direct `db.<project-ref>.supabase.co` host. The direct host resolves to an IPv6-only address; Render's compute has no IPv6 egress and will loop forever on `dial tcp ...: network is unreachable`, never binding its port. This applies to both the backend and demo-store services.

Setup, once:

```sh
# 1. Create the Postgres database, then apply schema to it (no local psql needed --
#    this works from any machine with Docker):
docker run --rm --network host -v "$(pwd)/backend/migrations:/m:ro" postgres:17 \
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f /m/000001_initial.sql -f /m/000002_domain_scraper_collector_pool.sql
docker run --rm --network host -v "$(pwd)/demo-store/db:/m:ro" postgres:17 \
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f /m/schema.sql
# (--network host is only needed if $DATABASE_URL is the IPv6-only direct Supabase
# host; drop it when applying against the pooler string or a plain IPv4 host like Neon.)

# 2. Deploy clever-consumer/render.yaml as the "clever-consumer" Render Project.
#    Set on the backend service: DATABASE_URL, PUBLIC_BASE_URL (frontend's
#    Render URL, once known), BRIGHTDATA_* (from your local .env), and optionally
#    GOOGLE_CLIENT_ID/SECRET + GOOGLE_REDIRECT_URL (must match a redirect URI
#    registered in Google Cloud Console).
#    Set on the frontend static site: VITE_API_BASE_URL (backend's Render URL) --
#    this is read at BUILD time (Vite inlines it), so set it before the first deploy.

# 3. Deploy demo-store/render.yaml as its own "demo-store" Render Project.
#    Set DATABASE_URL to the same Postgres connection string.
```

Because the backend and frontend end up on different `*.onrender.com` hosts, every
session-authenticated API call from the web app is cross-site. The session cookie
(`cc_session`, set in `backend/internal/http/controllers/api.go`) already accounts for
this: it uses `SameSite=None; Secure` whenever `PUBLIC_BASE_URL` is `https://`, and falls
back to `SameSite=Lax` for local `http://localhost` dev.

Render's free Web Services (backend + demo-store) spin down after 15 minutes idle with a
~1 minute cold start on the next request — hit both URLs once a few minutes before a live
demo. The frontend Static Site never sleeps.

## Verification

```sh
make test
cd web/clever-consumer && pnpm run typecheck && pnpm run build
cd extension/chrome && pnpm run typecheck && pnpm run build
cd demo-store && pnpm run typecheck && pnpm run build
```
