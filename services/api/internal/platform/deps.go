package platform

import (
	"context"
	"log/slog"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"github.com/thedobra/thedobra/services/api/internal/config"
)

type EventBus interface {
	Publish(ctx context.Context, topic string, event Event) error
	Close() error
}

type Event struct {
	ID             string         `json:"id"`
	Type           string         `json:"type"`
	Version        int            `json:"version"`
	OrgID          string         `json:"org_id"`
	WorkspaceID    string         `json:"workspace_id"`
	Timestamp      string         `json:"timestamp"`
	IdempotencyKey string         `json:"idempotency_key"`
	Payload        map[string]any `json:"payload"`
}

type Deps struct {
	Cfg    config.Config
	Log    *slog.Logger
	PG     *pgxpool.Pool
	CH     driver.Conn
	Redis  *redis.Client
	Minio  *minio.Client
	Bus    EventBus
}
