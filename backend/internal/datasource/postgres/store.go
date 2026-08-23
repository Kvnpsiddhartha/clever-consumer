package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"clever-consumer/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := applyMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	if err := recoverInterruptedCollectorOperations(ctx, pool, time.Now().UTC()); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %s: %w", filepath.Base(file), err)
		}
	}
	return nil
}

func migrationsDir() (string, error) {
	candidates := []string{
		"migrations",
		filepath.Join("backend", "migrations"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("migrations directory not found")
}

func recoverInterruptedCollectorOperations(ctx context.Context, pool *pgxpool.Pool, now time.Time) error {
	if _, err := pool.Exec(ctx, `
		update domain_scraper_collectors
		set last_error = case last_error
			when 'context canceled' then 'collector run was interrupted by backend restart'
			when 'context deadline exceeded' then 'collector run timed out before Bright Data returned a dataset'
			else last_error
		end,
		updated_at = case
			when last_error in ('context canceled', 'context deadline exceeded') then $1
			else updated_at
		end
		where last_error in ('context canceled', 'context deadline exceeded')
	`, now); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `
		update domain_scraper_collectors
		set status = $2,
		    last_error = coalesce(last_error, 'collector healing was interrupted by backend restart'),
		    updated_at = $3
		where status = $1 and external_collector_id is not null
	`, domain.ScraperCollectorHealing, domain.ScraperCollectorActive, now); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, `
		update domain_scraper_collectors
		set status = $2,
		    failure_count = failure_count + 1,
		    last_error = 'collector provisioning was interrupted by backend restart',
		    updated_at = $3
		where status = $1 and external_collector_id is null
	`, domain.ScraperCollectorProvisioning, domain.ScraperCollectorFailed, now)
	return err
}

func (s *Store) CreateMagicLink(ctx context.Context, id, email, tokenHash string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		insert into magic_link_tokens (id, email, token_hash, expires_at)
		values ($1, $2, $3, $4)
	`, id, email, tokenHash, expiresAt)
	return err
}

func (s *Store) ConsumeMagicLink(ctx context.Context, tokenHash string, now time.Time) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var id, email string
	err = tx.QueryRow(ctx, `
		select id, email from magic_link_tokens
		where token_hash = $1 and used_at is null and expires_at > $2
		for update
	`, tokenHash, now).Scan(&id, &email)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `update magic_link_tokens set used_at = $2 where id = $1`, id, now); err != nil {
		return "", err
	}
	return email, tx.Commit(ctx)
}

func (s *Store) UpsertUser(ctx context.Context, email, country, timezone string) (domain.User, error) {
	var user domain.User
	err := s.pool.QueryRow(ctx, `
		insert into users (id, email, country, timezone)
		values ('usr_' || replace(gen_random_uuid()::text, '-', ''), $1, $2, $3)
		on conflict (email) do update set email = excluded.email
		returning id, email, country, timezone, created_at
	`, email, country, timezone).Scan(&user.ID, &user.Email, &user.Country, &user.Timezone, &user.CreatedAt)
	return user, err
}

func (s *Store) CreateSession(ctx context.Context, id, userID string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		insert into sessions (id, user_id, expires_at)
		values ($1, $2, $3)
	`, id, userID, expiresAt)
	return err
}

func (s *Store) UserBySession(ctx context.Context, sessionID string, now time.Time) (domain.User, error) {
	var user domain.User
	err := s.pool.QueryRow(ctx, `
		select u.id, u.email, u.country, u.timezone, u.created_at
		from sessions s
		join users u on u.id = s.user_id
		where s.id = $1 and s.expires_at > $2 and s.revoked_at is null
	`, sessionID, now).Scan(&user.ID, &user.Email, &user.Country, &user.Timezone, &user.CreatedAt)
	return user, err
}

func (s *Store) UpdateProfile(ctx context.Context, userID, country, timezone string) (domain.User, error) {
	var user domain.User
	err := s.pool.QueryRow(ctx, `
		update users set country = $2, timezone = $3
		where id = $1
		returning id, email, country, timezone, created_at
	`, userID, country, timezone).Scan(&user.ID, &user.Email, &user.Country, &user.Timezone, &user.CreatedAt)
	return user, err
}

