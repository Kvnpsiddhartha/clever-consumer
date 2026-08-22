create extension if not exists pgcrypto;

create table if not exists users (
  id text primary key,
  email text not null unique,
  country text not null default 'IN',
  timezone text not null default 'Asia/Kolkata',
  created_at timestamptz not null default now()
);

create table if not exists magic_link_tokens (
  id text primary key,
  email text not null,
  token_hash text not null unique,
  expires_at timestamptz not null,
  used_at timestamptz,
  created_at timestamptz not null default now()
);

create table if not exists sessions (
  id text primary key,
  user_id text not null references users(id) on delete cascade,
  expires_at timestamptz not null,
  revoked_at timestamptz,
  created_at timestamptz not null default now()
);

create table if not exists extension_refresh_tokens (
  id text primary key,
  user_id text not null references users(id) on delete cascade,
  token_hash text not null unique,
  expires_at timestamptz not null,
  revoked_at timestamptz,
  created_at timestamptz not null default now()
);

create table if not exists product_previews (
  id text primary key,
  user_id text not null references users(id) on delete cascade,
  url text not null,
  status text not null,
  name text not null,
  image_url text not null default '',
  current_price numeric(12,2) not null,
  currency text not null,
  availability text not null,
  country text not null,
  confidence numeric(4,3) not null,
  error text,
  confirmed_at timestamptz,
  expires_at timestamptz not null,
  created_at timestamptz not null,
  updated_at timestamptz not null
);

create table if not exists trackers (
  id text primary key,
  user_id text not null references users(id) on delete cascade,
  preview_id text not null references product_previews(id),
  product_url text not null,
  name text not null,
  image_url text not null default '',
  currency text not null,
  country text not null,
  interval_hours integer not null check (interval_hours between 1 and 168),
  status text not null,
  current_price numeric(12,2) not null,
  availability text not null,
  last_checked_at timestamptz,
  next_check_at timestamptz not null,
  created_at timestamptz not null,
  updated_at timestamptz not null
);

create table if not exists alert_rules (
  id text primary key,
  tracker_id text not null references trackers(id) on delete cascade,
  type text not null,
  threshold_price numeric(12,2),
  enabled boolean not null default true,
  created_at timestamptz not null default now()
);

create table if not exists observations (
  id text primary key,
  tracker_id text not null references trackers(id) on delete cascade,
  price numeric(12,2) not null,
  currency text not null,
  availability text not null,
  method text not null,
  confidence numeric(4,3) not null,
  observed_at timestamptz not null
);

create table if not exists scrape_runs (
  id text primary key,
  tracker_id text references trackers(id) on delete set null,
  url text not null,
  country text not null,
  method text not null,
  status text not null,
  error text,
  started_at timestamptz not null default now(),
  finished_at timestamptz
);

create table if not exists domain_scraper_profiles (
  id text primary key,
  domain text not null unique,
  collector_id text,
  status text not null,
  consecutive_structural_failures integer not null default 0,
  active_since timestamptz,
  updated_at timestamptz not null default now()
);

create table if not exists healing_attempts (
  id text primary key,
  profile_id text not null references domain_scraper_profiles(id) on delete cascade,
  candidate_collector_id text,
  status text not null,
  validation_evidence jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  decided_at timestamptz
);

create table if not exists durable_jobs (
  id text primary key,
  kind text not null,
  idempotency_key text not null unique,
  payload jsonb not null,
  status text not null,
  run_after timestamptz not null,
  attempts integer not null default 0,
  locked_by text,
  locked_until timestamptz,
  last_error text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists email_outbox (
  id text primary key,
  user_id text references users(id) on delete set null,
  dedupe_key text not null unique,
  to_email text not null,
  subject text not null,
  text_body text not null,
  html_body text not null,
  status text not null,
  attempts integer not null default 0,
  next_attempt_at timestamptz not null default now(),
  last_error text,
  created_at timestamptz not null default now(),
  sent_at timestamptz
);

create index if not exists trackers_due_idx on trackers (next_check_at) where status = 'active';
create index if not exists observations_tracker_time_idx on observations (tracker_id, observed_at);
create index if not exists sessions_active_idx on sessions (id, expires_at) where revoked_at is null;
