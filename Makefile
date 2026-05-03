PROTO_DIR     := proto
AUTH_OUT      := auth/gen/
INVENTORY_OUT := inventory/gen/
SALES_OUT     := sales/gen/
ANALYTICS_OUT := analytics/gen/

.PHONY: proto proto-auth proto-inventory proto-sales proto-analytics \
        build build-auth build-inventory build-sales build-analytics \
        test test-auth test-inventory test-sales test-analytics \
        migrate-auth migrate-inventory migrate-sales migrate-analytics \
        lint lint-auth lint-inventory lint-sales lint-analytics lint-install \
        up down down-hard \
        e2e load

# ── proto ──────────────────────────────────────────────────────────────────
proto: proto-auth proto-inventory proto-sales proto-analytics

proto-auth:
	mkdir -p $(AUTH_OUT)
	protoc \
	  --go_out=$(AUTH_OUT) --go_opt=paths=source_relative \
	  --go-grpc_out=$(AUTH_OUT) --go-grpc_opt=paths=source_relative \
	  -I $(PROTO_DIR) \
	  $(PROTO_DIR)/auth/auth.proto

proto-inventory:
	mkdir -p $(INVENTORY_OUT)
	protoc \
	  --go_out=$(INVENTORY_OUT) --go_opt=paths=source_relative \
	  --go-grpc_out=$(INVENTORY_OUT) --go-grpc_opt=paths=source_relative \
	  -I $(PROTO_DIR) \
	  $(PROTO_DIR)/inventory/inventory.proto

proto-sales:
	mkdir -p $(SALES_OUT)
	protoc \
	  --go_out=$(SALES_OUT) --go_opt=paths=source_relative \
	  --go-grpc_out=$(SALES_OUT) --go-grpc_opt=paths=source_relative \
	  -I $(PROTO_DIR) \
	  $(PROTO_DIR)/auth/auth.proto \
	  $(PROTO_DIR)/inventory/inventory.proto \
	  $(PROTO_DIR)/sales/sales.proto

proto-analytics:
	mkdir -p $(ANALYTICS_OUT)
	protoc \
	  --go_out=$(ANALYTICS_OUT) --go_opt=paths=source_relative \
	  --go-grpc_out=$(ANALYTICS_OUT) --go-grpc_opt=paths=source_relative \
	  -I $(PROTO_DIR) \
	  $(PROTO_DIR)/auth/auth.proto \
	  $(PROTO_DIR)/inventory/inventory.proto \
	  $(PROTO_DIR)/analytics/analytics.proto

# ── build ──────────────────────────────────────────────────────────────────
build: build-auth build-inventory build-sales build-analytics

build-auth:
	cd auth && go build -o ../bin/auth ./...

build-inventory:
	cd inventory && go build -o ../bin/inventory ./...

build-sales:
	cd sales && go build -o ../bin/sales ./...

build-analytics:
	cd analytics && go build -o ../bin/analytics ./...

# ── test ───────────────────────────────────────────────────────────────────
test: test-auth test-inventory test-sales test-analytics

test-auth:
	cd auth && go test ./... -v -race -cover

test-inventory:
	cd inventory && go test ./... -v -race -cover

test-sales:
	cd sales && go test ./... -v -race -cover

test-analytics:
	cd analytics && go test ./... -v -race -cover

# ── migrate ────────────────────────────────────────────────────────────────
migrate-auth:
	psql "$(AUTH_DSN)" -f auth/migrations/001_init.sql

migrate-inventory:
	psql "$(INVENTORY_DSN)" -f inventory/migrations/001_init.sql

migrate-sales:
	psql "$(SALES_DSN)" -f sales/migrations/001_init.sql

migrate-analytics:
	psql "$(ANALYTICS_DSN)" -f analytics/migrations/postgres/001_init.sql
	clickhouse-client --host "$(CLICKHOUSE_HOST)" \
	  --query "$$(cat analytics/migrations/clickhouse/001_init.sql)"

# ── lint ───────────────────────────────────────────────────────────────────
# golangci-lint работает по go.mod; в проекте каждый сервис — это отдельный
# Go-модуль, поэтому запускаем его четыре раза в нужных директориях.
lint: lint-auth lint-inventory lint-sales lint-analytics

lint-auth:
	cd auth      && golangci-lint run --timeout 5m ./...

lint-inventory:
	cd inventory && golangci-lint run --timeout 5m ./...

lint-sales:
	cd sales     && golangci-lint run --timeout 5m ./...

lint-analytics:
	cd analytics && golangci-lint run --timeout 5m ./...

# Установка golangci-lint в $GOPATH/bin (нужно один раз).
lint-install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# ── docker ─────────────────────────────────────────────────────────────────
up:
	docker compose up --build -d

down:
	docker compose down

down-hard:
	docker compose down -v

# ── e2e / load ─────────────────────────────────────────────────────────────
# E2E-сценарии: ожидается, что контейнеры уже подняты (`make up`).
# По умолчанию делает 6 продаж и опрашивает аналитику.
e2e:
	cd e2e && go run . -sales-count 6

# 15-минутная нагрузка для дашбордов Grafana (см. http://localhost:3000).
load:
	cd loadgen && go run . -duration 15m
