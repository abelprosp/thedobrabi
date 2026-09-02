package gateway

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Instance is a registered on-premise or cloud gateway that proxies queries and CDC traffic.
type Instance struct {
	ID         uuid.UUID `json:"id"`
	OrgID      *uuid.UUID `json:"org_id,omitempty"`
	Name       string    `json:"name"`
	TokenHash  string    `json:"-"`
	Status     string    `json:"status"`
	Version    string    `json:"version,omitempty"`
	LastPingAt *time.Time `json:"last_ping_at,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Store is the gateway registry.
type Store struct{ pg *pgxpool.Pool }

func NewStore(pg *pgxpool.Pool) *Store { return &Store{pg: pg} }

func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func generateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return "gw_" + hex.EncodeToString(b)
}

// GenerateToken creates a new gateway instance for an org and returns the plaintext token (once).
func (s *Store) GenerateToken(ctx context.Context, orgID uuid.UUID, name string) (string, uuid.UUID, error) {
	token := generateToken()
	id := uuid.New()
	_, err := s.pg.Exec(ctx, `
		INSERT INTO gateway_instances (id, org_id, name, token_hash, status, version, metadata, last_ping_at)
		VALUES ($1,$2,$3,$4,'offline','','{}'::jsonb,now())
	`, id, orgID, name, HashToken(token))
	if err != nil {
		return "", uuid.Nil, err
	}
	return token, id, nil
}

func (s *Store) Register(ctx context.Context, orgID *uuid.UUID, name, tokenHash, version string, meta map[string]any) (uuid.UUID, error) {
	id := uuid.New()
	var metaJSON []byte
	if meta != nil {
		metaJSON = []byte(`{}`)
	}
	_, err := s.pg.Exec(ctx, `
		INSERT INTO gateway_instances (id, org_id, name, token_hash, status, version, metadata, last_ping_at)
		VALUES ($1,$2,$3,$4,'online',$5,$6,now())
	`, id, orgID, name, tokenHash, version, metaJSON)
	return id, err
}

func (s *Store) Heartbeat(ctx context.Context, tokenHash, status, version string) error {
	ct, err := s.pg.Exec(ctx, `
		UPDATE gateway_instances SET status=$1, version=$2, last_ping_at=now(), updated_at=now()
		WHERE token_hash=$3
	`, status, version, tokenHash)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("gateway not found")
	}
	return nil
}

func (s *Store) List(ctx context.Context, orgID uuid.UUID) ([]Instance, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id, org_id, name, status, version, last_ping_at, metadata, created_at, updated_at
		FROM gateway_instances WHERE org_id=$1 ORDER BY last_ping_at DESC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Instance
	for rows.Next() {
		var g Instance
		if err := rows.Scan(&g.ID, &g.OrgID, &g.Name, &g.Status, &g.Version, &g.LastPingAt, &g.Metadata, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) Health(ctx context.Context, tokenHash string) (Instance, error) {
	var g Instance
	err := s.pg.QueryRow(ctx, `
		SELECT id, org_id, name, status, version, last_ping_at, created_at, updated_at
		FROM gateway_instances WHERE token_hash=$1
	`, tokenHash).Scan(&g.ID, &g.OrgID, &g.Name, &g.Status, &g.Version, &g.LastPingAt, &g.CreatedAt, &g.UpdatedAt)
	return g, err
}
