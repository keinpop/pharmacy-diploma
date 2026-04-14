package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	GRPC struct {
		Port string
	}
	Postgres struct {
		DSN string
	}
	Redis struct {
		Addr     string
		Password string
		DB       int
	}
	Elasticsearch struct {
		Addresses []string
	}
	Inventory struct {
		ExpiringSoonDays int
	}
	ServiceToken string
	AuthAddr     string
}

func Load() (*Config, error) {
	cfg := &Config{}

	cfg.GRPC.Port = getEnv("GRPC_PORT", "50053")
	cfg.Postgres.DSN = getEnv("POSTGRES_DSN", "")
	cfg.Redis.Addr = getEnv("REDIS_ADDR", "localhost:6379")
	cfg.Redis.Password = getEnv("REDIS_PASSWORD", "")
	cfg.Elasticsearch.Addresses = strings.Split(getEnv("ES_ADDRESSES", "http://localhost:9200"), ",")
	cfg.ServiceToken = getEnv("SERVICE_TOKEN", "")
	cfg.AuthAddr = getEnv("AUTH_ADDR", "auth:50051")

	if cfg.Postgres.DSN == "" {
		return nil, fmt.Errorf("POSTGRES_DSN is required")
	}

	days, err := strconv.Atoi(getEnv("INVENTORY_EXPIRING_SOON_DAYS", "30"))
	if err != nil {
		return nil, fmt.Errorf("INVENTORY_EXPIRING_SOON_DAYS must be int: %w", err)
	}
	cfg.Inventory.ExpiringSoonDays = days

	dbNum, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	cfg.Redis.DB = dbNum

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
