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
WEB_SERVER="$ROOT/apps/web/.next/standalone/server.js"

if [[ "$WEB_PORT" != "13010" ]]; then
  echo "WEB_PORT=$WEB_PORT não é suportada neste deploy; a configuração nginx/systemd usa 13010." >&2
  exit 1
fi

# Export so child processes (systemctl EnvironmentFile or nohup) see required secrets.
export APP_HTTP_ADDR="$API_ADDR"
export REDIS_PASSWORD
export REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:16379}"
export APP_ENV="${APP_ENV:-production}"
if [[ "$APP_ENV" == "production" ]]; then
  case "${APP_PUBLIC_URL:-}" in
    https://app.thedobra.cc) ;;
    *) echo "APP_PUBLIC_URL deve ser https://app.thedobra.cc em produção." >&2; exit 1 ;;
  esac
  case "${WEB_ORIGIN:-}" in
    https://app.thedobra.cc) ;;
    *) echo "WEB_ORIGIN deve ser https://app.thedobra.cc em produção." >&2; exit 1 ;;
  esac
  export NEXT_PUBLIC_APP_URL="${NEXT_PUBLIC_APP_URL:-https://app.thedobra.cc}"
fi

web_process_is_owned() {
  local pid="$1"
  local cwd cmdline
  [[ "$pid" =~ ^[0-9]+$ && "$pid" != "1" && "$pid" != "$$" ]] || return 1
  [[ -r "/proc/$pid/cmdline" ]] || return 1
  cwd="$(readlink "/proc/$pid/cwd" 2>/dev/null || true)"
  cmdline="$(tr '\0' ' ' <"/proc/$pid/cmdline" 2>/dev/null || true)"
  [[ "$cwd" == "$ROOT/apps/web" ]] &&
    [[ "$cmdline" == *"$WEB_SERVER"* ]]
}

web_listener_pids() {
  ss -ltnpH "sport = :$WEB_PORT" 2>/dev/null |
    grep -oE 'pid=[0-9]+' | cut -d= -f2 | sort -u
}

stop_owned_processes() {
  local command_fragment="$1"
  local signal="${2:-TERM}"
  local pid args

  while read -r pid args; do
    [[ -n "$pid" && "$pid" != "$$" ]] || continue
    [[ "$args" == *"$command_fragment"* ]] || continue
    echo "==> a parar processo TheDobra pid=$pid ($command_fragment)"
    kill "-$signal" "$pid" 2>/dev/null || true
  done < <(ps -eo pid=,args=)
}

stop_owned_processes_and_wait() {
  local command_fragment="$1"
  local pid args
  stop_owned_processes "$command_fragment" TERM
  for _ in $(seq 1 10); do
    local found=0
    while read -r pid args; do
      [[ -n "$pid" && "$pid" != "$$" ]] || continue
      [[ "$args" == *"$command_fragment"* ]] || continue
      found=1
      break
    done < <(ps -eo pid=,args=)
    [[ "$found" -eq 0 ]] && return 0
    sleep 1
  done
  stop_owned_processes "$command_fragment" KILL
}

stop_owned_web_listeners() {
  local signal="${1:-TERM}"
  local pid
  while read -r pid; do
    [[ -n "$pid" ]] || continue
    if web_process_is_owned "$pid"; then
      echo "==> a parar processo web confirmado pid=$pid cwd=$ROOT/apps/web"
      kill "-$signal" "$pid" 2>/dev/null || true
    else
      echo "processo pid=$pid escuta :$WEB_PORT, mas não pertence a $ROOT/apps/web; recusando pará-lo." >&2
      return 1
    fi
  done < <(web_listener_pids)
}

