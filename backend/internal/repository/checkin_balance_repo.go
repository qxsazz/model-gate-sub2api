package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *userRepository) GetBillingBalance(ctx context.Context, id int64) (float64, time.Time, error) {
	client := clientFromContext(ctx, r.client)
	query := `SELECT balance, next_expiry FROM checkin_available_balance($1)`
	if client.Driver().Dialect() != dialect.Postgres {
		query = `SELECT balance, NULL FROM users WHERE id=$1 AND deleted_at IS NULL`
	}
	rows, err := client.QueryContext(ctx, query, id)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, time.Time{}, err
		}
		return 0, time.Time{}, service.ErrUserNotFound
	}
	var balance float64
	var expiry sql.NullTime
	if err := rows.Scan(&balance, &expiry); err != nil {
		return 0, time.Time{}, err
	}
	return balance, expiry.Time, rows.Err()
}

func (r *userRepository) ExpireCheckinBalances(ctx context.Context, limit int) ([]int64, error) {
	if r.client.Driver().Dialect() != dialect.Postgres {
		return nil, nil
	}
	rows, err := r.sql.QueryContext(ctx, `SELECT * FROM checkin_expire_balances($1)`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Absolute balance changes and capped refunds must settle expired credit before they
// calculate an amount from the wallet. Keep both statements under the same row lock.
func (r *userRepository) withSettledBalance(ctx context.Context, id int64, apply func(*dbent.Client) error) error {
	client := clientFromContext(ctx, r.client)
	if client.Driver().Dialect() != dialect.Postgres {
		return apply(client)
	}
	run := func(c *dbent.Client) error {
		if _, err := c.ExecContext(ctx, `SELECT checkin_lock_balance($1)`, id); err != nil {
			return err
		}
		return apply(c)
	}
	if dbent.TxFromContext(ctx) != nil {
		return run(client)
	}
	tx, err := client.Tx(ctx)
	if errors.Is(err, dbent.ErrTxStarted) {
		return run(client)
	}
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := run(tx.Client()); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *userRepository) changeSettledBalance(ctx context.Context, id int64, amount float64, query string) (change service.BalanceChange, found bool, err error) {
	err = r.withSettledBalance(ctx, id, func(client *dbent.Client) error {
		var scanErr error
		change, found, scanErr = scanBalanceChange(ctx, client, query, amount, id)
		return scanErr
	})
	return
}
