package flow

import (
	"testing"

	"github.com/google/uuid"
)

func TestDefaultLayoutCSVToClickHouse(t *testing.T) {
	e := Step{ID: uuid.New(), Kind: "extract", Subkind: "extract", Name: "Origem CSV"}
	l := Step{ID: uuid.New(), Kind: "load", Subkind: "load", Name: "ClickHouse"}
	g := DefaultLayout([]Step{e, l})
	if len(g.Nodes) != 2 {
		t.Fatalf("nodes=%d", len(g.Nodes))
	}
	if g.Nodes[0].X >= g.Nodes[1].X {
		t.Fatalf("extract should sit left of load: %+v", g.Nodes)
	}
	if len(g.Edges) != 1 || g.Edges[0].From != e.ID.String() || g.Edges[0].To != l.ID.String() {
		t.Fatalf("edges=%+v", g.Edges)
	}
}

func TestDefaultLayoutJoinTwoSources(t *testing.T) {
	a := Step{ID: uuid.New(), Kind: "extract", Subkind: "extract", Name: "Fonte A"}
	b := Step{ID: uuid.New(), Kind: "extract", Subkind: "extract", Name: "Fonte B"}
	j := Step{ID: uuid.New(), Kind: "transform", Subkind: "join", Name: "Juntar"}
	l := Step{ID: uuid.New(), Kind: "load", Subkind: "load", Name: "ClickHouse"}
	g := DefaultLayout([]Step{a, b, j, l})
	if len(g.Nodes) != 4 {
		t.Fatalf("nodes=%d", len(g.Nodes))
	}
	if g.Nodes[0].Y == g.Nodes[1].Y {
		t.Fatalf("two extracts should be stacked")
	}
	want := map[string]bool{
		a.ID.String() + "->" + j.ID.String(): true,
		b.ID.String() + "->" + j.ID.String(): true,
		j.ID.String() + "->" + l.ID.String(): true,
	}
	if len(g.Edges) != 3 {
		t.Fatalf("edges=%+v", g.Edges)
	}
	for _, e := range g.Edges {
		if !want[e.From+"->"+e.To] {
			t.Fatalf("unexpected edge %s -> %s", e.From, e.To)
		}
	}
}