stop_owned_web_listeners_and_wait() {
  stop_owned_web_listeners TERM
  for _ in $(seq 1 10); do
    [[ -z "$(web_listener_pids)" ]] && return 0
    sleep 1
  done
  stop_owned_web_listeners KILL
}

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
    if ! NEXT_DIST_DIR="$release" npm run build; then
      rm -rf "$release"
      return 1
    fi

    # Assets têm nomes com hash. Manter os assets anteriores evita que HTML
    # cacheado de uma versão anterior gere ChunkLoadError após o deploy.
    if [[ -d "$current/static" ]]; then
      mkdir -p "$release/static"
      cp -a "$current/static/." "$release/static/"
    fi

    # Cada mv no mesmo filesystem é atômico para os leitores do diretório.
    if ! compgen -G "$release/static/css/*.css" >/dev/null; then
      echo "build Next inválido: nenhum CSS em $release/static/css" >&2
      rm -rf "$release"
      return 1
    fi
    if ! compgen -G "$release/static/chunks/*.js" >/dev/null; then
      echo "build Next inválido: nenhum chunk JS em $release/static/chunks" >&2
      rm -rf "$release"
      return 1
    fi
    if [[ ! -s "$release/BUILD_ID" || ! -s "$release/standalone/server.js" ]]; then
      echo "build Next inválido: BUILD_ID ou standalone/server.js ausente" >&2
      rm -rf "$release"
      return 1
    fi
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
  stop_owned_processes_and_wait "$API_BIN"
  # Carrega o .env completo — sem REDIS_PASSWORD a API falha a arrancar (502 no nginx).
  nohup bash -c "set -a; [[ -f '$ROOT/.env' ]] && source '$ROOT/.env'; set +a; export APP_HTTP_ADDR='$API_ADDR' REDIS_PASSWORD='${REDIS_PASSWORD}'; exec '$API_BIN'" \
    >>/var/log/thedobra-api.log 2>&1 &
fi

if [[ -d /run/systemd/system ]]; then
  # A unit é parte deste deploy; não confiar numa cópia antiga já instalada.
  install -m 644 "$ROOT/deploy/systemd/thedobra-web.service" /etc/systemd/system/thedobra-web.service
  bash "$ROOT/deploy/sync-env-systemd.sh" "$ROOT/.env" /etc/thedobra/web.env
  systemctl daemon-reload
fi

if systemctl list-unit-files | grep -q '^thedobra-web.service'; then
  echo "==> systemctl restart thedobra-web"
  web_workdir="$(systemctl show -p WorkingDirectory --value thedobra-web 2>/dev/null || true)"
  if [[ -n "$web_workdir" && "$web_workdir" != "$ROOT/apps/web" ]]; then
    echo "thedobra-web.service aponta para $web_workdir, mas este deploy está em $ROOT/apps/web; recusando restart." >&2
    exit 1
  fi
  systemctl reset-failed thedobra-web || true
  # systemd não controla um Next antigo iniciado manualmente fora do cgroup.
  # Pare a unidade primeiro e remova apenas o standalone deste checkout.
  systemctl stop thedobra-web || true
  stop_owned_web_listeners_and_wait
  build_next_atomic
  systemctl start thedobra-web
else
  echo "==> a arrancar Next na porta $WEB_PORT"
  build_next_atomic
  stop_owned_web_listeners_and_wait
  cd "$ROOT/apps/web"
  nohup env NODE_ENV=production HOSTNAME=127.0.0.1 PORT="$WEB_PORT" \
    API_PROXY_URL="http://127.0.0.1:${API_PORT}" \
    node .next/standalone/server.js >>/var/log/thedobra-web.log 2>&1 &
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

web_ok=0
for i in $(seq 1 20); do
  if curl -fsS --max-time 2 "http://127.0.0.1:${WEB_PORT}/" >/dev/null 2>&1; then
    web_ok=1
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

if [[ "$web_ok" -ne 1 ]]; then
  echo "Web ainda não responde em :${WEB_PORT}" >&2
  if systemctl list-unit-files | grep -q '^thedobra-web.service'; then
    systemctl status thedobra-web --no-pager -l || true
    journalctl -u thedobra-web -n 40 --no-pager || true
  else
    tail -40 /var/log/thedobra-web.log || true
  fi
  exit 1
fi

echo "Pronto. Site: https://app.thedobra.cc"