func (s *Store) CreatePreview(ctx context.Context, preview domain.ProductPreview) error {
	_, err := s.pool.Exec(ctx, `
		insert into product_previews
		(id, user_id, url, status, name, image_url, current_price, currency, availability, country, confidence, error, confirmed_at, expires_at, created_at, updated_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`, preview.ID, preview.UserID, preview.URL, preview.Status, preview.Name, preview.ImageURL, preview.CurrentPrice, preview.Currency, preview.Availability, preview.Country, preview.Confidence, preview.Error, preview.ConfirmedAt, preview.ExpiresAt, preview.CreatedAt, preview.UpdatedAt)
	return err
}

func (s *Store) Preview(ctx context.Context, userID, previewID string) (domain.ProductPreview, error) {
	var preview domain.ProductPreview
	err := s.pool.QueryRow(ctx, `
		select id, user_id, url, status, name, image_url, current_price, currency, availability, country, confidence, coalesce(error, ''), confirmed_at, expires_at, created_at, updated_at
		from product_previews where user_id = $1 and id = $2
	`, userID, previewID).Scan(&preview.ID, &preview.UserID, &preview.URL, &preview.Status, &preview.Name, &preview.ImageURL, &preview.CurrentPrice, &preview.Currency, &preview.Availability, &preview.Country, &preview.Confidence, &preview.Error, &preview.ConfirmedAt, &preview.ExpiresAt, &preview.CreatedAt, &preview.UpdatedAt)
	return preview, err
}

func (s *Store) ConfirmPreview(ctx context.Context, userID, previewID string, at time.Time) (domain.ProductPreview, error) {
	_, err := s.pool.Exec(ctx, `
		update product_previews set status = $3, confirmed_at = $4, updated_at = $4
		where user_id = $1 and id = $2 and expires_at > $4
	`, userID, previewID, domain.PreviewReady, at)
	if err != nil {
		return domain.ProductPreview{}, err
	}
	return s.Preview(ctx, userID, previewID)
}

func (s *Store) CreateTracker(ctx context.Context, tracker domain.Tracker) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		insert into trackers
		(id, user_id, preview_id, product_url, name, image_url, currency, country, interval_hours, status, current_price, availability, next_check_at, created_at, updated_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, tracker.ID, tracker.UserID, tracker.PreviewID, tracker.ProductURL, tracker.Name, tracker.ImageURL, tracker.Currency, tracker.Country, tracker.IntervalHours, tracker.Status, tracker.CurrentPrice, tracker.Availability, tracker.NextCheckAt, tracker.CreatedAt, tracker.UpdatedAt)
	if err != nil {
		return err
	}
	for _, rule := range tracker.Rules {
		_, err = tx.Exec(ctx, `
			insert into alert_rules (id, tracker_id, type, threshold_price, enabled)
			values ($1, $2, $3, $4, $5)
		`, rule.ID, tracker.ID, rule.Type, rule.ThresholdPrice, rule.Enabled)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ListTrackers(ctx context.Context, userID string) ([]domain.Tracker, error) {
	rows, err := s.pool.Query(ctx, `
		select id, user_id, preview_id, product_url, name, image_url, currency, country, interval_hours, status, current_price, availability, last_checked_at, next_check_at, created_at, updated_at
		from trackers
		where user_id = $1
		order by created_at desc
	`, userID)
	if err != nil {
		return nil, err
	}
	trackers, err := pgx.CollectRows(rows, scanTracker)
	if err != nil {
		return nil, err
	}
	return s.attachRules(ctx, trackers)
}

func (s *Store) Tracker(ctx context.Context, userID, trackerID string) (domain.Tracker, error) {
	rows, err := s.pool.Query(ctx, `
		select id, user_id, preview_id, product_url, name, image_url, currency, country, interval_hours, status, current_price, availability, last_checked_at, next_check_at, created_at, updated_at
		from trackers
		where user_id = $1 and id = $2
	`, userID, trackerID)
	if err != nil {
		return domain.Tracker{}, err
	}
	trackers, err := pgx.CollectRows(rows, scanTracker)
	if err != nil {
		return domain.Tracker{}, err
	}
	if len(trackers) == 0 {
		return domain.Tracker{}, errors.New("tracker not found")
	}
	trackers, err = s.attachRules(ctx, trackers)
	if err != nil {
		return domain.Tracker{}, err
	}
	return trackers[0], nil
}

func (s *Store) DeleteTracker(ctx context.Context, userID, trackerID string) error {
	tag, err := s.pool.Exec(ctx, `delete from trackers where user_id = $1 and id = $2`, userID, trackerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("tracker not found")
	}
	return nil
}

func (s *Store) AddObservation(ctx context.Context, obs domain.Observation) error {
	_, err := s.pool.Exec(ctx, `
		insert into observations (id, tracker_id, price, currency, availability, method, confidence, observed_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
	`, obs.ID, obs.TrackerID, obs.Price, obs.Currency, obs.Availability, obs.Method, obs.Confidence, obs.ObservedAt)
	return err
}

func (s *Store) Observations(ctx context.Context, userID, trackerID string) ([]domain.Observation, error) {
	rows, err := s.pool.Query(ctx, `
		select o.id, o.tracker_id, o.price, o.currency, o.availability, o.method, o.confidence, o.observed_at
		from observations o
		join trackers t on t.id = o.tracker_id
		where t.user_id = $1 and t.id = $2
		order by o.observed_at asc
	`, userID, trackerID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Observation, error) {
		var obs domain.Observation
		err := row.Scan(&obs.ID, &obs.TrackerID, &obs.Price, &obs.Currency, &obs.Availability, &obs.Method, &obs.Confidence, &obs.ObservedAt)
		return obs, err
	})
}

