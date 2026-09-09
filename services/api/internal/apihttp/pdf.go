package apihttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thedobra/thedobra/services/api/internal/queryeng"
)

type reportPageIn struct {
	Name    string           `json:"name"`
	Widgets []reportWidgetIn `json:"widgets"`
}

type reportWidgetIn struct {
	ID    string           `json:"id"`
	Type  string           `json:"type"`
	Title string           `json:"title"`
	Text  string           `json:"text"`
	Query queryeng.Request `json:"query"`
}

type pdfSheet struct {
	Title  string
	Blocks []pdfBlock
}

type pdfBlock struct {
	Title   string
	Kind    string
	KPI     string
	Headers []string
	Rows    [][]string
	Text    string
}

func (s *Server) renderReportVisualPDF(ctx context.Context, org, ws, uid uuid.UUID, role, title string, pagesRaw []byte) []byte {
	var pages []reportPageIn
	if len(pagesRaw) > 0 {
		_ = json.Unmarshal(pagesRaw, &pages)
	}
	if len(pages) == 0 {
		return reportPDF(title, map[string]any{"executive_summary": "Relatório sem páginas."})
	}
	sheets := make([]pdfSheet, 0, len(pages))
	for i, p := range pages {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			name = fmt.Sprintf("Página %d", i+1)
		}
		sheet := pdfSheet{Title: name}
		for _, w := range p.Widgets {
			sheet.Blocks = append(sheet.Blocks, s.widgetToPDFBlock(ctx, org, ws, uid, role, w))
		}
		if len(sheet.Blocks) == 0 {
			sheet.Blocks = append(sheet.Blocks, pdfBlock{Title: name, Kind: "text", Text: "Esta página ainda não tem visuais."})
		}
		sheets = append(sheets, sheet)
	}
	return writePagedPDF(title, sheets)
}

func (s *Server) widgetToPDFBlock(ctx context.Context, org, ws, uid uuid.UUID, role string, w reportWidgetIn) pdfBlock {
	title := strings.TrimSpace(w.Title)
	if title == "" {
		title = w.Type
	}
	switch w.Type {
	case "text", "markdown":
		txt := strings.TrimSpace(w.Text)
		if txt == "" {
			txt = title
		}
		return pdfBlock{Title: title, Kind: "text", Text: txt}
	case "image", "iframe":
		return pdfBlock{Title: title, Kind: "text", Text: "Visual de " + w.Type}
	case "data_intelligence":
		return pdfBlock{Title: title, Kind: "text", Text: "Análise com IA dos visuais deste dashboard."}
	}
	if strings.TrimSpace(w.Query.DatasetID) == "" {
		return pdfBlock{Title: title, Kind: "text", Text: "Sem conjunto de dados."}
	}
	req := w.Query
	if req.Limit <= 0 || req.Limit > 80 {
		if w.Type == "big_table" {
			req.Limit = 80
		} else {
			req.Limit = 40
		}
	}
	if w.Type == "kpi" || w.Type == "kpi_goal" || w.Type == "metric_group" || w.Type == "gauge" {
		req.Dimensions = nil
	}
	res, err := s.query.Execute(ctx, org, ws, uid, role, req)
	if err != nil {
		return pdfBlock{Title: title, Kind: "text", Text: "Erro ao consultar: " + err.Error()}
	}
	if w.Type == "kpi" || w.Type == "kpi_goal" || w.Type == "gauge" {
		return pdfBlock{Title: title, Kind: "kpi", KPI: firstCell(res)}
	}
	headers := res.Columns
	rows := make([][]string, 0, len(res.Rows))
	for _, r := range res.Rows {
		line := make([]string, len(headers))
		for i, c := range headers {
			line[i] = fmt.Sprint(r[c])
		}
		rows = append(rows, line)
	}
	return pdfBlock{Title: title, Kind: "table", Headers: headers, Rows: rows}
}

func firstCell(res queryeng.Result) string {
	if len(res.Rows) == 0 || len(res.Columns) == 0 {
		return "—"
	}
	return fmt.Sprint(res.Rows[0][res.Columns[0]])
}

func reportPDF(title string, content map[string]any) []byte {
	return writePagedPDF(title, []pdfSheet{{
		Title: "Briefing",
		Blocks: []pdfBlock{
			{Title: "Resumo executivo", Kind: "text", Text: fmt.Sprint(content["executive_summary"])},
			{Title: "Performance", Kind: "text", Text: fmt.Sprint(content["performance"])},
			{Title: "Riscos", Kind: "text", Text: fmt.Sprint(content["risks"])},
			{Title: "Oportunidades", Kind: "text", Text: fmt.Sprint(content["opportunities"])},
			{Title: "Acções recomendadas", Kind: "text", Text: fmt.Sprint(content["recommended_actions"])},
		},
	}})
}

func reportHTML(title string, content map[string]any) string {
	esc := func(v any) string {
		s := fmt.Sprint(v)
		s = strings.ReplaceAll(s, "&", "&amp;")
		s = strings.ReplaceAll(s, "<", "&lt;")
		s = strings.ReplaceAll(s, ">", "&gt;")
		return s
	}
	return fmt.Sprintf(`<!doctype html><html><body style="font-family:sans-serif;color:#0f172a">
<h1>%s</h1>
<p style="color:#64748b">Gerado em %s</p>
<h2>Resumo</h2><p>%s</p>
<h2>Performance</h2><p>%s</p>
<h2>Riscos</h2><p>%s</p>
<h2>Oportunidades</h2><p>%s</p>
<h2>Acções</h2><p>%s</p>
</body></html>`,
		esc(title), time.Now().UTC().Format("2006-01-02 15:04 UTC"),
		esc(content["executive_summary"]), esc(content["performance"]),
		esc(content["risks"]), esc(content["opportunities"]), esc(content["recommended_actions"]))
}

