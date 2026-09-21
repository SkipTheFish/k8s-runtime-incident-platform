package incident

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestRetryTransactionRetriesTransientFailures(t *testing.T) {
	for _, code := range []string{"40001", "40P01", "08006"} {
		t.Run(code, func(t *testing.T) {
			attempts := 0
			got, err := RetryTransaction(context.Background(), RetryPolicy{MaxAttempts: 3}, func(context.Context) (string, error) {
				attempts++
				if attempts < 3 {
					return "", &pgconn.PgError{Code: code}
				}
				return "ok", nil
			})
			if err != nil || got != "ok" || attempts != 3 {
				t.Fatalf("got=%q attempts=%d err=%v", got, attempts, err)
			}
		})
	}
}

func TestRetryTransactionDoesNotRetryPermanentFailure(t *testing.T) {
	for _, failure := range []error{errors.New("invalid input"), &pgconn.PgError{Code: "23505"}} {
		attempts := 0
		_, err := RetryTransaction(context.Background(), DefaultRetryPolicy(), func(context.Context) (struct{}, error) {
			attempts++
			return struct{}{}, failure
		})
		if !errors.Is(err, failure) || attempts != 1 {
			t.Fatalf("attempts=%d err=%v", attempts, err)
		}
	}
}

func TestRetryTransactionHonorsContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	attempts := 0
	_, err := RetryTransaction(ctx, RetryPolicy{MaxAttempts: 5, BaseDelay: time.Second, MaxDelay: time.Second}, func(context.Context) (struct{}, error) {
		attempts++
		return struct{}{}, &pgconn.PgError{Code: "40001"}
	})
	if !errors.Is(err, context.DeadlineExceeded) || attempts != 1 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}
