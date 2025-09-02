package breaker

import (
	"context"
	"errors"
	"golang-project-boilerplate/internal/config"
	"sync"
	"time"
)

type CircuitBreaker struct {
	mu          sync.Mutex
	config      config.CircuitBreakerConfig
	state       State
	failures    int
	success     int
	lastFail    time.Time
	halfOpenReq int
}

func NewCircuitBreaker(config config.CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	cb.mu.Lock()
	state := cb.currentState()
	cb.mu.Unlock()

	switch state {
	case StateOpen:
		return nil, errors.New("circuit breaker is open")
	case StateHalfOpen:
		// batasi request di half-open
		cb.mu.Lock()
		if cb.halfOpenReq >= cb.config.MaxHalfOpenReq {
			cb.mu.Unlock()
			return nil, errors.New("circuit breaker half-open limit reached")
		}
		cb.halfOpenReq++
		cb.mu.Unlock()
	}

	// jalankan fungsi
	result, err := fn(ctx)

	// update status
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.setFailure()
		return result, err
	}

	cb.setSuccess()
	return result, nil
}

func (cb *CircuitBreaker) currentState() State {
	if cb.state == StateOpen {
		if time.Since(cb.lastFail) > cb.config.Timeout {
			cb.state = StateHalfOpen
			cb.halfOpenReq = 0
		}
	}
	return cb.state
}

func (cb *CircuitBreaker) setFailure() {
	cb.failures++
	cb.lastFail = time.Now()

	total := cb.failures + cb.success
	if cb.state == StateHalfOpen {
		// gagal di half-open → balik ke open
		cb.state = StateOpen
		cb.failures, cb.success = 0, 0
		return
	}

	if total >= cb.config.MinRequests {
		ratio := float64(cb.failures) / float64(total)
		if ratio >= cb.config.FailureThreshold {
			cb.state = StateOpen
			cb.failures, cb.success = 0, 0
		}
	}
}

func (cb *CircuitBreaker) setSuccess() {
	if cb.state == StateHalfOpen {
		// kalau sukses di half-open → kembali closed
		cb.state = StateClosed
		cb.failures, cb.success = 0, 0
		cb.halfOpenReq = 0
		return
	}
	cb.success++
}

func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