func writePagedPDF(title string, sheets []pdfSheet) []byte {
	if len(sheets) == 0 {
		sheets = []pdfSheet{{Title: title, Blocks: []pdfBlock{{Title: title, Kind: "text", Text: ""}}}}
	}
	type pageBuf struct{ body string }
	pages := []pageBuf{}
	var cur strings.Builder
	y := 0
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		pages = append(pages, pageBuf{body: cur.String()})
		cur.Reset()
	}
	startPage := func(heading string) {
		flush()
		y = 760
		cur.WriteString(fmt.Sprintf("BT /F2 16 Tf 40 %d Td (%s) Tj ET\n", y, pdfEscape(clipPDF(heading, 70))))
		y -= 22
		cur.WriteString(fmt.Sprintf("BT /F1 9 Tf 40 %d Td (%s) Tj ET\n", y, pdfEscape("TheDobra · "+time.Now().UTC().Format("2006-01-02 15:04 UTC"))))
		y -= 28
	}
	need := func(h int) {
		if y-h < 48 {
			startPage(title)
		}
	}
	for _, sh := range sheets {
		startPage(title + "  ·  " + sh.Title)
		for _, b := range sh.Blocks {
			need(36)
			cur.WriteString(fmt.Sprintf("BT /F2 12 Tf 40 %d Td (%s) Tj ET\n", y, pdfEscape(clipPDF(b.Title, 80))))
			y -= 18
			switch b.Kind {
			case "kpi":
				need(28)
				cur.WriteString(fmt.Sprintf("BT /F2 22 Tf 40 %d Td (%s) Tj ET\n", y, pdfEscape(clipPDF(b.KPI, 40))))
				y -= 30
			case "table":
				cols := b.Headers
				if len(cols) == 0 {
					cur.WriteString(fmt.Sprintf("BT /F1 10 Tf 40 %d Td (sem linhas) Tj ET\n", y))
					y -= 16
					break
				}
				width := 520 / max(1, min(len(cols), 6))
				drawRow := func(cells []string, font string) {
					need(14)
					x := 40
					n := min(len(cols), 6)
					for i := 0; i < n; i++ {
						val := ""
						if i < len(cells) {
							val = cells[i]
						}
						cur.WriteString(fmt.Sprintf("BT /%s 8 Tf %d %d Td (%s) Tj ET\n", font, x, y, pdfEscape(clipPDF(val, width/5))))
						x += width
					}
					y -= 13
				}
				drawRow(cols, "F2")
				limit := min(len(b.Rows), 28)
				for i := 0; i < limit; i++ {
					drawRow(b.Rows[i], "F1")
				}
				if len(b.Rows) > limit {
					cur.WriteString(fmt.Sprintf("BT /F1 8 Tf 40 %d Td (%s) Tj ET\n", y, pdfEscape(fmt.Sprintf("… %d linhas no total", len(b.Rows)))))
					y -= 14
				}
				y -= 8
			default:
				for _, line := range wrapPDF(b.Text, 92) {
					need(14)
					cur.WriteString(fmt.Sprintf("BT /F1 10 Tf 40 %d Td (%s) Tj ET\n", y, pdfEscape(line)))
					y -= 14
				}
				y -= 8
			}
		}
	}
	flush()
	if len(pages) == 0 {
		startPage(title)
		flush()
	}

	nPages := len(pages)
	fontObj := 3 + nPages*2
	objs := make([]string, 0, fontObj+1)
	kids := make([]string, 0, nPages)
	objs = append(objs, "") // catalog filled later
	objs = append(objs, "") // pages filled later
	for i, p := range pages {
		pageNo := 3 + i*2
		contentNo := pageNo + 1
		kids = append(kids, fmt.Sprintf("%d 0 R", pageNo))
		objs = append(objs, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents %d 0 R /Resources << /Font << /F1 %d 0 R /F2 %d 0 R >> >> >>", contentNo, fontObj, fontObj+1))
		objs = append(objs, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(p.body), p.body))
	}
	objs = append(objs, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	objs = append(objs, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>")
	objs[0] = "<< /Type /Catalog /Pages 2 0 R >>"
	objs[1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), nPages)

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs)+1)
	for i, obj := range objs {
		offsets[i+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for i := 1; i <= len(objs); i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buf, "trailer << /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return buf.Bytes()
}

func wrapPDF(s string, width int) []string {
	s = strings.ReplaceAll(fmt.Sprint(s), "\n", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{"—"}
	}
	var out []string
	for len(s) > 0 {
		if len(s) <= width {
			out = append(out, s)
			break
		}
		cut := strings.LastIndex(s[:width], " ")
		if cut < 20 {
			cut = width
		}
		out = append(out, strings.TrimSpace(s[:cut]))
		s = strings.TrimSpace(s[cut:])
		if len(out) >= 12 {
			out = append(out, "…")
			break
		}
	}
	return out
}

func pdfEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")
	s = strings.Map(func(r rune) rune {
		if r < 32 || r > 126 {
			return ' '
		}
		return r
	}, s)
	return s
}

func clipPDF(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
