package postgres

import (
	"context"
	"errors"
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
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
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
