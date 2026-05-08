package rabbitmq

// Config holds the connection-level settings the rabbitmq package needs.
// Higher-level concerns (circuit breaker, outbox) are wired separately in
// the application container, not stored here.
type Config struct {
	Username          string
	Password          string
	Host              string
	Port              int
	VHost             string
	TLS               bool
	ConnectTimeout    int // seconds
	Heartbeat         int // seconds
	PrefetchCount     int
	ReconnectDelay    int // seconds, initial backoff
	MaxReconnectDelay int // seconds, cap backoff
	PublishTimeout    int // seconds
	ChannelPoolSize   int // jumlah channel paralel untuk publish (default 5)
}
