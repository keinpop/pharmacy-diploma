package config

import (
	"os"
	"strconv"
)

type Config struct {
	GRPCPort      string
	PostgresDSN   string
	ClickHouseDSN string
	KafkaBrokers  string
	AuthAddr      string
	InventoryAddr string
	ServiceToken  string
	PollInterval  int // seconds
}

func Load() *Config {
	pollInterval := 5
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			pollInterval = n
		}
	}

	return &Config{
		GRPCPort:      getEnv("GRPC_PORT", "50055"),
		PostgresDSN:   getEnv("POSTGRES_DSN", "postgres://analytics:analytics@localhost:5432/analytics?sslmode=disable"),
		ClickHouseDSN: getEnv("CLICKHOUSE_DSN", "clickhouse://default:@localhost:9000/analytics"),
		KafkaBrokers:  getEnv("KAFKA_BROKERS", "localhost:9092"),
		AuthAddr:      getEnv("AUTH_ADDR", "auth:50051"),
		InventoryAddr: getEnv("INVENTORY_ADDR", "inventory:50053"),
		ServiceToken:  getEnv("SERVICE_TOKEN", "internal-service-secret"),
		PollInterval:  pollInterval,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
