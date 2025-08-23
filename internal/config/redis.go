package config

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedis(config *Config) *redis.Client {
	ctx := context.Background()

	// konfigurasi koneksi ke Dragonfly
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", config.Redis.Host, config.Redis.Port),
		Password:     config.Redis.Password,
		DB:           config.Redis.DB,
		PoolSize:     config.Redis.Pool.Max,
		MinIdleConns: config.Redis.Pool.Idle,
		PoolTimeout:  time.Duration(config.Redis.Pool.Timeout) * time.Second, //30s
	})

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Gagal konek ke Dragonfly(redis): %v", err)
	}

	return rdb
}
