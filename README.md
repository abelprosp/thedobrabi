# TheDobra

AI-Native Business Analytics Platform.

> Connect your data. Understand your business. Act with intelligence.

## Stack

- **Go** modular monolith (`services/api`) — auth, tenancy, ingestion, query engine, semantic layer, AI agent
- **PostgreSQL** — control plane
- **ClickHouse** — analytical store
- **Redis** — query cache / sessions
- **MinIO** — data lake (raw → gold)
- **Redpanda** — event backbone
- **Next.js** — `apps/web`
- **Python workers** — forecast / anomaly (not on the hot query path)

## Quick start

```bash
cp .env.example .env
make infra
cd services/api && go mod tidy && go run ./cmd/api
# another terminal
cd apps/web && npm install && npm run dev
```

Open http://localhost:3000 — create an organization, load demo sales data, ask TheDobra.

## Infra ports (VPS)

`docker compose` publishes Postgres on host **5432** by default (`POSTGRES_PORT`). If that port is already taken (typical on a VPS with another Postgres):

```bash
# see what owns 5432
ss -lptn | grep 5432
# or
docker ps
```

Set a free host port in `.env` (Compose interpolates it; the API DSN must match):

```
# VPS: se 5432 já estiver ocupado
POSTGRES_PORT=5433
POSTGRES_DSN=postgres://thedobra:thedobra@127.0.0.1:5433/thedobra?sslmode=disable
```

Then:

```bash
docker compose down
docker compose up -d
docker compose ps
```

Other published host ports (same pattern if they collide): `REDIS_PORT` (6379), `CLICKHOUSE_HTTP_PORT` (8123), `CLICKHOUSE_NATIVE_PORT` (9009), `MINIO_API_PORT` (9010), `MINIO_CONSOLE_PORT` (9011), `KAFKA_PORT` (9092). Redis 6379 and Kafka 9092 are the next most common conflicts.

API and web are **not** in this compose — they talk to infra via `localhost`. If you later run the API in the same Compose network, you can omit `ports` on Postgres and use hostname `postgres` in `POSTGRES_DSN`.

## Demo path (acceptance)

1. Sign up (creates org + workspace)
2. Load demo sales dataset **or** upload CSV / connect Postgres
3. Schema + data quality + semantic model are generated
4. Query via semantic layer (never raw SQL from the user/LLM)
5. Build a dashboard (manual or Build with AI)
6. Ask TheDobra — answers include evidence (source, metric, period, formula)
7. Insights, forecast baseline, alerts, executive report

## Architecture

See `docs/ARCHITECTURE.md`.
