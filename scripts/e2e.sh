#!/usr/bin/env bash
# scripts/e2e.sh — E2E-смоук для macOS (bash 3.2+; системный bash на macOS подходит).
#
# Требования:
#   • Docker Desktop для Mac (или Colima / OrbStack) с поддержкой `docker compose`
#   • Go 1.24+ (например: brew install go)
#
# Первый запуск после клонирования репозитория:
#   chmod +x scripts/e2e.sh scripts/load.sh
#
# Использование:
#   ./scripts/e2e.sh                          # docker compose up + тесты
#   SKIP_UP=1 ./scripts/e2e.sh                # контейнеры уже подняты — только go run
#   SALES_COUNT=12 POLL_TIMEOUT=45s ./scripts/e2e.sh
#
# Переменные окружения:
#   SKIP_UP       — "1" не вызывать docker compose
#   SALES_COUNT   — число продаж для сценария аналитики (по умолчанию 6)
#   WAIT_SECONDS  — пауза после compose перед E2E (по умолчанию 60)
#   POLL_TIMEOUT  — длительность опроса отчётов, например 30s, 1m (по умолчанию 30s)

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

SKIP_UP="${SKIP_UP:-0}"
SALES_COUNT="${SALES_COUNT:-6}"
WAIT_SECONDS="${WAIT_SECONDS:-5}"
POLL_TIMEOUT="${POLL_TIMEOUT:-30s}"

need_go() {
	if ! command -v go >/dev/null 2>&1; then
		echo "error: нужен Go в PATH (brew install go)" >&2
		exit 127
	fi
}

need_compose() {
	if ! command -v docker >/dev/null 2>&1; then
		echo "error: нужен Docker / Docker Desktop для Mac в PATH" >&2
		exit 127
	fi
	if ! docker compose version >/dev/null 2>&1; then
		echo "error: нужен плагин 'docker compose' (обновите Docker Desktop)" >&2
		exit 127
	fi
}

section() {
	printf '\n\033[1;36m═══ %s ═══\033[0m\n' "$1"
}

need_go

if [[ "$SKIP_UP" != "1" ]]; then
	need_compose
	section "docker compose up -d"
	(cd "$ROOT" && docker compose up -d --build)

	section "Ждём готовности сервисов (${WAIT_SECONDS}s)"
	sleep "$WAIT_SECONDS"
else
	printf '\033[1;33m(пропущен docker compose up — SKIP_UP=1)\033[0m\n'
fi

section "Запуск E2E"
cd "$ROOT/e2e"
if go run . -sales-count "$SALES_COUNT" -poll-timeout "$POLL_TIMEOUT"; then
	printf '\n\033[1;32mE2E завершилось успешно\033[0m\n'
	exit 0
else
	code=$?
	printf '\n\033[1;31mE2E завершилось с ошибками (exit=%s)\033[0m\n' "$code"
	exit "$code"
fi
