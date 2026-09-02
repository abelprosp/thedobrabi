package flow

type LayoutNode struct {
	ID      string `json:"id"`
	StepID  string `json:"stepId"`
	Kind    string `json:"kind"`
	Subkind string `json:"subkind"`
	Name    string `json:"name"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	Config  Config `json:"config"`
}

type LayoutEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type GraphLayout struct {
	Nodes []LayoutNode `json:"nodes"`
	Edges []LayoutEdge `json:"edges"`
}

// DefaultLayout places extracts on the left, transforms in the middle and loads on the right,
// then wires edges so a starter pipeline is already connected on the canvas.
func DefaultLayout(steps []Step) GraphLayout {
	nodes := make([]LayoutNode, 0, len(steps))
	var extracts, mids, loads []LayoutNode
	exI, midI, loadI := 0, 0, 0
	for _, st := range steps {
		cfg := st.Config
		if cfg == nil {
			cfg = Config{}
		}
		n := LayoutNode{
			ID:      st.ID.String(),
			StepID:  st.ID.String(),
			Kind:    st.Kind,
			Subkind: st.Subkind,
			Name:    st.Name,
			Config:  cfg,
		}
		switch st.Kind {
		case "extract":
			n.X, n.Y = 36, 40+exI*120
			exI++
			extracts = append(extracts, n)
		case "load":
			n.X, n.Y = 520, 40+loadI*120
			loadI++
			loads = append(loads, n)
		default:
			n.X, n.Y = 278, 40+midI*120
			midI++
			mids = append(mids, n)
		}
		nodes = append(nodes, n)
	}

	edges := []LayoutEdge{}
	chain := func(list []LayoutNode) {
		for i := 0; i < len(list)-1; i++ {
			edges = append(edges, LayoutEdge{From: list[i].ID, To: list[i+1].ID})
		}
	}

	switch {
	case len(mids) > 0:
		for _, e := range extracts {
			edges = append(edges, LayoutEdge{From: e.ID, To: mids[0].ID})
		}
		chain(mids)
		last := mids[len(mids)-1]
		for _, l := range loads {
			edges = append(edges, LayoutEdge{From: last.ID, To: l.ID})
		}
	case len(extracts) > 0 && len(loads) > 0:
		for _, e := range extracts {
			edges = append(edges, LayoutEdge{From: e.ID, To: loads[0].ID})
		}
		chain(loads)
	default:
		chain(nodes)
	}

	if len(edges) == 0 && len(nodes) > 1 {
		chain(nodes)
	}

	return GraphLayout{Nodes: nodes, Edges: edges}
}
