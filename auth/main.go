package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
	goredis "github.com/redis/go-redis/v9"

	"pharma/auth/adapter/postgres"
	redisrepo "pharma/auth/adapter/redis"
	appgrpc "pharma/auth/app/grpc"
	"pharma/auth/config"
	usecase "pharma/auth/domain/use_case"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := connectPostgres(cfg.DBDsn)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	redisClient := goredis.NewClient(&goredis.Options{
		Addr: cfg.RedisAddr,
	})

	userRepo := postgres.NewUserRepository(db)
	sessionRepo := redisrepo.NewSessionRepository(redisClient)

	authUC := usecase.NewAuthUseCase(userRepo, sessionRepo)

	srv := appgrpc.NewServer(authUC)

	log.Printf("Auth gRPC server starting on port %s", cfg.GRPCPort)
	if err := appgrpc.Run(srv, cfg.GRPCPort); err != nil {
		log.Fatalf("grpc server: %v", err)
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
