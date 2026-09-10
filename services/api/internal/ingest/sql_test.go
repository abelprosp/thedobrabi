package ingest

import "testing"

func TestSafeSelectAllowsTimestampColumns(t *testing.T) {
	q, err := buildSelectionSQL("postgres", SourceSelection{
		Tables: []SelectedTable{
			{Schema: "public", Name: "template_evento", Columns: []string{"id", "updated_at", "deleted_at"}},
			{Schema: "public", Name: "template", Columns: []string{"id", "updated_at"}},
		},
		Joins: []SelectedJoin{{
			LeftTable: "public.template_evento", LeftColumn: "template_id",
			RightTable: "public.template", RightColumn: "id",
			Match: "both",
		}},
	}, 500)
	if err != nil {
		t.Fatal(err)
	}
	if !safeSelect(q) {
		t.Fatalf("join with updated_at/deleted_at should be allowed:\n%s", q)
	}
}

func TestSafeSelect(t *testing.T) {
	allow := []string{
		`SELECT id, updated_at, deleted_at FROM public.utilizador LIMIT 10`,
		`SELECT "demanda"."updated_at" AS "demanda_updated_at" FROM "public"."demanda" AS "demanda"`,
		`SELECT * FROM t WHERE note = 'please update' AND status = 'deleted'`,
		`SELECT alteracao, grant_id, copyright FROM t`,
		`SELECT 1;`,
		`/* comment with DROP TABLE */ SELECT id FROM t`,
	}
	for _, q := range allow {
		if !safeSelect(q) {
			t.Fatalf("expected allow: %s", q)
		}
	}
	deny := []string{
		``,
		`UPDATE t SET x = 1`,
		`INSERT INTO t VALUES (1)`,
		`DELETE FROM t`,
		`SELECT 1; DROP TABLE t`,
		`SELECT * FROM t; UPDATE t SET x = 1`,
		`DROP TABLE t`,
		`ALTER TABLE t ADD COLUMN x int`,
		`COPY t FROM STDIN`,
		`GRANT SELECT ON t TO u`,
	}
	for _, q := range deny {
		if safeSelect(q) {
			t.Fatalf("expected deny: %s", q)
		}
	}
}
