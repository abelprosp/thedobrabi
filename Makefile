.PHONY: infra infra-down api web workers generator tidy test

infra:
	docker compose up -d postgres redis clickhouse minio redpanda
	@echo "waiting for postgres..."
	@until docker compose exec -T postgres pg_isready -U thedobra >/dev/null 2>&1; do sleep 1; done
	@echo "infra ready"

infra-down:
	docker compose down

api:
	cd services/api && go run ./cmd/api

web:
	cd apps/web && npm run dev

workers:
	cd workers/analytics && python3 main.py

generator:
	cd data/generators && go run . --help

tidy:
	cd services/api && go mod tidy
	cd data/generators && go mod tidy

test:
	cd services/api && go test ./...
	cd data/generators && go test ./...

dev: infra
	@echo "Start API:  make api"
	@echo "Start Web:  make web"
