package breaker

import (
	"context"
	"errors"
	"time"
)

// Retry menjalankan fn beberapa kali dengan backoff sederhana
func Retry(
	ctx context.Context,
	attempts int,
	backoff time.Duration,
	fn func(context.Context) (interface{}, error),
) (interface{}, error) {

	var result interface{}
	var err error

	for i := 0; i < attempts; i++ {
		result, err = fn(ctx)
		if err == nil {
			return result, nil // sukses
		}

		// kalau context cancelled → stop
		select {
		case <-ctx.Done():
			return nil, errors.New("context cancelled")
		case <-time.After(backoff * time.Duration(i+1)):
			// lanjut retry
		}
	}

	return result, err
}

// RetryWithCircuitBreaker mencoba eksekusi fn beberapa kali dengan backoff
func RetryWithCircuitBreaker(
	ctx context.Context,
	cb *CircuitBreaker,
	attempts int,
	backoff time.Duration,
	fn func(context.Context) (interface{}, error),
) (interface{}, error) {

	var result interface{}
	var err error

	for i := 0; i < attempts; i++ {
		result, err = cb.Execute(ctx, fn)
		if err == nil {
			return result, nil // sukses
		}

		// kalau circuit breaker open → stop retry
		if err.Error() == "circuit breaker is open" {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, errors.New("context cancelled")
		case <-time.After(backoff * time.Duration(i+1)):
			// lanjut retry
		}
	}

	return result, err
}
