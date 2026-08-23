# demo-store

A standalone fake e-commerce site used to demo `clever-consumer`'s scraper on stage: its
product pages visibly change layout (selectors, DOM depth, sibling order, visible text,
gallery/description structure, and where the `application/ld+json` block sits) every few
loads, while the underlying schema.org product data stays valid throughout — proving the
scraper's extraction isn't brittle to DOM/selector churn.

This is a fully independent Next.js project, not part of the main `clever-consumer` app
or its `docker-compose.yml` — see the root `README.md`'s "Deployment" section for how it
fits into the overall setup.

## Product images

Every product uses real photography from two free, no-API-key catalogs (not generic
placeholder scenery), picked to actually match what each product claims to be:
[DummyJSON](https://dummyjson.com/products) for shoes, mobile accessories, watches,
shirts, and bags, and [Fake Store API](https://fakestoreapi.com) for the handful of
apparel items DummyJSON doesn't cover. See the comment at the top of
`src/data/products.ts` for details, including the small number of product names that
were adjusted so the name/description/photo triplet stays honest.

## Why it always parses

The real scraper (`backend/internal/scraping/brightdata.go`) only cares about two things:
an HTML page's `<script type="application/ld+json">` schema.org `Product`/`Offer` blocks,
or specific JSON field names. It ignores CSS classes, DOM depth, element order, and
visible text entirely. Every rotation on this site keeps the JSON-LD valid — only the
DOM/visual layout around it changes.

## Local setup

```
cp .env.example .env.local   # set DATABASE_URL to the same Neon instance the main app uses
psql "$DATABASE_URL" -f db/schema.sql   # once, adds the demo_store schema/table
pnpm install
pnpm dev
```

Visit `http://localhost:3000`. Reload `/product/<slug>` a few times (past `ROTATE_EVERY_N`,
default 4) to see the layout and JSON-LD placement rotate.

## Live demo controls

Rotation is **global and DB-backed** (same Neon Postgres as the main app, its own
`demo_store` schema) — it advances on every real request, including Bright Data's own
fetches, not per-browser. Three ways to control it during a live demo:

- **"Change layout →" button**, inline on every page (home/search/product), next to each
  scenario track's status note. Click it to persistently advance that track to its next
  variant right now — no need to reload the page `ROTATE_EVERY_N` times. This is a real,
  sticky change to the shared counter (`POST /api/advance`), not a one-off preview: it's
  what the next viewer, and Bright Data's next fetch, will see too.
- **`/admin`** — the same "Next" button per track, plus numbered buttons (`[0]`, `[1]`, …)
  to jump straight to an exact variant (`POST /api/set-variant`), also persistent. Gated
  by `ADMIN_TOKEN` (`?token=...`) when one is configured.
- **`?demoVariant=<track>:<index>`** query param — a one-request-only preview that does
  *not* touch the shared counter (natural rotation resumes on the next unparameterized
  load). Handy for a quick look without disturbing what everyone else sees.

A "reset all counters" button on `/admin` zeroes every track back to its first variant.

## Deploy

Render Web Service (Node runtime): build `pnpm install && pnpm build`, start `pnpm start`.
See `render.yaml` and the root README's deployment section for the full env var list.
