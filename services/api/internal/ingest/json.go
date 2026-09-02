package ingest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

func parseJSON(raw []byte) ([]string, [][]string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, nil, fmt.Errorf("json vazio")
	}
	if raw[0] == '[' {
		var arr []any
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, nil, fmt.Errorf("json inválido: %w", err)
		}
		return anyArrayToRows(arr)
	}
	if raw[0] == '{' {
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			return parseNDJSON(raw)
		}
		for _, key := range []string{"data", "value", "results", "items", "records", "rows"} {
			if arr, ok := obj[key].([]any); ok && len(arr) > 0 {
				return anyArrayToRows(arr)
			}
		}
		return mapsToRows([]map[string]any{obj})
	}
	return parseNDJSON(raw)
}

func parseNDJSON(raw []byte) ([]string, [][]string, error) {
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var maps []map[string]any
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			return nil, nil, fmt.Errorf("ndjson inválido: %w", err)
		}
		maps = append(maps, obj)
	}
	if err := sc.Err(); err != nil {
		return nil, nil, err
	}
	if len(maps) == 0 {
		return nil, nil, fmt.Errorf("a resposta não é um array JSON simples")
	}
	return mapsToRows(maps)
}

func anyArrayToRows(arr []any) ([]string, [][]string, error) {
	if len(arr) == 0 {
		return nil, nil, fmt.Errorf("array JSON vazio")
	}
	maps := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		switch t := item.(type) {
		case map[string]any:
			maps = append(maps, t)
		default:
			maps = append(maps, map[string]any{"value": t})
		}
	}
	return mapsToRows(maps)
}

func mapsToRows(maps []map[string]any) ([]string, [][]string, error) {
	if len(maps) == 0 {
		return nil, nil, fmt.Errorf("sem objectos JSON")
	}
	seen := map[string]bool{}
	var headers []string
	for _, m := range maps {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				headers = append(headers, k)
			}
		}
	}
	if len(headers) == 0 {
		return nil, nil, fmt.Errorf("objectos JSON sem chaves")
	}
	rows := make([][]string, 0, len(maps))
	for _, m := range maps {
		rec := make([]string, len(headers))
		for i, h := range headers {
			if v, ok := m[h]; ok && v != nil {
				switch t := v.(type) {
				case map[string]any, []any:
					b, _ := json.Marshal(t)
					rec[i] = string(b)
				default:
					rec[i] = fmt.Sprint(t)
				}
			}
		}
		rows = append(rows, rec)
	}
	return headers, rows, nil
}
