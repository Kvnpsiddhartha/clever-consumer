-- Shared-state rotation counters for the demo storefront. Lives in its own schema on the
-- SAME Neon Postgres instance as the main clever-consumer app's database, so it can't
-- collide with that app's tables (see backend/migrations/). Apply once:
--
--   psql "$DATABASE_URL" -f demo-store/db/schema.sql
--
-- One row per scenario track (see src/lib/rotation.ts for the list of track keys). The
-- counter is global and site-wide -- it advances on every real request to that page,
-- including a live presenter's reloads and Bright Data's own fetches -- not per-visitor.

CREATE SCHEMA IF NOT EXISTS demo_store;

CREATE TABLE IF NOT EXISTS demo_store.rotation_counters (
    track_key  TEXT PRIMARY KEY,
    counter    BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
