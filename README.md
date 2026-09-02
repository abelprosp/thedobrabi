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
