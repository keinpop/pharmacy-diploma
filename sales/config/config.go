package config

import "os"

type Config struct {
	GRPCPort      string
	PostgresDSN   string
	AuthAddr      string
	InventoryAddr string
	ServiceToken  string
	KafkaBrokers  string
}

func Load() *Config {
	return &Config{
		GRPCPort:      getEnv("GRPC_PORT", "50054"),
		PostgresDSN:   getEnv("POSTGRES_DSN", "postgres://sales:sales@localhost:5432/sales?sslmode=disable"),
		AuthAddr:      getEnv("AUTH_ADDR", "auth:50051"),
		InventoryAddr: getEnv("INVENTORY_ADDR", "inventory:50053"),
		ServiceToken:  getEnv("SERVICE_TOKEN", "internal-service-secret"),
		KafkaBrokers:  getEnv("KAFKA_BROKERS", "kafka:9092"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
