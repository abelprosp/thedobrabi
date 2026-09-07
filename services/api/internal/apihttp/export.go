package apihttp

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/thedobra/thedobra/services/api/internal/httpx"
)

func (s *Server) exportDataset(w http.ResponseWriter, r *http.Request) {
	_, org, ws, _ := principal(r)
	id := chi.URLParam(r, "id")
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "xlsx" {
		httpx.Error(w, 400, "invalid", "formato deve ser csv ou xlsx")
		return
	}
	var name string
	err := s.deps.PG.QueryRow(r.Context(), `SELECT name FROM datasets WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, org, ws).Scan(&name)
	if err != nil {
		httpx.Error(w, 404, "not_found", "conjunto não encontrado")
		return
	}
	headers, rows, err := s.query.ReadRows(r.Context(), org, ws, id, 100000)
	if err != nil {
		httpx.Error(w, 400, "export_failed", err.Error())
		return
	}
	safe := slugFile(name)
	if format == "xlsx" {
		raw, err := writeXLSX(headers, rows)
		if err != nil {
			httpx.Error(w, 500, "export_failed", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, safe))
		w.Write(raw)
		return
	}
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	cw := csv.NewWriter(&buf)
	_ = cw.Write(headers)
	for _, row := range rows {
		rec := make([]string, len(headers))
		for i, h := range headers {
			rec[i] = fmt.Sprint(row[h])
			if row[h] == nil {
				rec[i] = ""
			}
		}
		_ = cw.Write(rec)
	}
	cw.Flush()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, safe))
	w.Write(buf.Bytes())
}

func slugFile(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "conjunto"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "conjunto"
	}
	return out
}

func writeXLSX(headers []string, rows []map[string]any) ([]byte, error) {
	sheet := buildSheetXML(headers, rows)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="Dados" sheetId="1" r:id="rId1"/></sheets>
</workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`,
		"xl/worksheets/sheet1.xml": sheet,
	}
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(body)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildSheetXML(headers []string, rows []map[string]any) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	writeXLSXRow(&b, 1, headers)
	for i, row := range rows {
		vals := make([]string, len(headers))
		for j, h := range headers {
			if row[h] == nil {
				continue
			}
			vals[j] = fmt.Sprint(row[h])
		}
		writeXLSXRow(&b, i+2, vals)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

func writeXLSXRow(b *strings.Builder, rowNum int, vals []string) {
	fmt.Fprintf(b, `<row r="%d">`, rowNum)
	for i, v := range vals {
		ref := xlsxCol(i) + strconv.Itoa(rowNum)
		fmt.Fprintf(b, `<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, xmlEscape(v))
	}
	b.WriteString(`</row>`)
}

func xlsxCol(i int) string {
	s := ""
	for i >= 0 {
		s = string(rune('A'+i%26)) + s
		i = i/26 - 1
	}
	return s
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
