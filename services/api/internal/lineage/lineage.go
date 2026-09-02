package lineage

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ pg *pgxpool.Pool }

func New(pg *pgxpool.Pool) *Service { return &Service{pg: pg} }

type Node struct {
	ID    uuid.UUID      `json:"id"`
	Kind  string         `json:"kind"`
	RefID *uuid.UUID     `json:"ref_id,omitempty"`
	Name  string         `json:"name"`
	Meta  map[string]any `json:"meta"`
}

type Edge struct {
	From     uuid.UUID `json:"from"`
	To       uuid.UUID `json:"to"`
	Relation string    `json:"relation"`
}

type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

func (s *Service) Ensure(ctx context.Context, orgID, wsID uuid.UUID, kind string, ref uuid.UUID, name string, meta map[string]any) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pg.QueryRow(ctx, `
		SELECT id FROM lineage_nodes WHERE org_id=$1 AND workspace_id=$2 AND kind=$3 AND ref_id=$4 LIMIT 1
	`, orgID, wsID, kind, ref).Scan(&id)
	if err == nil {
		return id, nil
	}
	id = uuid.New()
	raw, _ := json.Marshal(meta)
	if raw == nil {
		raw = []byte("{}")
	}
	_, err = s.pg.Exec(ctx, `
		INSERT INTO lineage_nodes (id, org_id, workspace_id, kind, ref_id, name, meta) VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, id, orgID, wsID, kind, ref, name, raw)
	return id, err
}

func (s *Service) Link(ctx context.Context, orgID, wsID, from, to uuid.UUID, rel string) {
	_, _ = s.pg.Exec(ctx, `
		INSERT INTO lineage_edges (org_id, workspace_id, from_id, to_id, relation) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (from_id, to_id, relation) DO NOTHING
	`, orgID, wsID, from, to, rel)
}

func (s *Service) RecordIngest(ctx context.Context, orgID, wsID, sourceID, datasetID uuid.UUID, sourceName, datasetName string) {
	src, _ := s.Ensure(ctx, orgID, wsID, "source", sourceID, sourceName, nil)
	ds, _ := s.Ensure(ctx, orgID, wsID, "dataset", datasetID, datasetName, nil)
	tf, _ := s.Ensure(ctx, orgID, wsID, "transformation", datasetID, "Normalização · "+datasetName, map[string]any{"stage": "bronze→gold"})
	if src != uuid.Nil && ds != uuid.Nil {
		s.Link(ctx, orgID, wsID, src, tf, "extrai")
		s.Link(ctx, orgID, wsID, tf, ds, "grava")
	}
}

func (s *Service) RecordMetric(ctx context.Context, orgID, wsID, datasetID uuid.UUID, metricName, expr string) {
	ds, _ := s.Ensure(ctx, orgID, wsID, "dataset", datasetID, "dataset", nil)
	mid := uuid.NewSHA1(datasetID, []byte(metricName))
	m, _ := s.Ensure(ctx, orgID, wsID, "metric", mid, metricName, map[string]any{"expression": expr})
	s.Link(ctx, orgID, wsID, ds, m, "define")
}

func (s *Service) RecordDashboard(ctx context.Context, orgID, wsID, dashID, datasetID uuid.UUID, dashName string) {
	d, _ := s.Ensure(ctx, orgID, wsID, "dashboard", dashID, dashName, nil)
	if datasetID != uuid.Nil {
		ds, _ := s.Ensure(ctx, orgID, wsID, "dataset", datasetID, "dataset", nil)
		s.Link(ctx, orgID, wsID, ds, d, "visualiza")
	}
}

func (s *Service) RecordReport(ctx context.Context, orgID, wsID, reportID, dashID uuid.UUID, name string) {
	r, _ := s.Ensure(ctx, orgID, wsID, "report", reportID, name, nil)
	if dashID != uuid.Nil {
		d, _ := s.Ensure(ctx, orgID, wsID, "dashboard", dashID, "dashboard", nil)
		s.Link(ctx, orgID, wsID, d, r, "reporta")
	}
}

func (s *Service) DeleteForDataset(ctx context.Context, orgID, wsID, datasetID uuid.UUID) {
	_, _ = s.pg.Exec(ctx, `
		DELETE FROM lineage_nodes
		WHERE org_id=$1 AND workspace_id=$2
		  AND (
		    ref_id = $3
		    OR (
		      kind = 'dataset'
		      AND (
		        ref_id IS NULL
		        OR ref_id NOT IN (SELECT id FROM datasets WHERE org_id=$1 AND workspace_id=$2)
		      )
		    )
		  )
	`, orgID, wsID, datasetID)
	_, _ = s.pg.Exec(ctx, `
		DELETE FROM lineage_edges
		WHERE org_id=$1 AND workspace_id=$2
		  AND (
		    NOT EXISTS (SELECT 1 FROM lineage_nodes n WHERE n.id = from_id)
		    OR NOT EXISTS (SELECT 1 FROM lineage_nodes n WHERE n.id = to_id)
		  )
	`, orgID, wsID)
}

func (s *Service) Graph(ctx context.Context, orgID, wsID uuid.UUID) (Graph, error) {
	g, err := s.load(ctx, orgID, wsID)
	if err != nil {
		return g, err
	}
	if len(g.Nodes) == 0 {
		s.backfill(ctx, orgID, wsID)
		return s.load(ctx, orgID, wsID)
	}
	return g, nil
}

func (s *Service) load(ctx context.Context, orgID, wsID uuid.UUID) (Graph, error) {
	g := Graph{Nodes: []Node{}, Edges: []Edge{}}
	rows, err := s.pg.Query(ctx, `SELECT id, kind, ref_id, name, meta FROM lineage_nodes WHERE org_id=$1 AND workspace_id=$2 ORDER BY created_at`, orgID, wsID)
	if err != nil {
		return g, err
	}
	defer rows.Close()
	for rows.Next() {
		var n Node
		var meta []byte
		if err := rows.Scan(&n.ID, &n.Kind, &n.RefID, &n.Name, &meta); err != nil {
			return g, err
		}
		_ = json.Unmarshal(meta, &n.Meta)
		g.Nodes = append(g.Nodes, n)
	}
	erows, err := s.pg.Query(ctx, `SELECT from_id, to_id, relation FROM lineage_edges WHERE org_id=$1 AND workspace_id=$2`, orgID, wsID)
	if err != nil {
		return g, err
	}
	defer erows.Close()
	for erows.Next() {
		var e Edge
		if err := erows.Scan(&e.From, &e.To, &e.Relation); err != nil {
			return g, err
		}
		g.Edges = append(g.Edges, e)
	}
	return g, nil
}

func (s *Service) backfill(ctx context.Context, orgID, wsID uuid.UUID) {
	drows, err := s.pg.Query(ctx, `SELECT id, name FROM datasets WHERE org_id=$1 AND workspace_id=$2`, orgID, wsID)
	if err != nil {
		return
	}
	defer drows.Close()
	for drows.Next() {
		var id uuid.UUID
		var name string
		if err := drows.Scan(&id, &name); err != nil {
			continue
		}
		s.RecordIngest(ctx, orgID, wsID, id, id, name, name)
		mrows, err := s.pg.Query(ctx, `SELECT model_json FROM semantic_models WHERE dataset_id=$1`, id)
		if err != nil {
			continue
		}
		for mrows.Next() {
			var raw []byte
			if err := mrows.Scan(&raw); err != nil {
				continue
			}
			var model struct {
				Measures []struct {
					Name       string `json:"name"`
					Expression string `json:"expression"`
				} `json:"measures"`
			}
			_ = json.Unmarshal(raw, &model)
			for _, m := range model.Measures {
				s.RecordMetric(ctx, orgID, wsID, id, m.Name, m.Expression)
			}
		}
		mrows.Close()
	}
	brows, err := s.pg.Query(ctx, `SELECT id, name FROM dashboards WHERE org_id=$1 AND workspace_id=$2`, orgID, wsID)
	if err == nil {
		defer brows.Close()
		for brows.Next() {
			var id uuid.UUID
			var name string
			_ = brows.Scan(&id, &name)
			s.RecordDashboard(ctx, orgID, wsID, id, uuid.Nil, name)
		}
	}
	rrows, err := s.pg.Query(ctx, `SELECT id, name FROM reports WHERE org_id=$1 AND workspace_id=$2`, orgID, wsID)
	if err == nil {
		defer rrows.Close()
		for rrows.Next() {
			var id uuid.UUID
			var name string
			_ = rrows.Scan(&id, &name)
			s.RecordReport(ctx, orgID, wsID, id, uuid.Nil, name)
		}
	}
}
