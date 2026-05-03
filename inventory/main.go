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

	"github.com/elastic/go-elasticsearch/v8"
	"go.uber.org/zap"

	esadapter "pharmacy/inventory/adapter/elastic"
	kafkaadapter "pharmacy/inventory/adapter/kafka"
	pgadapter "pharmacy/inventory/adapter/postgres"
	grpcapp "pharmacy/inventory/app/grpc"
	"pharmacy/inventory/app/metrics"
	"pharmacy/inventory/config"
	usecase "pharmacy/inventory/domain/use_case"
)

func main() {
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	cfg := config.Load()

	// Postgres
	db, err := connectPostgres(cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	kafkaProducer := kafkaadapter.NewKafkaProducer(cfg.KafkaBrokers, logger)
	defer kafkaProducer.Close()

	// Elasticsearch
	esClient, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{cfg.ESAddresses},
	})
	if err != nil {
		logger.Fatal("elasticsearch connect", zap.Error(err))
	}

	// Auth gRPC client — для валидации user-токенов
	authClient, err := grpcapp.NewAuthClient(cfg.AuthAddr)
	if err != nil {
		logger.Fatal("auth client init", zap.Error(err))
	}

	// Repositories
	productRepo := pgadapter.NewProductRepository(db)
	batchRepo := pgadapter.NewBatchRepository(db)
	stockRepo := pgadapter.NewStockRepository(db)
	searchRepo := esadapter.NewProductSearchRepo(esClient)

	// После инициализации репозиториев и до запуска gRPC:
	if err := syncElasticsearch(context.Background(), productRepo, searchRepo, logger); err != nil {
		logger.Warn("elasticsearch initial sync failed", zap.Error(err))
		// не fatal — сервис работает, поиск просто пустой
	}

	// Use-case

	uc := usecase.NewInventoryUseCase(
		batchRepo,
		stockRepo,
		productRepo,
		searchRepo,
		kafkaProducer,
		cfg.ExpiringSoonDays,
	)

	// gRPC server
	handler := grpcapp.NewHandler(uc)
	srv := grpcapp.NewServer(cfg.GRPCPort, handler, authClient, logger, cfg.ServiceToken)

	go func() {
		if err := srv.Run(); err != nil {
			logger.Fatal("grpc serve", zap.Error(err))
		}
	}()

	go runMetricsServer(cfg.MetricsAddr, logger)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down inventory service")
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

func connectPostgres(dsn string) (*sql.DB, error) {
	var db *sql.DB
	var err error

	for i := range 10 {
		db, err = sql.Open("postgres", dsn)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			return db, nil
		}
		log.Printf("waiting for postgres (attempt %d/10): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("could not connect to postgres after 10 attempts: %w", err)
}

func syncElasticsearch(
	ctx context.Context,
	products usecase.ProductRepository,
	search usecase.SearchRepository,
	logger *zap.Logger,
) error {
	logger.Info("syncing products to elasticsearch...")
	// Грузим все продукты постранично
	page, pageSize := 1, 100
	total := 0
	for {
		batch, count, err := products.List(ctx, page, pageSize)
		if err != nil {
			return err
		}
		if err := search.ReindexAll(ctx, batch); err != nil {
			return err
		}
		total += len(batch)
		if total >= count {
			break
		}
		page++
	}
	logger.Info("elasticsearch sync complete", zap.Int("indexed", total))
	return nil
}
