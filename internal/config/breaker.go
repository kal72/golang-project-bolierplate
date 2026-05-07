package config

import "golang-project-boilerplate/internal/shared/breaker"

// NewCircuitBreaker constructs a circuit breaker from a CircuitBreakerConfig.
// Kept here so callers don't repeat the conversion at every call site.
func NewCircuitBreaker(c CircuitBreakerConfig) *breaker.CircuitBreaker {
	return breaker.NewCircuitBreaker(breaker.Config{
		FailureThreshold: c.FailureThreshold,
		MinRequests:      c.MinRequests,
		Timeout:          c.Timeout,
		MaxHalfOpenReq:   c.MaxHalfOpenReq,
	})
}