func (s *Store) SetTrackerStatus(ctx context.Context, userID, trackerID, status string) error {
	tag, err := s.pool.Exec(ctx, `update trackers set status = $3, updated_at = now() where user_id = $1 and id = $2`, userID, trackerID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("tracker not found")
	}
	return nil
}

func (s *Store) TouchTrackerRun(ctx context.Context, userID, trackerID string, price float64, availability string, observedAt, nextCheckAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		update trackers
		set current_price = $3, availability = $4, last_checked_at = $5, next_check_at = $6, updated_at = now()
		where user_id = $1 and id = $2
	`, userID, trackerID, price, availability, observedAt, nextCheckAt)
	return err
}

func (s *Store) DueTrackers(ctx context.Context, now time.Time, limit int) ([]domain.Tracker, error) {
	rows, err := s.pool.Query(ctx, `
		select id, user_id, preview_id, product_url, name, image_url, currency, country, interval_hours, status, current_price, availability, last_checked_at, next_check_at, created_at, updated_at
		from trackers
		where status = $1 and next_check_at <= $2
		order by next_check_at asc
		limit $3
	`, domain.TrackerActive, now, limit)
	if err != nil {
		return nil, err
	}
	trackers, err := pgx.CollectRows(rows, scanTracker)
	if err != nil {
		return nil, err
	}
	return s.attachRules(ctx, trackers)
}

func (s *Store) PrepareScrapeTarget(ctx context.Context, domainName, targetURL, provider string, now time.Time, thresholds domain.CollectorThresholds) (domain.ScrapeTarget, error) {
	if thresholds.MaxPerDomain <= 0 {
		thresholds.MaxPerDomain = 1
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.ScrapeTarget{}, err
	}
	defer tx.Rollback(ctx)

	var profile domain.ScraperProfile
	err = tx.QueryRow(ctx, `
		insert into domain_scraper_profiles (id, domain, status, request_count, product_count, updated_at)
		values ('dsp_' || replace(gen_random_uuid()::text, '-', ''), $1, $2, 1, 0, $3)
		on conflict (domain) do update
		set request_count = domain_scraper_profiles.request_count + 1,
		    status = $2,
		    updated_at = $3
		returning id, domain, status, request_count, product_count, updated_at
	`, domainName, domain.ScraperProfileActive, now).Scan(&profile.ID, &profile.Domain, &profile.Status, &profile.RequestCount, &profile.ProductCount, &profile.UpdatedAt)
	if err != nil {
		return domain.ScrapeTarget{}, err
	}

	tag, err := tx.Exec(ctx, `
		insert into domain_scraper_products (profile_id, url, first_seen_at, last_seen_at)
		values ($1, $2, $3, $3)
		on conflict (profile_id, url) do nothing
	`, profile.ID, targetURL, now)
	if err != nil {
		return domain.ScrapeTarget{}, err
	}
	if tag.RowsAffected() == 1 {
		err = tx.QueryRow(ctx, `
			update domain_scraper_profiles
			set product_count = product_count + 1, updated_at = $2
			where id = $1
			returning id, domain, status, request_count, product_count, updated_at
		`, profile.ID, now).Scan(&profile.ID, &profile.Domain, &profile.Status, &profile.RequestCount, &profile.ProductCount, &profile.UpdatedAt)
		if err != nil {
			return domain.ScrapeTarget{}, err
		}
	}

	var activeCount, totalCount, provisioningCount int
	err = tx.QueryRow(ctx, `
		select
			count(*) filter (where status = $3 and external_collector_id is not null),
			count(*) filter (where status in ($3, $4, $5)),
			count(*) filter (where status = $4)
		from domain_scraper_collectors
		where profile_id = $1 and provider = $2
	`, profile.ID, provider, domain.ScraperCollectorActive, domain.ScraperCollectorProvisioning, domain.ScraperCollectorHealing).Scan(&activeCount, &totalCount, &provisioningCount)
	if err != nil {
		return domain.ScrapeTarget{}, err
	}

	target := domain.ScrapeTarget{
		Profile:           profile,
		ActiveCollectors:  activeCount,
		TotalCollectors:   totalCount,
		ProvisioningCount: provisioningCount,
	}

	collector, err := selectActiveCollector(ctx, tx, profile.ID, provider)
	if err != nil {
		return domain.ScrapeTarget{}, err
	}
	target.Collector = collector

	if shouldProvisionCollector(profile, activeCount, totalCount, provisioningCount, thresholds) {
		purpose := domain.ScraperCollectorPurposeDefault
		if activeCount > 0 {
			purpose = domain.ScraperCollectorPurposeLoadShard
		}
		provision, err := insertProvisioningCollector(ctx, tx, profile.ID, provider, purpose, now)
		if err != nil {
			return domain.ScrapeTarget{}, err
		}
		if provision != nil {
			target.Provision = provision
			target.TotalCollectors++
			target.ProvisioningCount++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.ScrapeTarget{}, err
	}
	return target, nil
}

func (s *Store) ListCollectorOperations(ctx context.Context) ([]domain.CollectorOperationsProfile, error) {
	rows, err := s.pool.Query(ctx, `
		select p.id, p.domain, p.status, p.request_count, p.product_count, coalesce(latest.url, ''), p.updated_at
		from domain_scraper_profiles p
		left join lateral (
			select url
			from domain_scraper_products
			where profile_id = p.id
			order by last_seen_at desc
			limit 1
		) latest on true
		order by p.request_count desc, p.updated_at desc, p.domain asc
	`)
	if err != nil {
		return nil, err
	}
	profiles, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.CollectorOperationsProfile, error) {
		var profile domain.CollectorOperationsProfile
		err := row.Scan(&profile.ID, &profile.Domain, &profile.Status, &profile.RequestCount, &profile.ProductCount, &profile.LatestProductURL, &profile.UpdatedAt)
		return profile, err
	})
	if err != nil {
		return nil, err
	}

	for i := range profiles {
		collectors, err := s.collectorsForProfile(ctx, profiles[i].ID)
		if err != nil {
			return nil, err
		}
		profiles[i].Collectors = collectors
	}
	return profiles, nil
}

func (s *Store) CollectorHealingTarget(ctx context.Context, collectorID string) (domain.CollectorHealingTarget, error) {
	var target domain.CollectorHealingTarget
	var activeSince sql.NullTime
	err := s.pool.QueryRow(ctx, `
		select
			p.id, p.domain, p.status, p.request_count, p.product_count, p.updated_at,
			c.id, c.profile_id, c.provider, coalesce(c.external_collector_id, ''), c.status, c.purpose, coalesce(c.url_pattern, ''),
			c.request_count, c.success_count, c.failure_count, c.consecutive_structural_failures, coalesce(c.last_error, ''),
			c.active_since, c.created_at, c.updated_at,
			coalesce(latest.url, ''),
			coalesce(
				(select t.country from trackers t where t.product_url = latest.url order by t.updated_at desc limit 1),
				(select pp.country from product_previews pp where pp.url = latest.url order by pp.updated_at desc limit 1),
				'IN'
			)
		from domain_scraper_collectors c
		join domain_scraper_profiles p on p.id = c.profile_id
		left join lateral (
			select url
			from domain_scraper_products
			where profile_id = p.id
			order by last_seen_at desc
			limit 1
		) latest on true
		where c.id = $1
	`, collectorID).Scan(
		&target.Profile.ID,
		&target.Profile.Domain,
		&target.Profile.Status,
		&target.Profile.RequestCount,
		&target.Profile.ProductCount,
		&target.Profile.UpdatedAt,
		&target.Collector.ID,
		&target.Collector.ProfileID,
		&target.Collector.Provider,
		&target.Collector.ExternalCollectorID,
		&target.Collector.Status,
		&target.Collector.Purpose,
		&target.Collector.URLPattern,
		&target.Collector.RequestCount,
		&target.Collector.SuccessCount,
		&target.Collector.FailureCount,
		&target.Collector.ConsecutiveStructuralFailures,
		&target.Collector.LastError,
		&activeSince,
		&target.Collector.CreatedAt,
		&target.Collector.UpdatedAt,
		&target.TargetURL,
		&target.Country,
	)
	if err != nil {
		return domain.CollectorHealingTarget{}, err
	}
	if activeSince.Valid {
		target.Collector.ActiveSince = &activeSince.Time
	}
	return target, nil
}

func (s *Store) collectorsForProfile(ctx context.Context, profileID string) ([]domain.ScraperCollector, error) {
	rows, err := s.pool.Query(ctx, `
		select id, profile_id, provider, coalesce(external_collector_id, ''), status, purpose, coalesce(url_pattern, ''),
		       request_count, success_count, failure_count, consecutive_structural_failures, coalesce(last_error, ''),
		       active_since, created_at, updated_at
		from domain_scraper_collectors
		where profile_id = $1
		order by
			case status
				when $2 then 1
				when $3 then 2
				when $4 then 3
				else 4
			end,
			updated_at desc
	`, profileID, domain.ScraperCollectorHealing, domain.ScraperCollectorActive, domain.ScraperCollectorProvisioning)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.ScraperCollector, error) {
		return scanScraperCollector(row)
	})
}

type queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func selectActiveCollector(ctx context.Context, q queryer, profileID, provider string) (*domain.ScraperCollector, error) {
	row := q.QueryRow(ctx, `
		select id, profile_id, provider, coalesce(external_collector_id, ''), status, purpose, coalesce(url_pattern, ''),
		       request_count, success_count, failure_count, consecutive_structural_failures, coalesce(last_error, ''),
		       active_since, created_at, updated_at
		from domain_scraper_collectors
		where profile_id = $1 and provider = $2 and status in ($3, $4, $5) and external_collector_id is not null
		order by
			case status
				when $3 then 1
				when $4 then 2
				else 3
			end,
			consecutive_structural_failures asc,
			request_count asc,
			updated_at asc
		limit 1
	`, profileID, provider, domain.ScraperCollectorActive, domain.ScraperCollectorHealing, domain.ScraperCollectorFailed)
	collector, err := scanScraperCollector(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &collector, nil
}

func insertProvisioningCollector(ctx context.Context, q queryer, profileID, provider, purpose string, now time.Time) (*domain.ScraperCollector, error) {
	row := q.QueryRow(ctx, `
		insert into domain_scraper_collectors
		(id, profile_id, provider, status, purpose, created_at, updated_at)
		values ('dsc_' || replace(gen_random_uuid()::text, '-', ''), $1, $2, $3, $4, $5, $5)
		on conflict do nothing
		returning id, profile_id, provider, coalesce(external_collector_id, ''), status, purpose, coalesce(url_pattern, ''),
		          request_count, success_count, failure_count, consecutive_structural_failures, coalesce(last_error, ''),
		          active_since, created_at, updated_at
	`, profileID, provider, domain.ScraperCollectorProvisioning, purpose, now)
	collector, err := scanScraperCollector(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &collector, nil
}

func shouldProvisionCollector(profile domain.ScraperProfile, activeCount, totalCount, provisioningCount int, thresholds domain.CollectorThresholds) bool {
	if provisioningCount > 0 || totalCount >= thresholds.MaxPerDomain {
		return false
	}
	if activeCount == 0 {
		return totalCount == 0
	}
	if thresholds.RequestThreshold > 0 && profile.RequestCount >= activeCount*thresholds.RequestThreshold {
		return true
	}
	if thresholds.ProductThreshold > 0 && profile.ProductCount >= activeCount*thresholds.ProductThreshold {
		return true
	}
	return false
}

func (s *Store) CompleteCollectorProvisioning(ctx context.Context, collectorID, externalCollectorID string, now time.Time) error {
	_, err := s.pool.Exec(ctx, `
		update domain_scraper_collectors
		set external_collector_id = $2, status = $3, active_since = coalesce(active_since, $4), last_error = null, updated_at = $4
		where id = $1 and status = $5
	`, collectorID, externalCollectorID, domain.ScraperCollectorActive, now, domain.ScraperCollectorProvisioning)
	return err
}

func (s *Store) ActivateDiscoveredCollector(ctx context.Context, profileID, provider, externalCollectorID, purpose string, now time.Time) (domain.ScraperCollector, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.ScraperCollector{}, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		update domain_scraper_collectors
		set external_collector_id = $3, status = $4, purpose = $5, active_since = coalesce(active_since, $6), last_error = null, updated_at = $6
		where id = (
			select id from domain_scraper_collectors
			where profile_id = $1 and provider = $2 and status = $7
			order by created_at asc
			limit 1
		)
		returning id, profile_id, provider, coalesce(external_collector_id, ''), status, purpose, coalesce(url_pattern, ''),
		          request_count, success_count, failure_count, consecutive_structural_failures, coalesce(last_error, ''),
		          active_since, created_at, updated_at
	`, profileID, provider, externalCollectorID, domain.ScraperCollectorActive, purpose, now, domain.ScraperCollectorProvisioning)
	collector, err := scanScraperCollector(row)
	if err == nil {
		return collector, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.ScraperCollector{}, err
	}

	row = tx.QueryRow(ctx, `
		insert into domain_scraper_collectors
		(id, profile_id, provider, external_collector_id, status, purpose, active_since, created_at, updated_at)
		values ('dsc_' || replace(gen_random_uuid()::text, '-', ''), $1, $2, $3, $4, $5, $6, $6, $6)
		on conflict (provider, external_collector_id) where external_collector_id is not null
		do update set profile_id = excluded.profile_id, status = excluded.status, purpose = excluded.purpose, active_since = coalesce(domain_scraper_collectors.active_since, excluded.active_since), last_error = null, updated_at = excluded.updated_at
		returning id, profile_id, provider, coalesce(external_collector_id, ''), status, purpose, coalesce(url_pattern, ''),
		          request_count, success_count, failure_count, consecutive_structural_failures, coalesce(last_error, ''),
		          active_since, created_at, updated_at
	`, profileID, provider, externalCollectorID, domain.ScraperCollectorActive, purpose, now)
	collector, err = scanScraperCollector(row)
	if err != nil {
		return domain.ScraperCollector{}, err
	}
	return collector, tx.Commit(ctx)
}

