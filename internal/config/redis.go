package config

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedis(cfg *Config) *redis.Client {
	ctx := context.Background()

	// konfigurasi koneksi ke Dragonfly
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.Pool.Max,
		MinIdleConns: cfg.Redis.Pool.Idle,
		PoolTimeout:  time.Duration(cfg.Redis.Pool.Timeout) * time.Second, //30s
	})

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Gagal konek ke Dragonfly(redis): %v", err)
	}

	return rdb
}
