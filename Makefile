PROTO_DIR     := proto
AUTH_OUT      := auth/gen/
INVENTORY_OUT := inventory/gen/
SALES_OUT     := sales/gen/

.PHONY: proto proto-auth proto-inventory proto-sales \
        build build-auth build-inventory build-sales \
        test test-auth test-inventory test-sales \
        migrate-auth migrate-inventory \
        lint up down down-hard

# ── proto ──────────────────────────────────────────────────────────────────
proto: proto-auth proto-inventory proto-sales

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

# ── build ──────────────────────────────────────────────────────────────────
build: build-auth build-inventory build-sales

build-auth:
	cd auth && go build -o ../bin/auth ./...

build-inventory:
	cd inventory && go build -o ../bin/inventory ./...

build-sales:
	cd sales && go build -o ../bin/sales ./...

# ── test ───────────────────────────────────────────────────────────────────
t: test-auth test-inventory test-sales

test-auth:
	cd auth && go test ./... -v -race -cover

test-inventory:
	cd inventory && go test ./... -v -race -cover

test-sales:
	cd sales && go test ./... -v -race -cover

# ── migrate ────────────────────────────────────────────────────────────────
migrate-auth:
	psql "$(AUTH_DSN)" -f auth/migrations/001_init.sql

migrate-inventory:
	psql "$(INVENTORY_DSN)" -f inventory/migrations/001_init.sql

migrate-sales:
	psql "$(SALES_DSN)" -f sales/migrations/001_init.sql

# ── lint ───────────────────────────────────────────────────────────────────
lint:
	golangci-lint run ./auth/... ./inventory/... ./sales/...

# ── docker ─────────────────────────────────────────────────────────────────
up:
	docker compose up --build -d

down:
	docker compose down

down-hard:
	docker compose down -v