func (s *Store) FailCollectorProvisioning(ctx context.Context, collectorID, message string, now time.Time) error {
	_, err := s.pool.Exec(ctx, `
		update domain_scraper_collectors
		set status = $2, last_error = $3, failure_count = failure_count + 1, updated_at = $4
		where id = $1
	`, collectorID, domain.ScraperCollectorFailed, message, now)
	return err
}

func (s *Store) RecordCollectorSuccess(ctx context.Context, collectorID string, now time.Time) error {
	_, err := s.pool.Exec(ctx, `
		update domain_scraper_collectors
		set request_count = request_count + 1,
		    success_count = success_count + 1,
		    consecutive_structural_failures = 0,
		    last_error = null,
		    status = $2,
		    updated_at = $3
		where id = $1
	`, collectorID, domain.ScraperCollectorActive, now)
	return err
}

func (s *Store) RecordCollectorFailure(ctx context.Context, collectorID, message string, structural bool, now time.Time) error {
	_, err := s.pool.Exec(ctx, `
		update domain_scraper_collectors
		set request_count = request_count + 1,
		    failure_count = failure_count + 1,
		    consecutive_structural_failures = case when $2 then consecutive_structural_failures + 1 else consecutive_structural_failures end,
		    last_error = $3,
		    updated_at = $4
		where id = $1
	`, collectorID, structural, message, now)
	return err
}

