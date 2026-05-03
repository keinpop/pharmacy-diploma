#!/usr/bin/env bash
# scripts/load.sh — лёгкая нагрузка для дашбордов Grafana (macOS, bash 3.2+).
#
# Контейнеры должны быть уже подняты (./scripts/e2e.sh или make up).
#
# Требования: Go 1.24+ (brew install go).
#
# Первый запуск:
#   chmod +x scripts/e2e.sh scripts/load.sh
#
# Использование:
#   ./scripts/load.sh                              # 15 минут по умолчанию
#   DURATION=5m ./scripts/load.sh                 # 5 минут
#   RPS=8 SALES_WORKERS=4 ./scripts/load.sh       # выше интенсивность
#
# Переменные окружения:
#   DURATION, RPS, SALES_WORKERS, READ_WORKERS — см. loadgen/main.go (-h)

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

DURATION="${DURATION:-10m}"
RPS="${RPS:-8}"
SALES_WORKERS="${SALES_WORKERS:-2}"
READ_WORKERS="${READ_WORKERS:-2}"

if ! command -v go >/dev/null 2>&1; then
	echo "error: нужен Go в PATH (brew install go)" >&2
	exit 127
fi

section() { printf '\n\033[1;36m═══ %s ═══\033[0m\n' "$1"; }

section "Где смотреть метрики (на Mac — в браузере)"
cat <<'EOF'
  Grafana    : http://localhost:3000   (admin / admin)
  Prometheus : http://localhost:9090
  Дашборд    : Dashboards → Pharmacy → Pharmacy — Service Overview

Стек должен быть запущен: из корня репозитория выполни
  docker compose up -d
или один раз ./scripts/e2e.sh — затем в другом терминале ./scripts/load.sh
EOF

section "Запуск нагрузки на ${DURATION}"
cd "$ROOT/loadgen"
exec go run . \
	-duration "$DURATION" \
	-rps "$RPS" \
	-sales-workers "$SALES_WORKERS" \
	-read-workers "$READ_WORKERS"
