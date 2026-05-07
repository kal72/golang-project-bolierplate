package kafka

// SASLConfig configures SASL authentication. Mechanism accepts
// "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512".
type SASLConfig struct {
	Enabled   bool
	Mechanism string
	Username  string
	Password  string
}

// Config holds the Kafka client settings the package needs. Higher-level
// concerns (circuit breaker) are wired by the application container.
type Config struct {
	Brokers      []string
	ClientID     string
	TLS          bool
	SASL         SASLConfig
	DialTimeout  int    // seconds
	WriteTimeout int    // seconds
	ReadTimeout  int    // seconds
	RequiredAcks int    // -1=all, 0=none, 1=leader
	Compression  string // "none","gzip","snappy","lz4","zstd"
	BatchSize    int
	BatchTimeout int    // ms
	GroupID      string
	StartOffset  string // "earliest" | "latest"
	MinBytes     int
	MaxBytes     int
	MaxAttempts  int
}
