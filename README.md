# Pharmacy — автоматизированная система фармацевтического учёта

Дипломный проект: распределённая (микросервисная) система учёта аптеки.
Покрывает базовые сценарии: аутентификация, ведение справочника товаров,
приёмка партий с контролем сроков годности, продажа с FEFO-списанием,
аналитические отчёты и прогноз спроса.

## Что и зачем

Бизнес-задача: дать аптеке инструмент, который одновременно:

- ведёт **актуальный остаток** по каждому SKU и партии;
- **контролирует сроки годности** (`expiring_soon`, автосписание `expired`);
- фиксирует **продажи и продавца** (имя пользователя из JWT) — для аудита и расчёта премий;
- даёт менеджеру **аналитику и прогноз спроса** (учёт сезонности, доля списаний).

Архитектура — микросервисы на Go, связанные синхронно по gRPC и асинхронно через Kafka.
Такое разделение позволило развивать сервисы независимо, держать БД, нагрузку и
схемы данных под каждое назначение разными (PostgreSQL для оперативного хранения,
Elasticsearch для поиска, ClickHouse для аналитики).

## Сервисы

| Сервис      | Назначение                                                                 | Порт  | Метрики |
|-------------|----------------------------------------------------------------------------|-------|---------|
| `auth`      | регистрация, логин, выпуск/валидация **JWT**, отзыв через Redis-whitelist  | 50051 | 9101    |
| `inventory` | продукты, партии (FEFO), приёмка, списание, низкие остатки, поиск          | 50053 | 9103    |
| `sales`     | оформление продаж, история, публикация событий `sales.completed`           | 50054 | 9104    |
| `analytics` | отчёты (sales/write-off/forecast/waste), фоновый воркер, потребитель Kafka | 50055 | 9105    |
| `gateway`   | Nginx, единый вход для клиента (gRPC-over-HTTP/2)                          | 8080  | —       |

Поддерживающие сервисы:

- **PostgreSQL** — отдельная БД на каждый сервис (auth/inventory/sales/analytics).
- **Redis** — whitelist активных JWT (`jti`) для возможности logout-а.
- **Elasticsearch** — поиск товаров/аналогов в `inventory`.
- **Kafka** — событийная шина (`sales.completed`, `inventory.received`, `inventory.written_off`).
- **ClickHouse** — событийное хранилище в `analytics`, источник данных для отчётов.
- **Prometheus + Grafana** — сбор и визуализация метрик.

## Стек

- Go 1.24, gRPC + protobuf, `go.uber.org/zap` для логов.
- PostgreSQL 16, Redis 7, Elasticsearch 8.13, Kafka (Bitnami), ClickHouse 24.3.
- Prometheus 2.55, Grafana 11.
- Линтер `golangci-lint` (см. `.golangci.yml`).

## Аутентификация

- Токены — **JWT (HS256)**.
- В payload хранятся `user_id`, `username`, `role` (`admin` / `manager` / `pharmacist`)
  и `jti` — идентификатор сессии.
- Redis работает как **whitelist** активных `jti`: при `Logout` запись удаляется,
  и токен мгновенно становится невалидным, даже если срок жизни ещё не истёк.
- Внутри других сервисов gRPC-интерсептор парсит токен, проверяет роль и кладёт
  `username` продавца в `context` — это значение и сохраняется в `sales.seller_username`.

## Метрики

Каждый сервис поднимает HTTP-эндпоинт `/metrics` (порты 9101/9103/9104/9105).
Собираются:

- общие gRPC-метрики (`*_grpc_requests_total`, `*_grpc_request_duration_seconds`);
- бизнес-метрики:
  - `auth_login_attempts_total{result=...}`, `auth_tokens_issued_total`, `auth_tokens_revoked_total`;
  - `sales_sales_created_total`, `sales_sale_total_amount` (гистограмма сумм);
  - `inventory_batches_received_total`, `inventory_batches_written_off_total`, `inventory_stock_deductions_total`;
  - `analytics_reports_created_total{type}`, `analytics_reports_completed_total{type,status}`,
    `analytics_kafka_events_consumed_total{topic}`.

