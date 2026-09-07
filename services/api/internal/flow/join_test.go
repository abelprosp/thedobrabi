package flow

import "testing"

func TestHashJoinInner(t *testing.T) {
	left := []map[string]any{{"id": "1", "nome": "A"}, {"id": "2", "nome": "B"}}
	right := []map[string]any{{"id": "1", "salario": 10}, {"id": "3", "salario": 9}}
	out := hashJoin(left, right, "id", "id", "inner")
	if len(out) != 1 || out[0]["nome"] != "A" || out[0]["salario"] != 10 {
		t.Fatalf("inner join: %+v", out)
	}
}

func TestHashJoinLeft(t *testing.T) {
	left := []map[string]any{{"id": "1", "nome": "A"}, {"id": "2", "nome": "B"}}
	right := []map[string]any{{"id": "1", "salario": 10}}
	out := hashJoin(left, right, "id", "id", "left")
	if len(out) != 2 {
		t.Fatalf("left join rows: %d", len(out))
	}
}

func TestAppendRows(t *testing.T) {
	out := appendRows(
		[]map[string]any{{"a": 1}},
		[]map[string]any{{"a": 2}},
	)
	if len(out) != 2 {
		t.Fatalf("append: %d", len(out))
	}
}

func TestApplyTransformJoin(t *testing.T) {
	extras := map[string][]map[string]any{
		"ds-b": {{"id": "1", "x": 9}},
	}
	st := Step{Subkind: "join", Config: map[string]any{"right_dataset_id": "ds-b", "left_key": "id", "right_key": "id"}}
	out, err := applyTransform(st, []string{"id"}, []map[string]any{{"id": "1", "n": "a"}}, extras, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0]["x"] != 9 {
		t.Fatalf("join transform: %+v", out)
	}
}
