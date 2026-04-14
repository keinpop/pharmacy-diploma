package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"pharmacy/sales/adapter/grpcclient"
	pgadapter "pharmacy/sales/adapter/postgres"
	grpcapp "pharmacy/sales/app/grpc"
	"pharmacy/sales/config"
	usecase "pharmacy/sales/domain/use_case"
)

func main() {
	cfg := config.Load()

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Sync()

	db, err := connectPostgres(cfg.PostgresDSN)
	if err != nil {
		logger.Fatal("postgres connect failed", zap.Error(err))
	}
	defer db.Close()

	authClient, err := grpcapp.NewAuthClient(cfg.AuthAddr)
	if err != nil {
		logger.Fatal("auth client failed", zap.Error(err))
	}

	inventoryClient, err := grpcclient.NewInventoryClient(cfg.InventoryAddr)
	if err != nil {
		logger.Fatal("inventory client failed", zap.Error(err))
	}

	saleRepo := pgadapter.NewSaleRepository(db)
	salesUC := usecase.NewSalesUseCase(saleRepo, inventoryClient)

	handler := grpcapp.NewHandler(salesUC)
	srv := grpcapp.NewServer(cfg.GRPCPort, handler, authClient, logger, cfg.ServiceToken)

	go func() {
		if err := srv.Run(); err != nil {
			logger.Fatal("grpc server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down sales service")
	srv.Stop()
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
