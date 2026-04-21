package config

import (
	"os"
	"strconv"
)

type Config struct {
	GRPCPort         string
	PostgresDSN      string
	RedisAddr        string
	ESAddresses      string
	ExpiringSoonDays int
	ServiceToken     string
	AuthAddr         string
	KafkaBrokers     string
}

func Load() *Config {
	return &Config{
		GRPCPort:         getEnv("GRPC_PORT", "50053"),
		PostgresDSN:      getEnv("POSTGRES_DSN", "postgres://inventory:inventory@localhost:5432/inventory?sslmode=disable"),
		RedisAddr:        getEnv("REDIS_ADDR", "redis:6379"),
		ESAddresses:      getEnv("ES_ADDRESSES", "http://elasticsearch:9200"),
		ExpiringSoonDays: getEnvInt("EXPIRING_SOON_DAYS", 30),
		ServiceToken:     getEnv("SERVICE_TOKEN", "internal-service-secret"),
		AuthAddr:         getEnv("AUTH_ADDR", "auth:50051"),
		KafkaBrokers:     getEnv("KAFKA_BROKERS", "kafka:9092"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
