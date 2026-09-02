package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func mongoURI(cfg SQLConfig) string {
	if strings.TrimSpace(cfg.URL) != "" {
		return strings.TrimSpace(cfg.URL)
	}
	port := cfg.Port
	if port == 0 {
		port = 27017
	}
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}
	u := &url.URL{Scheme: "mongodb", Host: fmt.Sprintf("%s:%d", host, port)}
	if cfg.User != "" {
		u.User = url.UserPassword(cfg.User, cfg.Password)
	}
	if cfg.Database != "" {
		u.Path = "/" + cfg.Database
	}
	return u.String()
}

func (e *Engine) mongoClient(ctx context.Context, cfg SQLConfig) (*mongo.Client, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI(cfg)).SetServerSelectionTimeout(8*time.Second))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	return client, nil
}

func (e *Engine) pingMongo(ctx context.Context, cfg SQLConfig) error {
	c, err := e.mongoClient(ctx, cfg)
	if err != nil {
		return err
	}
	return c.Disconnect(ctx)
}

func (e *Engine) discoverMongo(ctx context.Context, cfg SQLConfig) ([]string, error) {
	c, err := e.mongoClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer c.Disconnect(ctx)
	dbName := cfg.Database
	if dbName == "" {
		dbName = "test"
	}
	names, err := c.Database(dbName).ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	if names == nil {
		names = []string{}
	}
	return names, nil
}

func (e *Engine) readMongo(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	c, err := e.mongoClient(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	defer c.Disconnect(ctx)
	dbName := cfg.Database
	if dbName == "" {
		dbName = "test"
	}
	collName := cfg.Table
	if collName == "" {
		names, err := c.Database(dbName).ListCollectionNames(ctx, bson.D{})
		if err != nil {
			return nil, nil, err
		}
		if len(names) == 0 {
			return nil, nil, fmt.Errorf("nenhuma colecção encontrada")
		}
		collName = names[0]
	}
	lim := int64(cfg.RowLimit())
	cur, err := c.Database(dbName).Collection(collName).Find(ctx, bson.D{}, options.Find().SetLimit(lim))
	if err != nil {
		return nil, nil, err
	}
	defer cur.Close(ctx)
	var maps []map[string]any
	for cur.Next(ctx) {
		var raw bson.M
		if err := cur.Decode(&raw); err != nil {
			return nil, nil, err
		}
		maps = append(maps, flattenBSON(raw))
	}
	if err := cur.Err(); err != nil {
		return nil, nil, err
	}
	return mapsToRows(maps)
}

func flattenBSON(m bson.M) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		switch t := v.(type) {
		case bson.M:
			b, _ := json.Marshal(t)
			out[k] = string(b)
		case bson.A, []any, map[string]any:
			b, _ := json.Marshal(t)
			out[k] = string(b)
		default:
			out[k] = v
		}
	}
	return out
}
