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

	"pharmacy/sales/adapter/grpcclient"
	kafkaadapter "pharmacy/sales/adapter/kafka"
	pgadapter "pharmacy/sales/adapter/postgres"
	grpcapp "pharmacy/sales/app/grpc"
	"pharmacy/sales/app/metrics"
	"pharmacy/sales/config"
	usecase "pharmacy/sales/domain/use_case"
)

func main() {
	cfg := config.Load()

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = logger.Sync() }()

	db, err := connectPostgres(cfg.PostgresDSN)
	if err != nil {
		logger.Fatal("postgres connect failed", zap.Error(err))
	}
	defer db.Close()

	kafkaProducer := kafkaadapter.NewKafkaProducer(cfg.KafkaBrokers, logger)
	defer kafkaProducer.Close()

	authClient, err := grpcapp.NewAuthClient(cfg.AuthAddr)
	if err != nil {
		logger.Fatal("auth client failed", zap.Error(err))
	}

	inventoryClient, err := grpcclient.NewInventoryClient(cfg.InventoryAddr)
	if err != nil {
		logger.Fatal("inventory client failed", zap.Error(err))
	}

	saleRepo := pgadapter.NewSaleRepository(db)
	// SellerProvider достаёт username из context — его туда кладёт AuthInterceptor,
	// который, в свою очередь, читает данные из JWT через auth-сервис.
	salesUC := usecase.NewSalesUseCase(saleRepo, inventoryClient, kafkaProducer, grpcapp.AuthUsernameFromContext)

	handler := grpcapp.NewHandler(salesUC)
	srv := grpcapp.NewServer(cfg.GRPCPort, handler, authClient, logger, cfg.ServiceToken)

	go func() {
		if err := srv.Run(); err != nil {
			logger.Fatal("grpc server failed", zap.Error(err))
		}
	}()

	go runMetricsServer(cfg.MetricsAddr, logger)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down sales service")
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
