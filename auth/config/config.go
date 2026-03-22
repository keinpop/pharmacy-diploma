package config

import (
	"fmt"
	"os"
)

type Config struct {
	GRPCPort  string
	DBDsn     string
	RedisAddr string
}

func Load() (*Config, error) {
	cfg := &Config{
		GRPCPort:  getEnv("GRPC_PORT", "50051"),
		DBDsn:     getEnv("DB_DSN", ""),
		RedisAddr: getEnv("REDIS_ADDR", "localhost:6379"),
	}
	if cfg.DBDsn == "" {
		return nil, fmt.Errorf("DB_DSN is required")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