Преднастроенный дашборд лежит в `monitoring/grafana/dashboards/pharmacy-overview.json`.

## Как запустить

```bash
make up                       # docker compose up --build -d (всё разом)
# или по частям:
make build                    # бинарники в bin/
make test                     # юнит-тесты во всех сервисах
make lint                     # golangci-lint по всем сервисам
make lint-install             # один раз поставить линтер
```

После `make up`:

- gRPC-шлюз — `localhost:8080`;
- Prometheus UI — `http://localhost:9090`;
- Grafana — `http://localhost:3000` (login/password: `admin` / `admin`),
  дашборд **Pharmacy / Pharmacy Overview** уже привязан.

## Структура репозитория

```
auth/        — сервис аутентификации (JWT + Redis whitelist)
inventory/   — товары, партии, остатки, FEFO-списание, ES-поиск
sales/       — оформление продаж, история, публикация событий
analytics/   — отчёты, прогноз спроса, фоновый воркер, ClickHouse
gateway/     — nginx-конфиг для gRPC-over-HTTP
proto/       — общие .proto файлы (исходники для всех сервисов)
monitoring/  — конфиги Prometheus и Grafana (provisioning + dashboard)
scripts/     — e2e.sh и load.sh для macOS (bash)
docker-compose.yml — локальный запуск всей системы
Makefile     — proto / build / test / lint / migrate / up
.golangci.yml — конфиг линтера
```

## Тестирование

Юнит-тесты написаны в стиле **table-driven** (`testify` + `sqlmock` + `miniredis`)
и помечены `t.Parallel()` — суммарный прогон занимает считанные секунды.
Покрываются:

- доменные правила (`Sale`, `Batch`, `Product`, `StockItem`);
- use-case-сценарии с моками репозиториев и внешних клиентов;
- адаптеры PostgreSQL — на ожидаемые SQL-запросы и сканирование;
- gRPC-handlers — на корректность маппинга в коды gRPC.

Запуск: `make test` или `go test ./...` в нужном сервисе.

## E2E-сценарии и нагрузка

В корне репозитория лежат два самостоятельных Go-модуля, которые ходят
в **запущенные контейнеры по gRPC** и эмулируют живых клиентов.

### `e2e/` — smoke-сценарии

Прогоняет полный путь *auth → inventory → sales → analytics*:
регистрация пользователей, создание каталога, приёмка партий, продажи (с проверкой
`seller_username` из JWT), запросы остатков и отчётов аналитики (poll до READY).
Если данные не совпадают, выводит сообщение и продолжает выполнение, а в конце
печатает разноцветный отчёт.

```bash
make up                        # 1. поднять контейнеры
make e2e                       # 2. прогнать сценарии (только Go, compose уже поднят)

# Или один скрипт под macOS (bash): compose + ожидание + E2E
chmod +x scripts/e2e.sh scripts/load.sh
./scripts/e2e.sh
SKIP_UP=1 ./scripts/e2e.sh     # если `docker compose up` уже выполнен
```

### `loadgen/` — лёгкая нагрузка для дашбордов Grafana

Запускает несколько горутин, которые в течение **15 минут** дёргают auth,
inventory, sales, analytics с настраиваемым RPS, чтобы метрики на дашбордах
показывали реальные графики.

```bash
make load                       # 15 минут, дефолтный RPS (нужен запущенный compose)
DURATION=5m RPS=8 ./scripts/load.sh   # 5 минут и выше RPS
```

Где смотреть метрики, пока крутится нагрузка:

| Где                        | URL                                                                  |
|----------------------------|----------------------------------------------------------------------|
| **Grafana** (admin/admin)  | <http://localhost:3000> → *Dashboards → Pharmacy → Pharmacy — Service Overview* |
| Prometheus UI              | <http://localhost:9090>                                              |
| Сырые `/metrics` сервисов  | `http://localhost:9101/metrics` (auth), `:9103` (inventory), `:9104` (sales), `:9105` (analytics) |
