package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/thedobra/thedobra/services/api/internal/apihttp"
	"github.com/thedobra/thedobra/services/api/internal/config"
	"github.com/thedobra/thedobra/services/api/internal/db"
	"github.com/thedobra/thedobra/services/api/internal/events"
	"github.com/thedobra/thedobra/services/api/internal/platform"
	"github.com/thedobra/thedobra/services/api/migrations"
)

func main() {
	loadDotEnv("../../.env")
	loadDotEnv(".env")

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := config.Load()
	ctx := context.Background()

	pg, err := db.ConnectPostgres(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Error("postgres", "err", err)
		os.Exit(1)
	}
	defer pg.Close()

	if err := db.Migrate(ctx, pg, migrations.Files, log); err != nil {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}

	ch, err := db.ConnectClickHouse(ctx, cfg)
	if err != nil {
		log.Error("clickhouse", "err", err)
		os.Exit(1)
	}
	defer ch.Close()

	rdb, err := db.ConnectRedis(ctx, cfg.RedisAddr)
	if err != nil {
		log.Error("redis", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	minioClient, err := db.ConnectMinio(ctx, cfg)
	if err != nil {
		log.Warn("minio unavailable", "err", err)
	}

	bus := events.Connect(cfg.KafkaBrokers, log)
	defer bus.Close()

	deps := &platform.Deps{
		Cfg:   cfg,
		Log:   log,
		PG:    pg,
		CH:    ch,
		Redis: rdb,
		Minio: minioClient,
		Bus:   bus,
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           apihttp.New(deps),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("thedobra api listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http", "err", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	cctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(cctx)
}

func loadDotEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}
