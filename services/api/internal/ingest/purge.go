package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"github.com/thedobra/thedobra/services/api/internal/schemax"
)

func (e *Engine) DeleteDataset(ctx context.Context, orgID, wsID, id uuid.UUID) error {
	var table, slug, name string
	err := e.pg.QueryRow(ctx, `
		SELECT COALESCE(clickhouse_table, ''), COALESCE(slug, ''), COALESCE(name, '')
		FROM datasets WHERE id=$1 AND org_id=$2 AND workspace_id=$3
	`, id, orgID, wsID).Scan(&table, &slug, &name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("conjunto não encontrado")
		}
		return err
	}

	keys, _ := e.lakeObjectKeys(ctx, id)
	e.removeLakeObjects(ctx, orgID, id, slug, name, keys)

	if e.ch != nil && table != "" && identOK(table) && identOK(e.cfg.ClickHouseDB) {
		if err := e.ch.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s.`%s`", e.cfg.ClickHouseDB, table)); err != nil {
			return fmt.Errorf("falha a eliminar tabela ClickHouse: %w", err)
		}
	}

	tag, err := e.pg.Exec(ctx, `DELETE FROM datasets WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, orgID, wsID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("conjunto não encontrado")
	}

	_ = e.StripDatasetFromLayouts(ctx, orgID, wsID, id)
	_, _ = e.pg.Exec(ctx, `
		DELETE FROM query_history
		WHERE org_id=$1 AND workspace_id=$2 AND query_json->>'dataset_id' = $3
	`, orgID, wsID, id.String())
	_ = e.PurgeOrphanClickHouseTables(ctx)
	return nil
}

func (e *Engine) lakeObjectKeys(ctx context.Context, datasetID uuid.UUID) ([]string, error) {
	rows, err := e.pg.Query(ctx, `SELECT object_key FROM lake_objects WHERE dataset_id=$1`, datasetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			continue
		}
		if k != "" {
			keys = append(keys, k)
		}
	}
	return keys, rows.Err()
}

func (e *Engine) removeLakeObjects(ctx context.Context, orgID, datasetID uuid.UUID, slug, name string, keys []string) {
	if e.minio == nil {
		return
	}
	seen := map[string]struct{}{}
	remove := func(key string) {
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		_ = e.minio.RemoveObject(ctx, e.cfg.MinioBucket, key, minio.RemoveObjectOptions{})
	}
	for _, k := range keys {
		remove(k)
	}
	org := orgID.String()
	id := datasetID.String()
	slugs := uniqueSanitizedSlugs(slug, name)
	for _, stage := range []string{"gold", "silver", "bronze"} {
		e.removeMinioPrefix(ctx, path.Join(stage, "company_id="+org, "dataset_id="+id), remove)
		for _, s := range slugs {
			e.removeMinioPrefix(ctx, path.Join(stage, "company_id="+org, "dataset="+s), remove)
		}
	}
}

func uniqueSanitizedSlugs(slug, name string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(raw string) {
		s := schemax.SanitizeIdent(raw)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	add(name)
	add(slug)
	if i := strings.LastIndex(slug, "-"); i > 0 && len(slug)-i == 9 {
		add(slug[:i])
	}
	return out
}

func (e *Engine) removeMinioPrefix(ctx context.Context, prefix string, remove func(string)) {
	if e.minio == nil || prefix == "" {
		return
	}
	for obj := range e.minio.ListObjects(ctx, e.cfg.MinioBucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil || obj.Key == "" {
			continue
		}
		remove(obj.Key)
	}
}

// PurgeOrphanClickHouseTables drops ds_* tables that are not referenced by datasets.clickhouse_table.
func (e *Engine) PurgeOrphanClickHouseTables(ctx context.Context) error {
	if e.ch == nil || !identOK(e.cfg.ClickHouseDB) {
		return nil
	}
	rows, err := e.ch.Query(ctx, fmt.Sprintf("SHOW TABLES FROM `%s`", e.cfg.ClickHouseDB))
	if err != nil {
		return err
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		name = strings.Trim(name, "`")
		if strings.HasPrefix(name, "ds_") && identOK(name) {
			tables = append(tables, name)
		}
	}
	rows.Close()
	if len(tables) == 0 {
		return nil
	}

	live := map[string]struct{}{}
	pgRows, err := e.pg.Query(ctx, `SELECT clickhouse_table FROM datasets WHERE clickhouse_table IS NOT NULL AND clickhouse_table <> ''`)
	if err != nil {
		return err
	}
	defer pgRows.Close()
	for pgRows.Next() {
		var t string
		if err := pgRows.Scan(&t); err != nil {
			continue
		}
		if t != "" {
			live[t] = struct{}{}
		}
	}

	for _, t := range tables {
		if _, ok := live[t]; ok {
			continue
		}
		_ = e.ch.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS `%s`.`%s`", e.cfg.ClickHouseDB, t))
	}
	return nil
}

