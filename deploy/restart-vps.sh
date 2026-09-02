#!/usr/bin/env bash
# Reinicia infra Docker + API Go + Next no VPS.
# Uso: sudo bash /root/thedobrabi/deploy/restart-vps.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ -f "$ROOT/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/.env"
  set +a
fi

API_ADDR="${APP_HTTP_ADDR:-:2003}"
API_PORT="${API_ADDR#:}"
WEB_PORT="${WEB_PORT:-13010}"
API_BIN="${API_BIN:-/usr/local/bin/thedobra-api}"

echo "==> Docker Compose"
docker compose -f "$ROOT/docker-compose.yml" up -d
docker compose -f "$ROOT/docker-compose.yml" ps

echo "==> Portas publicadas (têm de bater com POSTGRES_DSN / REDIS_ADDR no .env)"
docker compose -f "$ROOT/docker-compose.yml" ps --format '{{.Name}} {{.Ports}}'

echo "==> A esperar Postgres / Redis / ClickHouse"
for i in $(seq 1 40); do
  if docker compose -f "$ROOT/docker-compose.yml" ps --status running | grep -q postgres \
    && docker compose -f "$ROOT/docker-compose.yml" ps --status running | grep -q redis; then
    break
  fi
  sleep 1
done
sleep 2

if systemctl list-unit-files | grep -q '^thedobra-api.service'; then
  echo "==> systemctl restart thedobra-api"
  systemctl reset-failed thedobra-api || true
  systemctl restart thedobra-api
else
  echo "==> a arrancar API ($API_BIN) em $API_ADDR"
  if [[ ! -x "$API_BIN" ]]; then
    echo "falta $API_BIN — corre: cd $ROOT/services/api && go build -o $API_BIN ./cmd/api" >&2
    exit 1
  fi
  pkill -f "$API_BIN" || true
  nohup env APP_HTTP_ADDR="$API_ADDR" "$API_BIN" >>/var/log/thedobra-api.log 2>&1 &
fi

if systemctl list-unit-files | grep -q '^thedobra-web.service'; then
  echo "==> systemctl restart thedobra-web"
  systemctl reset-failed thedobra-web || true
  systemctl restart thedobra-web
else
  echo "==> a arrancar Next na porta $WEB_PORT"
  pkill -f "next start" || true
  cd "$ROOT/apps/web"
  nohup env NODE_ENV=production PORT="$WEB_PORT" API_PROXY_URL="http://127.0.0.1:${API_PORT}" \
    npm run start >>/var/log/thedobra-web.log 2>&1 &
fi

echo "==> A esperar listen"
ok=0
for i in $(seq 1 20); do
  if curl -fsS "http://127.0.0.1:${API_PORT}/healthz" >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 1
done

echo "==> ss"
ss -lptn | grep -E ":${API_PORT}|:${WEB_PORT}" || true

if [[ "$ok" -eq 1 ]]; then
  echo "API healthz: $(curl -sS "http://127.0.0.1:${API_PORT}/healthz")"
else
  echo "API ainda não responde em :${API_PORT}" >&2
  if systemctl list-unit-files | grep -q '^thedobra-api.service'; then
    journalctl -u thedobra-api -n 40 --no-pager || true
  else
    tail -40 /var/log/thedobra-api.log || true
  fi
  exit 1
fi

echo "Pronto. Site: https://app.thedobra.cc"
