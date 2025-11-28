package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/GooferByte/Backend_021Trade/internal/models"
	"github.com/GooferByte/Backend_021Trade/internal/repository"

	"github.com/lib/pq"
)

// Repository implements RewardRepository backed by PostgreSQL.
type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateReward(ctx context.Context, reward models.RewardEvent) error {
	const query = `
		INSERT INTO rewards
		(id, user_id, symbol, quantity, rewarded_at, idempotency_key, fees_brokerage, fees_stt, fees_gst, fees_other, unit_price_inr, total_inr_cost, priced_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`
	_, err := r.db.ExecContext(ctx, query,
		reward.ID, reward.UserID, reward.Symbol, reward.Quantity, reward.RewardedAt, nullableString(reward.IdempotencyKey),
		reward.Fees.Brokerage, reward.Fees.STT, reward.Fees.GST, reward.Fees.Other, reward.UnitPriceINR, reward.TotalINRCost, reward.PricedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return repository.ErrDuplicateReward
		}
		return err
	}
	return nil
}

func (r *Repository) FindByIdempotencyKey(ctx context.Context, userID, key string) (*models.RewardEvent, error) {
	if key == "" {
		return nil, nil
	}
	const query = `
		SELECT id, user_id, symbol, quantity, rewarded_at, idempotency_key, fees_brokerage, fees_stt, fees_gst, fees_other, unit_price_inr, total_inr_cost, priced_at
		FROM rewards
		WHERE user_id = $1 AND idempotency_key = $2
	`
	row := r.db.QueryRowContext(ctx, query, userID, key)
	var evt models.RewardEvent
	var idem sql.NullString
	if err := row.Scan(&evt.ID, &evt.UserID, &evt.Symbol, &evt.Quantity, &evt.RewardedAt, &idem, &evt.Fees.Brokerage, &evt.Fees.STT, &evt.Fees.GST, &evt.Fees.Other, &evt.UnitPriceINR, &evt.TotalINRCost, &evt.PricedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	evt.IdempotencyKey = idem.String
	return &evt, nil
}

func (r *Repository) ListRewardsByUserAndDate(ctx context.Context, userID string, day time.Time) ([]models.RewardEvent, error) {
	start, end := bounds(day)
	const query = `
		SELECT id, user_id, symbol, quantity, rewarded_at, idempotency_key, fees_brokerage, fees_stt, fees_gst, fees_other, unit_price_inr, total_inr_cost, priced_at
		FROM rewards
		WHERE user_id = $1 AND rewarded_at >= $2 AND rewarded_at < $3
		ORDER BY rewarded_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRewards(rows)
}

func (r *Repository) ListRewardsBeforeDate(ctx context.Context, userID string, before time.Time) ([]models.RewardEvent, error) {
	cutoff, _ := bounds(before)
	const query = `
		SELECT id, user_id, symbol, quantity, rewarded_at, idempotency_key, fees_brokerage, fees_stt, fees_gst, fees_other, unit_price_inr, total_inr_cost, priced_at
		FROM rewards
		WHERE user_id = $1 AND rewarded_at < $2
		ORDER BY rewarded_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRewards(rows)
}

func (r *Repository) ListAllRewards(ctx context.Context, userID string) ([]models.RewardEvent, error) {
	const query = `
		SELECT id, user_id, symbol, quantity, rewarded_at, idempotency_key, fees_brokerage, fees_stt, fees_gst, fees_other, unit_price_inr, total_inr_cost, priced_at
		FROM rewards
		WHERE user_id = $1
		ORDER BY rewarded_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRewards(rows)
}

func (r *Repository) UpsertLedgerEntries(ctx context.Context, entries []models.LedgerEntry) error {
	const query = `
		INSERT INTO ledger_entries
		(id, event_id, user_id, account, symbol, units, amount_inr, entry_type, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if _, err := tx.ExecContext(ctx, query, e.ID, e.EventID, e.UserID, e.Account, e.Symbol, e.Units, e.AmountINR, e.EntryType, e.CreatedAt); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func scanRewards(rows *sql.Rows) ([]models.RewardEvent, error) {
	out := []models.RewardEvent{}
	for rows.Next() {
		var evt models.RewardEvent
		var idem sql.NullString
		if err := rows.Scan(&evt.ID, &evt.UserID, &evt.Symbol, &evt.Quantity, &evt.RewardedAt, &idem, &evt.Fees.Brokerage, &evt.Fees.STT, &evt.Fees.GST, &evt.Fees.Other, &evt.UnitPriceINR, &evt.TotalINRCost, &evt.PricedAt); err != nil {
			return nil, err
		}
		evt.IdempotencyKey = idem.String
		out = append(out, evt)
	}
	return out, rows.Err()
}

func bounds(t time.Time) (time.Time, time.Time) {
	y, m, d := t.Date()
	start := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return start, start.Add(24 * time.Hour)
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}
	return false
}