func (s *Store) StartCollectorHealing(ctx context.Context, collectorID string, now time.Time) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		update domain_scraper_collectors
		set status = $2, updated_at = $3
		where id = $1 and status in ($4, $5) and external_collector_id is not null
	`, collectorID, domain.ScraperCollectorHealing, now, domain.ScraperCollectorActive, domain.ScraperCollectorFailed)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) FinishCollectorHealing(ctx context.Context, collectorID, message string, success bool, now time.Time) error {
	status := domain.ScraperCollectorFailed
	if success {
		status = domain.ScraperCollectorActive
	}
	_, err := s.pool.Exec(ctx, `
		update domain_scraper_collectors
		set status = $2,
		    last_error = nullif($3, ''),
		    consecutive_structural_failures = case when $4 then 0 else consecutive_structural_failures end,
		    updated_at = $5
		where id = $1
	`, collectorID, status, message, success, now)
	return err
}

func scanScraperCollector(row pgx.Row) (domain.ScraperCollector, error) {
	var collector domain.ScraperCollector
	var activeSince sql.NullTime
	err := row.Scan(
		&collector.ID,
		&collector.ProfileID,
		&collector.Provider,
		&collector.ExternalCollectorID,
		&collector.Status,
		&collector.Purpose,
		&collector.URLPattern,
		&collector.RequestCount,
		&collector.SuccessCount,
		&collector.FailureCount,
		&collector.ConsecutiveStructuralFailures,
		&collector.LastError,
		&activeSince,
		&collector.CreatedAt,
		&collector.UpdatedAt,
	)
	if err != nil {
		return domain.ScraperCollector{}, err
	}
	if activeSince.Valid {
		collector.ActiveSince = &activeSince.Time
	}
	return collector, nil
}

func scanTracker(row pgx.CollectableRow) (domain.Tracker, error) {
	var tracker domain.Tracker
	err := row.Scan(&tracker.ID, &tracker.UserID, &tracker.PreviewID, &tracker.ProductURL, &tracker.Name, &tracker.ImageURL, &tracker.Currency, &tracker.Country, &tracker.IntervalHours, &tracker.Status, &tracker.CurrentPrice, &tracker.Availability, &tracker.LastCheckedAt, &tracker.NextCheckAt, &tracker.CreatedAt, &tracker.UpdatedAt)
	return tracker, err
}

func (s *Store) attachRules(ctx context.Context, trackers []domain.Tracker) ([]domain.Tracker, error) {
	for i := range trackers {
		rows, err := s.pool.Query(ctx, `select id, type, threshold_price, enabled from alert_rules where tracker_id = $1 order by created_at asc`, trackers[i].ID)
		if err != nil {
			return nil, err
		}
		rules, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.AlertRule, error) {
			var rule domain.AlertRule
			err := row.Scan(&rule.ID, &rule.Type, &rule.ThresholdPrice, &rule.Enabled)
			return rule, err
		})
		if err != nil {
			return nil, err
		}
		trackers[i].Rules = rules
	}
	return trackers, nil
}