func (e *Engine) StripDatasetFromLayouts(ctx context.Context, orgID, wsID, datasetID uuid.UUID) error {
	id := datasetID.String()
	drows, err := e.pg.Query(ctx, `SELECT id, layout_json FROM dashboards WHERE org_id=$1 AND workspace_id=$2`, orgID, wsID)
	if err != nil {
		return err
	}
	type dashRow struct {
		id  uuid.UUID
		raw []byte
	}
	var dashes []dashRow
	for drows.Next() {
		var r dashRow
		if err := drows.Scan(&r.id, &r.raw); err != nil {
			continue
		}
		dashes = append(dashes, r)
	}
	drows.Close()
	for _, r := range dashes {
		next, changed := stripDatasetFromLayoutJSON(r.raw, id)
		if !changed {
			continue
		}
		_, _ = e.pg.Exec(ctx, `UPDATE dashboards SET layout_json=$1, updated_at=now() WHERE id=$2 AND org_id=$3 AND workspace_id=$4`, next, r.id, orgID, wsID)
	}

	rrows, err := e.pg.Query(ctx, `SELECT id, pages_json FROM reports WHERE org_id=$1 AND workspace_id=$2`, orgID, wsID)
	if err != nil {
		return err
	}
	type reportRow struct {
		id  uuid.UUID
		raw []byte
	}
	var reports []reportRow
	for rrows.Next() {
		var r reportRow
		if err := rrows.Scan(&r.id, &r.raw); err != nil {
			continue
		}
		reports = append(reports, r)
	}
	rrows.Close()
	for _, r := range reports {
		next, changed := stripDatasetFromPagesJSON(r.raw, id)
		if !changed {
			continue
		}
		_, _ = e.pg.Exec(ctx, `UPDATE reports SET pages_json=$1, updated_at=now() WHERE id=$2 AND org_id=$3 AND workspace_id=$4`, next, r.id, orgID, wsID)
	}
	return nil
}

func stripDatasetFromLayoutJSON(raw []byte, datasetID string) ([]byte, bool) {
	if len(raw) == 0 {
		return raw, false
	}
	var layout map[string]any
	if err := json.Unmarshal(raw, &layout); err != nil {
		return raw, false
	}
	widgets, ok := layout["widgets"].([]any)
	if !ok {
		return raw, false
	}
	kept, changed := stripWidgetsForDataset(widgets, datasetID)
	if !changed {
		return raw, false
	}
	layout["widgets"] = kept
	out, err := json.Marshal(layout)
	if err != nil {
		return raw, false
	}
	return out, true
}

func stripDatasetFromPagesJSON(raw []byte, datasetID string) ([]byte, bool) {
	if len(raw) == 0 {
		return raw, false
	}
	var pages []any
	if err := json.Unmarshal(raw, &pages); err != nil {
		return raw, false
	}
	changed := false
	for i, p := range pages {
		pm, ok := p.(map[string]any)
		if !ok {
			continue
		}
		widgets, ok := pm["widgets"].([]any)
		if !ok {
			continue
		}
		kept, wchanged := stripWidgetsForDataset(widgets, datasetID)
		if !wchanged {
			continue
		}
		pm["widgets"] = kept
		pages[i] = pm
		changed = true
	}
	if !changed {
		return raw, false
	}
	out, err := json.Marshal(pages)
	if err != nil {
		return raw, false
	}
	return out, true
}

func stripWidgetsForDataset(widgets []any, datasetID string) ([]any, bool) {
	kept := make([]any, 0, len(widgets))
	changed := false
	for _, raw := range widgets {
		w, ok := raw.(map[string]any)
		if !ok {
			kept = append(kept, raw)
			continue
		}
		if widgetQueryDatasetID(w) == datasetID {
			changed = true
			continue
		}
		kept = append(kept, w)
	}
	return kept, changed
}

func widgetQueryDatasetID(w map[string]any) string {
	q, _ := w["query"].(map[string]any)
	if q == nil {
		return ""
	}
	switch v := q["dataset_id"].(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}
