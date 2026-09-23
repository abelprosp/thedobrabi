#!/usr/bin/env bash
# Diagnostica e tenta corrigir 502 Bad Gateway (nginx → API/web) no VPS.
# Uso: sudo bash /root/thedobrabi/deploy/fix-502.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ -f "$ROOT/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/.env"
  set +a
else
  echo "AVISO: não existe $ROOT/.env — copie de .env.example e ajuste." >&2
fi

API_ADDR="${APP_HTTP_ADDR:-127.0.0.1:2003}"
# Normaliza ":2003" → "127.0.0.1:2003" (evita API em 0.0.0.0 sem o nginx achar em 127.0.0.1)
if [[ "$API_ADDR" == :* ]]; then
  API_ADDR="127.0.0.1${API_ADDR}"
fi
API_PORT="${API_ADDR##*:}"
WEB_PORT="${WEB_PORT:-13010}"
API_BIN="${API_BIN:-/usr/local/bin/thedobra-api}"
REDIS_PASSWORD="${REDIS_PASSWORD:-thedobra-redis-local}"
REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:16379}"

echo "=== TheDobra fix-502 ==="
echo "ROOT=$ROOT"
echo "APP_ENV=${APP_ENV:-?} APP_HTTP_ADDR=$API_ADDR WEB_PORT=$WEB_PORT"
echo "REDIS_ADDR=$REDIS_ADDR REDIS_PASSWORD=${REDIS_PASSWORD:+(definida)}"
echo

echo "==> 1) Portas em escuta"
ss -lptn 2>/dev/null | grep -E ":${API_PORT}|:${WEB_PORT}|:80|:443" || true
echo

echo "==> 2) Health directo (bypass nginx)"
if curl -fsS --max-time 3 "http://127.0.0.1:${API_PORT}/healthz"; then
  echo
  echo "API responde em 127.0.0.1:${API_PORT} — o 502 pode ser nginx a apontar para porta errada."
else
  echo
  echo "API NÃO responde em 127.0.0.1:${API_PORT} (causa típica do 502)."
fi
echo

echo "==> 3) Últimos logs da API"
if systemctl list-unit-files 2>/dev/null | grep -q '^thedobra-api.service'; then
  systemctl status thedobra-api --no-pager -l || true
  journalctl -u thedobra-api -n 50 --no-pager || true
else
  tail -50 /var/log/thedobra-api.log 2>/dev/null || echo "(sem /var/log/thedobra-api.log)"
fi
echo

echo "==> 4) Garantir REDIS_PASSWORD no .env (obrigatório com Redis requirepass)"
if [[ ! -f "$ROOT/.env" ]]; then
  cp "$ROOT/.env.example" "$ROOT/.env"
  echo "Criado .env a partir de .env.example — EDITE secrets antes de produção real."
fi
if ! grep -q '^REDIS_PASSWORD=' "$ROOT/.env" 2>/dev/null; then
  echo "REDIS_PASSWORD=${REDIS_PASSWORD}" >>"$ROOT/.env"
  echo "Acrescentei REDIS_PASSWORD ao .env"
elif grep -q '^REDIS_PASSWORD=$' "$ROOT/.env" 2>/dev/null; then
  sed -i "s/^REDIS_PASSWORD=$/REDIS_PASSWORD=${REDIS_PASSWORD}/" "$ROOT/.env"
  echo "Preenchi REDIS_PASSWORD vazio no .env"
fi
# Re-source após possível edição
set -a
# shellcheck disable=SC1091
source "$ROOT/.env"
set +a
REDIS_PASSWORD="${REDIS_PASSWORD:-thedobra-redis-local}"

# Alinha APP_HTTP_ADDR para loopback se estiver só com :PORT
if grep -qE '^APP_HTTP_ADDR=:' "$ROOT/.env" 2>/dev/null; then
  PORT_ONLY="$(grep -E '^APP_HTTP_ADDR=:' "$ROOT/.env" | head -1 | cut -d= -f2)"
  sed -i "s|^APP_HTTP_ADDR=${PORT_ONLY}|APP_HTTP_ADDR=127.0.0.1${PORT_ONLY}|" "$ROOT/.env"
  echo "Ajustei APP_HTTP_ADDR para 127.0.0.1${PORT_ONLY}"
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/.env"
  set +a
  API_ADDR="${APP_HTTP_ADDR:-127.0.0.1:2003}"
  API_PORT="${API_ADDR##*:}"
