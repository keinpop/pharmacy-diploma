package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"go.uber.org/zap"

	chchadapter "pharmacy/analytics/adapter/clickhouse"
	grpcclientadapter "pharmacy/analytics/adapter/grpcclient"
	kafkaadapter "pharmacy/analytics/adapter/kafka"
	pgadapter "pharmacy/analytics/adapter/postgres"
	grpcapp "pharmacy/analytics/app/grpc"
	"pharmacy/analytics/app/metrics"
	"pharmacy/analytics/app/worker"
	"pharmacy/analytics/config"
	usecase "pharmacy/analytics/domain/use_case"
)

func main() {
	cfg := config.Load()

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = logger.Sync() }()

	// — PostgreSQL —
	db, err := connectPostgres(cfg.PostgresDSN)
	if err != nil {
		logger.Fatal("postgres connect failed", zap.Error(err))
	}
	defer db.Close()

	if err := runPostgresMigrations(db); err != nil {
		logger.Fatal("postgres migrations failed", zap.Error(err))
	}

	// — ClickHouse —
	eventRepo, err := connectClickHouse(cfg.ClickHouseDSN)
	if err != nil {
		logger.Fatal("clickhouse connect failed", zap.Error(err))
	}

	if err := runClickHouseMigrations(eventRepo); err != nil {
		logger.Fatal("clickhouse migrations failed", zap.Error(err))
	}

	// — Context for consumers & worker —
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// — Kafka consumers —
	kafkaadapter.StartConsumers(ctx, cfg.KafkaBrokers, eventRepo, logger)

	// — gRPC clients —
	inventoryClient, err := grpcclientadapter.NewInventoryClient(cfg.InventoryAddr, cfg.ServiceToken)
	if err != nil {
		logger.Fatal("inventory client failed", zap.Error(err))
	}

	authClient, err := grpcapp.NewAuthClient(cfg.AuthAddr)
	if err != nil {
		logger.Fatal("auth client failed", zap.Error(err))
	}

	// — Repos & use case —
	reportRepo := pgadapter.NewReportRepo(db)
	analyticsUC := usecase.NewAnalyticsUseCase(
		reportRepo,
		eventRepo,
		inventoryClient,
		config.GetSeasonalCoefficient,
	)

	// — Worker —
	w := worker.NewWorker(
		reportRepo,
		analyticsUC,
		logger,
		time.Duration(cfg.PollInterval)*time.Second,
	)
	go w.Run(ctx)

	// — gRPC server —
	handler := grpcapp.NewHandler(analyticsUC)
	srv := grpcapp.NewServer(cfg.GRPCPort, handler, authClient, logger, cfg.ServiceToken)

	go func() {
		if err := srv.Run(); err != nil {
			logger.Fatal("grpc server failed", zap.Error(err))
		}
	}()

	go runMetricsServer(cfg.MetricsAddr, logger)

	// — Graceful shutdown —
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down analytics service")

	cancel() // stop consumers & worker
	srv.Stop()
}

func runMetricsServer(addr string, logger *zap.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	logger.Info("metrics endpoint listening", zap.String("addr", addr))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Warn("metrics server stopped", zap.Error(err))
	}
}

// connectPostgres opens a PostgreSQL connection with retry.
func connectPostgres(dsn string) (*sql.DB, error) {
	var db *sql.DB
	var err error
	for i := 0; i < 10; i++ {
		db, err = sql.Open("postgres", dsn)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err = db.PingContext(ctx)
			cancel()
			if err == nil {
				return db, nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("could not connect to postgres after 10 attempts: %w", err)
}

// connectClickHouse opens a ClickHouse connection with retry.
func connectClickHouse(dsn string) (*chchadapter.EventRepo, error) {
	var repo *chchadapter.EventRepo
	var err error
	for i := 0; i < 10; i++ {
		repo, err = chchadapter.NewEventRepo(dsn)
		if err == nil {
			return repo, nil
		}
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("could not connect to clickhouse after 10 attempts: %w", err)
}

// runPostgresMigrations executes the Postgres DDL inline.
func runPostgresMigrations(db *sql.DB) error {
	ddl := `
CREATE TABLE IF NOT EXISTS reports (
    id           TEXT PRIMARY KEY,
    type         TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending',
    params       JSONB NOT NULL DEFAULT '{}',
    result       JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    error        TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_reports_status ON reports(status);
CREATE INDEX IF NOT EXISTS idx_reports_type ON reports(type);
`
	_, err := db.Exec(ddl)
	return err
}

// runClickHouseMigrations executes the ClickHouse DDL inline.
func runClickHouseMigrations(repo *chchadapter.EventRepo) error {
	return repo.RunMigrations(context.Background())
}
