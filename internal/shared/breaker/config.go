package breaker

import "time"

// Config governs circuit-breaker behavior. Defined here (not in the
// `config` package) so the breaker package stays import-cycle free.
type Config struct {
	FailureThreshold float64       // 0..1
	MinRequests      int           // minimum samples before ratio check
	Timeout          time.Duration // Open → HalfOpen wait
	MaxHalfOpenReq   int           // probes allowed during HalfOpen
}
