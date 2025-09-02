package config

import (
	"log"
	"time"

	"github.com/spf13/viper"
)

type AppConfig struct {
	Name string
	Host string
	Port int
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

type Config struct {
	App      AppConfig
	Log      LogConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Jwt      JwtConfig
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
