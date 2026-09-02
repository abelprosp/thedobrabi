package ingest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
	"github.com/parquet-go/parquet-go"
)

func (e *Engine) readRemoteFile(ctx context.Context, typ string, cfg SQLConfig) ([]string, [][]string, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, nil, fmt.Errorf("carregue um ficheiro ou indique um URL HTTP(S)")
	}
	raw, err := e.downloadBytes(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	return parseFileBytes(typ, raw)
}

func parseFileBytes(kind string, raw []byte) ([]string, [][]string, error) {
	switch kind {
	case "xlsx":
		return parseXLSX(raw)
	case "json":
		return parseJSON(raw)
	case "parquet":
		return parseParquet(raw)
	case "pdf":
		return parsePDF(raw)
	case "ofx":
		return parseOFX(raw)
	default:
		return parseCSV(raw)
	}
}

func (e *Engine) downloadBytes(ctx context.Context, cfg SQLConfig) ([]byte, error) {
	if err := assertHTTPURL(cfg.URL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.URL, nil)
	if err != nil {
		return nil, err
	}
	if t := cfg.AuthToken(); t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	resp, err := connectorHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 180))
	}
	return raw, nil
}

func parseParquet(raw []byte) ([]string, [][]string, error) {
	if len(raw) < 8 {
		return nil, nil, fmt.Errorf("parquet inválido (ficheiro demasiado pequeno)")
	}
	f, err := parquet.OpenFile(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, nil, fmt.Errorf("parquet inválido: %w", err)
	}
	schema := f.Schema()
	fields := schema.Fields()
	if len(fields) == 0 {
		return nil, nil, fmt.Errorf("parquet sem colunas")
	}
	headers := make([]string, len(fields))
	for i, field := range fields {
		headers[i] = field.Name()
	}
	reader := parquet.NewReader(f)
	defer reader.Close()
	var rows [][]string
	for {
		row := map[string]any{}
		if err := reader.Read(&row); err != nil {
			if err == io.EOF {
				break
			}
			return nil, nil, fmt.Errorf("parquet inválido: %w", err)
		}
		rec := make([]string, len(headers))
		for i, h := range headers {
			if v, ok := row[h]; ok && v != nil {
				rec[i] = fmt.Sprint(v)
			}
		}
		rows = append(rows, rec)
		if len(rows) >= 500000 {
			break
		}
	}
	if len(headers) == 0 {
		return nil, nil, fmt.Errorf("parquet sem colunas")
	}
	return headers, rows, nil
}

func parsePDF(raw []byte) ([]string, [][]string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, nil, fmt.Errorf("pdf inválido: %w", err)
	}
	var lines []string
	n := reader.NumPage()
	for i := 1; i <= n; i++ {
		p := reader.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		for _, ln := range strings.Split(text, "\n") {
			ln = strings.TrimSpace(ln)
			if ln != "" {
				lines = append(lines, ln)
			}
		}
	}
	if len(lines) == 0 {
		return nil, nil, fmt.Errorf("pdf sem texto extraível")
	}
	table := splitPDFTable(lines)
	if len(table) >= 2 {
		return table[0], table[1:], nil
	}
	headers := []string{"line", "text"}
	rows := make([][]string, len(lines))
	for i, ln := range lines {
		rows[i] = []string{fmt.Sprintf("%d", i+1), ln}
	}
	return headers, rows, nil
}

func splitPDFTable(lines []string) [][]string {
	var out [][]string
	for _, ln := range lines {
		parts := splitPDFCols(ln)
		if len(parts) < 2 {
			continue
		}
		out = append(out, parts)
	}
	if len(out) < 2 {
		return nil
	}
	width := len(out[0])
	ok := 0
	for _, r := range out {
		if len(r) == width {
			ok++
		}
	}
	if ok < len(out)/2 {
		return nil
	}
	return out
}

func splitPDFCols(s string) []string {
	var parts []string
	var b strings.Builder
	spaces := 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			spaces++
			if spaces >= 2 && b.Len() > 0 {
				parts = append(parts, strings.TrimSpace(b.String()))
				b.Reset()
				spaces = 0
			}
			continue
		}
		if spaces == 1 && b.Len() > 0 {
			b.WriteRune(' ')
		}
		spaces = 0
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		parts = append(parts, strings.TrimSpace(b.String()))
	}
	return parts
}
