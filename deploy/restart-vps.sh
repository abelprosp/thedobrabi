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

# Bind loopback by default so the API is not exposed on all interfaces (Nginx proxies).
API_ADDR="${APP_HTTP_ADDR:-127.0.0.1:2003}"
if [[ "$API_ADDR" == :* ]]; then
  API_ADDR="127.0.0.1${API_ADDR}"
fi
API_PORT="${API_ADDR##*:}"
WEB_PORT="${WEB_PORT:-13010}"
API_BIN="${API_BIN:-/usr/local/bin/thedobra-api}"
REDIS_PASSWORD="${REDIS_PASSWORD:-thedobra-redis-local}"

if [[ "$WEB_PORT" != "13010" ]]; then
  echo "WEB_PORT=$WEB_PORT não é suportada neste deploy; a configuração nginx/systemd usa 13010." >&2
  exit 1
fi

# Export so child processes (systemctl EnvironmentFile or nohup) see required secrets.
export APP_HTTP_ADDR="$API_ADDR"
export REDIS_PASSWORD
export REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:16379}"
export APP_ENV="${APP_ENV:-production}"

build_next_atomic() {
  local web_root="$ROOT/apps/web"
  local current="$web_root/.next"
  local release=".next-release-$(date +%s)-$$"
  local previous="$web_root/.next.previous"

  echo "==> build Next.js atômico ($release)"
  (
    cd "$web_root"
    # O build ocorre fora de .next; portanto o serviço atual continua servindo
    # o bundle anterior até a troca final.
    NEXT_DIST_DIR="$release" npm run build

    # Assets têm nomes com hash. Manter os assets anteriores evita que HTML
    # cacheado de uma versão anterior gere ChunkLoadError após o deploy.
    if [[ -d "$current/static" ]]; then
      mkdir -p "$release/static"
      cp -a "$current/static/." "$release/static/"
    fi

    # Cada mv no mesmo filesystem é atômico para os leitores do diretório.
    rm -rf "$previous"
    if [[ -e "$current" || -L "$current" ]]; then
      mv "$current" "$previous"
    fi
    if ! mv "$release" "$current"; then
      [[ -e "$previous" ]] && mv "$previous" "$current"
      return 1
    fi
    rm -rf "$previous"
  )
}

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
  api_workdir="$(systemctl show -p WorkingDirectory --value thedobra-api 2>/dev/null || true)"
  if [[ -n "$api_workdir" && "$api_workdir" != "$ROOT" ]]; then
    echo "thedobra-api.service aponta para $api_workdir, mas este deploy está em $ROOT; recusando restart." >&2
    exit 1
  fi
  systemctl reset-failed thedobra-api || true
  # Garante env limpo antes do restart (NOAUTH se REDIS_PASSWORD faltar ao processo)
  if [[ -x "$ROOT/deploy/sync-env-systemd.sh" ]]; then
    bash "$ROOT/deploy/sync-env-systemd.sh" "$ROOT/.env" /etc/thedobra/api.env || true
    systemctl daemon-reload || true
  fi
  systemctl restart thedobra-api
else
  echo "==> a arrancar API ($API_BIN) em $API_ADDR"
  if [[ ! -x "$API_BIN" ]]; then
    echo "falta $API_BIN — corre: cd $ROOT/services/api && go build -o $API_BIN ./cmd/api" >&2
    exit 1
  fi
  pkill -f "$API_BIN" || true
  # Carrega o .env completo — sem REDIS_PASSWORD a API falha a arrancar (502 no nginx).
  nohup bash -c "set -a; [[ -f '$ROOT/.env' ]] && source '$ROOT/.env'; set +a; export APP_HTTP_ADDR='$API_ADDR' REDIS_PASSWORD='${REDIS_PASSWORD}'; exec '$API_BIN'" \
    >>/var/log/thedobra-api.log 2>&1 &
fi

if systemctl list-unit-files | grep -q '^thedobra-web.service'; then
  echo "==> systemctl restart thedobra-web"
  web_workdir="$(systemctl show -p WorkingDirectory --value thedobra-web 2>/dev/null || true)"
  if [[ -n "$web_workdir" && "$web_workdir" != "$ROOT/apps/web" ]]; then
    echo "thedobra-web.service aponta para $web_workdir, mas este deploy está em $ROOT/apps/web; recusando restart." >&2
    exit 1
  fi
  systemctl reset-failed thedobra-web || true
  build_next_atomic
  systemctl restart thedobra-web
else
  echo "==> a arrancar Next na porta $WEB_PORT"
  build_next_atomic
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
