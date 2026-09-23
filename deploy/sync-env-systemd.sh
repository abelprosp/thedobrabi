#!/usr/bin/env bash
# Gera /etc/thedobra/api.env a partir de .env (formato compatível com systemd EnvironmentFile).
# Uso: sudo bash deploy/sync-env-systemd.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SRC="${1:-$ROOT/.env}"
DEST="${2:-/etc/thedobra/api.env}"

if [[ ! -f "$SRC" ]]; then
  echo "em falta: $SRC" >&2
  exit 1
fi

mkdir -p "$(dirname "$DEST")"
tmp="$(mktemp)"

# systemd EnvironmentFile: KEY=VALUE, sem export, sem aspas problemáticas, sem espaços à volta do =
# Remove BOM/CRLF e linhas inválidas.
python3 - "$SRC" "$tmp" <<'PY'
import sys
src, dst = sys.argv[1], sys.argv[2]
out = []
with open(src, "rb") as f:
    raw = f.read()
if raw.startswith(b"\xef\xbb\xbf"):
    raw = raw[3:]
text = raw.decode("utf-8", errors="replace").replace("\r\n", "\n").replace("\r", "\n")
for line in text.split("\n"):
    s = line.strip()
    if not s or s.startswith("#"):
        continue
    if s.startswith("export "):
        s = s[7:].strip()
    if "=" not in s:
        continue
    k, v = s.split("=", 1)
    k = k.strip()
    v = v.strip()
    if len(v) >= 2 and ((v[0] == v[-1] == '"') or (v[0] == v[-1] == "'")):
        v = v[1:-1]
    if not k or not k.replace("_", "").isalnum() or k[0].isdigit():
        continue
    out.append(f"{k}={v}")
with open(dst, "w", encoding="utf-8") as f:
    f.write("\n".join(out) + "\n")
PY

grep -q '^REDIS_PASSWORD=' "$tmp" || echo "REDIS_PASSWORD=thedobra-redis-local" >>"$tmp"
grep -q '^APP_HTTP_ADDR=' "$tmp" || echo "APP_HTTP_ADDR=127.0.0.1:2003" >>"$tmp"
sed -i -E 's/^APP_HTTP_ADDR=:([0-9]+)/APP_HTTP_ADDR=127.0.0.1:\1/' "$tmp"

# Corrige WEB_ORIGIN localhost → domínio de produção se APP_PUBLIC_URL ou default thedobra.
if grep -qiE '^WEB_ORIGIN=https?://(localhost|127\.0\.0\.1)(:[0-9]+)?' "$tmp"; then
  if grep -qE '^APP_PUBLIC_URL=https?://' "$tmp"; then
    # Prefer site origin derived from known public app host when WEB_ORIGIN is local.
    :
  fi
  if ! grep -q '^WEB_ORIGIN=https://app.thedobra.cc' "$tmp"; then
    sed -i -E 's|^WEB_ORIGIN=https?://(localhost|127\.0\.0\.1)(:[0-9]+)?|WEB_ORIGIN=https://app.thedobra.cc|' "$tmp"
    echo "AVISO: WEB_ORIGIN apontava para localhost — corrigido para https://app.thedobra.cc"
  fi
fi

install -m 600 "$tmp" "$DEST"
rm -f "$tmp"

echo "Escrito $DEST"
echo "--- chaves relevantes ---"
grep -E '^(APP_ENV|APP_HTTP_ADDR|REDIS_ADDR|REDIS_PASSWORD|POSTGRES_DSN|JWT_SECRET|ENCRYPTION_KEY)=' "$DEST" \
  | sed -E 's/(PASSWORD|SECRET|KEY|DSN)=.*/\1=***/'
