#!/usr/bin/env bash
# Runs every backend service as a local process (no Docker) against
# infrastructure you already have running: PostgreSQL (with the identity,
# world, character and item databases), Dragonfly/Redis, NATS with
# JetStream, Centrifugo and MongoDB. Logs go to .data/logs/<service>.log.
#
#   ./scripts/run-local.sh          # build + start all services
#   ./scripts/run-local.sh stop     # stop them
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LOGS="$ROOT/.data/logs"
PIDS="$ROOT/.data/pids"
mkdir -p "$LOGS" "$PIDS"

SERVICES=(identity world sprite character item asset ton presence combat dungeon history gateway)

if [[ "${1:-}" == "stop" ]]; then
  for s in "${SERVICES[@]}"; do
    [[ -f "$PIDS/$s" ]] && kill "$(cat "$PIDS/$s")" 2>/dev/null || true
    rm -f "$PIDS/$s"
  done
  exit 0
fi

(cd "$ROOT/backend" && for s in "${SERVICES[@]}"; do go build -o "bin/$s" "./services/$s"; done)

export NATS_URL="${NATS_URL:-nats://localhost:4222}"
export DRAGONFLY_URL="${DRAGONFLY_URL:-redis://localhost:6379/0}"
export MONGO_URI="${MONGO_URI:-mongodb://localhost:27017}"
export CENTRIFUGO_API_URL="${CENTRIFUGO_API_URL:-http://localhost:8000}"
export DEV_LOGIN="${DEV_LOGIN:-true}"
export SPRITE_CATALOG="$ROOT/assets/lpc/catalog.json"
export SPRITE_CACHE_DIR="$ROOT/.data/sprite-cache"
PG="${PG_BASE:-postgres://ommrpg:ommrpg@localhost:5432}"

port=8101
for s in "${SERVICES[@]}"; do
  db=""
  case "$s" in identity|world|character|item|asset) db="POSTGRES_DSN=$PG/$s?sslmode=disable";; esac
  env HEALTH_ADDR=":$port" $db "$ROOT/backend/bin/$s" > "$LOGS/$s.log" 2>&1 &
  echo $! > "$PIDS/$s"
  echo "started $s (pid $!, health :$port)"
  port=$((port+1))
done