fi

echo "==> 5) Docker Compose (Postgres/Redis/ClickHouse)"
docker compose -f "$ROOT/docker-compose.yml" up -d
sleep 3
docker compose -f "$ROOT/docker-compose.yml" ps
echo

echo "==> 6) Teste Redis com password"
REDIS_HOST="${REDIS_ADDR%%:*}"
REDIS_PORT_NUM="${REDIS_ADDR##*:}"
if docker compose -f "$ROOT/docker-compose.yml" exec -T redis \
  redis-cli -a "$REDIS_PASSWORD" ping 2>/dev/null | grep -q PONG; then
  echo "Redis OK (auth)"
else
  echo "Redis auth falhou — a reiniciar redis com REDIS_PASSWORD do .env..."
  docker compose -f "$ROOT/docker-compose.yml" up -d --force-recreate redis
  sleep 2
  docker compose -f "$ROOT/docker-compose.yml" exec -T redis \
    redis-cli -a "$REDIS_PASSWORD" ping || echo "Ainda falhou: confira REDIS_PASSWORD no compose e no .env" >&2
fi
echo

echo "==> 7) Rebuild API se o binário faltar ou for antigo"
NEED_BUILD=0
if [[ ! -x "$API_BIN" ]]; then
  NEED_BUILD=1
elif [[ "$ROOT/services/api/cmd/api/main.go" -nt "$API_BIN" ]]; then
  NEED_BUILD=1
fi
# Also rebuild if source tree is newer than binary (security fixes, Validate, etc.)
if [[ -x "$API_BIN" ]] && find "$ROOT/services/api" -name '*.go' -newer "$API_BIN" | grep -q .; then
  NEED_BUILD=1
fi
if [[ "$NEED_BUILD" -eq 1 ]]; then
  echo "A compilar API → $API_BIN"
  (cd "$ROOT/services/api" && go build -o "$API_BIN" ./cmd/api)
fi

echo "==> 7b) Teste de arranque (mostra erro de Validate/Redis/DSN)"
set +e
timeout 4 bash -c "set -a; source '$ROOT/.env'; set +a; export APP_HTTP_ADDR='$API_ADDR'; exec '$API_BIN'" \
  >/tmp/thedobra-api-boot.out 2>&1
boot_rc=$?
set -e
if [[ -s /tmp/thedobra-api-boot.out ]]; then
  echo "--- saída do arranque ---"
  cat /tmp/thedobra-api-boot.out
  echo "-------------------------"
fi
if [[ "$boot_rc" -eq 124 ]]; then
  echo "API manteve-se a correr ~4s (provável OK). A matar o teste..."
  pkill -f "$API_BIN" || true
  sleep 1
elif [[ "$boot_rc" -ne 0 ]]; then
  echo "API saiu com código $boot_rc — corrija o .env com base na saída acima." >&2
fi

echo "==> 8) Reinício completo"
bash "$ROOT/deploy/restart-vps.sh"

echo
echo "==> 9) Verificação via paths públicos locais"
curl -fsS --max-time 5 "http://127.0.0.1:${API_PORT}/healthz" && echo "  API healthz OK"
curl -fsS --max-time 5 -o /dev/null -w "  web :%{http_code}\n" "http://127.0.0.1:${WEB_PORT}/" || echo "  web sem resposta (Next)"
curl -fsS --max-time 5 -o /dev/null -w "  nginx /api healthz via 80: %{http_code}\n" \
  "http://127.0.0.1/healthz" 2>/dev/null || echo "  (nginx local pode exigir Host app.thedobra.cc)"

echo
echo "Se ainda houver 502:"
echo "  1) journalctl -u thedobra-api -n 80 --no-pager"
echo "  2) Confirme que nginx faz proxy para 127.0.0.1:${API_PORT}"
echo "  3) Em produção, APP_ENV=production exige JWT_SECRET/ENCRYPTION_KEY/REDIS_PASSWORD fortes"
echo "Pronto. Teste: https://app.thedobra.cc"
