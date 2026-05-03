package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	GRPCPort    string
	DBDsn       string
	RedisAddr   string
	JWTSecret   string
	JWTTTL      time.Duration
	MetricsAddr string
}

func Load() (*Config, error) {
	cfg := &Config{
		GRPCPort:    getEnv("GRPC_PORT", "50051"),
		DBDsn:       getEnv("DB_DSN", ""),
		RedisAddr:   getEnv("REDIS_ADDR", "localhost:6379"),
		JWTSecret:   getEnv("JWT_SECRET", "dev-secret-change-me"),
		JWTTTL:      24 * time.Hour,
		MetricsAddr: getEnv("METRICS_ADDR", ":9101"),
	}
	if cfg.DBDsn == "" {
		return nil, fmt.Errorf("DB_DSN is required")
	}
	if v := os.Getenv("JWT_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.JWTTTL = d
		}
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
