package config

import (
	"log"
	"time"

	"github.com/spf13/viper"
)

type AppConfig struct {
	Name            string
	Host            string
	Port            int
	ShutdownTimeout int // seconds (default 15)
}

type LogConfig struct {
	Path   string
	Stdout bool
}

type RedisConfig struct {
	Username string
	Password string
	Host     string
	Port     int
	DB       int
	Pool     struct {
		Idle    int
		Max     int
		Timeout int
	}
}

type DatabaseConfig struct {
	Username string
	Password string
	Host     string
	Port     int
	Name     string
	Pool     struct {
		Idle     int
		Max      int
		Lifetime int
	}
}

type JwtConfig struct {
	Secret  string
	Expired int //minutes
}

type CircuitBreakerConfig struct {
	FailureThreshold float64
	MinRequests      int
	Timeout          time.Duration
	MaxHalfOpenReq   int
}

type RabbitMQOutboxConfig struct {
	Enabled      bool
	PollInterval int // seconds (default 2)
	BatchSize    int // (default 50)
	MaxAttempts  int // (default 10)
	BaseBackoff  int // seconds (default 5)
	MaxBackoff   int // seconds (default 300)
}

type RabbitMQConfig struct {
	Username          string
	Password          string
	Host              string
	Port              int
	VHost             string
	TLS               bool // pakai amqps:// untuk koneksi terenkripsi
	ConnectTimeout    int  // seconds (default 30)
	Heartbeat         int  // seconds (default 10)
	PrefetchCount     int
	ReconnectDelay    int // seconds, initial backoff (default 2)
	MaxReconnectDelay int // seconds, cap backoff (default 30)
	PublishTimeout    int // seconds, timeout konfirmasi publish (default 10)
	ChannelPoolSize   int // jumlah channel paralel untuk publish (default 5)
	Breaker           CircuitBreakerConfig
	Outbox            RabbitMQOutboxConfig
}

type KafkaSASLConfig struct {
	Enabled   bool
	Mechanism string // "PLAIN" | "SCRAM-SHA-256" | "SCRAM-SHA-512"
	Username  string
	Password  string
}

type KafkaConfig struct {
	Brokers      []string
	ClientID     string
	TLS          bool
	SASL         KafkaSASLConfig
	DialTimeout  int    // seconds (default 10)
	WriteTimeout int    // seconds (default 10)
	ReadTimeout  int    // seconds (default 10)
	RequiredAcks int    // -1=all, 0=none, 1=leader (default -1)
	Compression  string // "none","gzip","snappy","lz4","zstd" (default "snappy")
	BatchSize    int    // (default 100)
	BatchTimeout int    // ms (default 50)
	GroupID      string // default consumer group
	StartOffset  string // "earliest" | "latest" (default "latest")
	MinBytes     int    // (default 1)
	MaxBytes     int    // (default 1MiB = 1048576)
	MaxAttempts  int    // producer retries (default 5)
	Breaker      CircuitBreakerConfig
}

type Config struct {
	App      AppConfig
	Log      LogConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Jwt      JwtConfig
	RabbitMQ RabbitMQConfig
	Kafka    KafkaConfig
}

func NewConfig() *Config {
	viper := viper.New()

	viper.SetConfigName("config")
	viper.SetConfigType("json")
	viper.AddConfigPath("./../")
	viper.AddConfigPath("./")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Fatal error config file: %w \n", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("Fatal error parse config: %w \n", err)
	}

	return &config
}
