alter table domain_scraper_profiles
  add column if not exists request_count integer not null default 0,
  add column if not exists product_count integer not null default 0;

create table if not exists domain_scraper_products (
  profile_id text not null references domain_scraper_profiles(id) on delete cascade,
  url text not null,
  first_seen_at timestamptz not null default now(),
  last_seen_at timestamptz not null default now(),
  primary key (profile_id, url)
);

create table if not exists domain_scraper_collectors (
  id text primary key,
  profile_id text not null references domain_scraper_profiles(id) on delete cascade,
  provider text not null,
  external_collector_id text,
  status text not null,
  purpose text not null,
  url_pattern text,
  request_count integer not null default 0,
  success_count integer not null default 0,
  failure_count integer not null default 0,
  consecutive_structural_failures integer not null default 0,
  last_error text,
  active_since timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

insert into domain_scraper_collectors
(id, profile_id, provider, external_collector_id, status, purpose, active_since, created_at, updated_at)
select
  'dsc_' || replace(gen_random_uuid()::text, '-', ''),
  id,
  'brightdata',
  collector_id,
  'active',
  'default',
  coalesce(active_since, now()),
  now(),
  now()
from domain_scraper_profiles
where collector_id is not null
  and not exists (
    select 1
    from domain_scraper_collectors c
    where c.provider = 'brightdata'
      and c.external_collector_id = domain_scraper_profiles.collector_id
  );

create index if not exists domain_scraper_collectors_profile_status_idx on domain_scraper_collectors (profile_id, provider, status);
create unique index if not exists domain_scraper_collectors_provider_external_idx on domain_scraper_collectors (provider, external_collector_id) where external_collector_id is not null;
create unique index if not exists domain_scraper_collectors_one_provisioning_idx on domain_scraper_collectors (profile_id, provider) where status = 'provisioning';
