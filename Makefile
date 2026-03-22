
.PHONY: proto up down

proto:
	mkdir -p auth/gen/auth
	protoc \
		--go_out=auth/gen/auth \
		--go_opt=paths=source_relative \
		--go-grpc_out=auth/gen/auth \
		--go-grpc_opt=paths=source_relative \
		-I proto \
		proto/auth/auth.proto

up:
	docker-compose up --build -d

down:
	docker-compose down -v

# выключить контейнер без очистки данных - soft-down
s-down:
	docker-compose down

# Запуск всех тестов
t:
	cd auth && go test ./... -v -count=1

test-cover:
	cd auth && go test ./... -coverprofile=coverage.out -count=1
	cd auth && go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: auth/coverage.html"