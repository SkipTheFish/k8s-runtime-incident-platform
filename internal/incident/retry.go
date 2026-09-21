package incident

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxAttempts: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond}
}

func RetryTransaction[T any](ctx context.Context, policy RetryPolicy, operation func(context.Context) (T, error)) (T, error) {
	var zero T
	if policy.MaxAttempts < 1 {
		policy.MaxAttempts = 1
	}
	if policy.BaseDelay < 0 {
		policy.BaseDelay = 0
	}
	if policy.MaxDelay < policy.BaseDelay {
		policy.MaxDelay = policy.BaseDelay
	}
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		value, err := operation(ctx)
		if err == nil {
			return value, nil
		}
		if !IsRetryableTransactionError(err) || attempt == policy.MaxAttempts {
			return zero, err
		}
		delay := policy.BaseDelay << (attempt - 1)
		if delay > policy.MaxDelay {
			delay = policy.MaxDelay
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, ctx.Err()
		case <-timer.C:
		}
	}
	return zero, nil
}

func IsRetryableTransactionError(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40001" || pgErr.Code == "40P01" || len(pgErr.Code) >= 2 && pgErr.Code[:2] == "08"
	}
	return pgconn.SafeToRetry(err)
}
