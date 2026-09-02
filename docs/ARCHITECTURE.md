# TheDobra architecture

## Shape

Modular monolith in Go + independent workers + event bus + separate analytics database.

This is intentional. Microservices are extracted when a bounded context (ingestion, query, AI) needs to scale independently — not before.

```
Users → Next.js → Go API
                    ├ PostgreSQL (tenancy, semantic models, dashboards, audit)
                    ├ Redis (query cache, rate limits)
                    ├ ClickHouse (facts)
                    ├ MinIO (Parquet lake: raw/bronze/silver/gold)
                    └ Redpanda (versioned, idempotent events)
Python workers consume Redis `thedobra:ml:jobs` for forecast/anomaly.
```

## Tenant isolation

Every request JWT carries `org_id` + `workspace_id`. Dataset tables in ClickHouse always filter `_tenant = org_id`. Object keys are prefixed with `company_id=`. Cache keys include tenant. There is no cross-tenant SQL path.

## Query engine

User/LLM never executes arbitrary SQL.

```
Ask / widget → semantic request → official measures/dimensions → validated SQL → timeout + row limit → Redis fingerprint cache → ClickHouse
```

If a metric is not in the semantic model, the engine refuses rather than inventing a formula.

## Ingestion

CSV/XLSX upload and Postgres/MySQL connectors stream in batches. Schema is inferred, quality scored, a semantic model suggested, then rows are inserted into a MergeTree table partitioned by month.

## AI trust

Every Dobra AI answer includes evidence:

- source dataset
- official metric + expression
- period
- comparison window when relevant

Insufficient data returns: `I don't have enough data to answer this reliably.`

## Growth

ClickHouse tables are per-dataset MergeTree with tenant column. Adding shards later is a cluster topology change, not an application rewrite. Workers and the API are already independently deployable.

## Phases

1. Foundation (this repo): auth, tenancy, ingest, query, semantic, dashboards, Ask TheDobra
2. MQTT/EMQX, streaming, forecasting workers, alerts fan-out
3. Kubernetes HPA, SSO/SAML/SCIM, CDC, lineage UI
