package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

var (
	googleSheetsAPIBase  = "https://sheets.googleapis.com/v4"
	googleSheetsDocsBase = "https://docs.google.com"
)

type googleSheetRef struct {
	ID        string
	GID       string
	Title     string
	A1        string
	Published bool
}

func parseGoogleSheetRef(raw, table, a1 string) (googleSheetRef, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return googleSheetRef{}, fmt.Errorf("indique o URL ou o ID da planilha Google")
	}
	ref := googleSheetRef{A1: strings.TrimSpace(a1)}
	if strings.Contains(raw, "://") || strings.Contains(raw, "docs.google.com") {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			return googleSheetRef{}, fmt.Errorf("URL da planilha inválido")
		}
		if g := u.Query().Get("gid"); g != "" {
			ref.GID = g
		}
		if ref.GID == "" {
			frag := u.Fragment
			if i := strings.Index(frag, "gid="); i >= 0 {
				ref.GID = frag[i+4:]
				if j := strings.IndexAny(ref.GID, "&/#"); j >= 0 {
					ref.GID = ref.GID[:j]
				}
			}
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		for i, p := range parts {
			if p != "d" || i+1 >= len(parts) {
				continue
			}
			if parts[i+1] == "e" && i+2 < len(parts) {
				ref.ID = parts[i+2]
				ref.Published = true
			} else {
				ref.ID = parts[i+1]
			}
			break
		}
		if ref.ID == "" {
			return googleSheetRef{}, fmt.Errorf("não encontrei o ID da planilha no URL")
		}
	} else {
		if !googleSheetIDOK(raw) {
			return googleSheetRef{}, fmt.Errorf("ID da planilha inválido")
		}
		ref.ID = raw
		ref.Published = strings.HasPrefix(raw, "2PACX")
	}

	table = strings.TrimSpace(table)
	if table != "" {
		if isAllDigits(table) {
			ref.GID = table
		} else {
			ref.Title = table
		}
	}
	return ref, nil
}

func googleSheetIDOK(id string) bool {
	if len(id) < 10 || len(id) > 200 {
		return false
	}
	for _, r := range id {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func googleSheetTitleBare(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		ok := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_'
		if i > 0 && r >= '0' && r <= '9' {
			ok = true
		}
		if !ok {
			return false
		}
	}
	return true
}

func googleSheetsA1(title, extra string) string {
	extra = strings.TrimSpace(extra)
	title = strings.TrimSpace(title)
	if title == "" {
		return extra
	}
	quoted := title
	if !googleSheetTitleBare(title) {
		quoted = "'" + strings.ReplaceAll(title, "'", "''") + "'"
	}
	if extra == "" {
		return quoted
	}
	if strings.Contains(extra, "!") {
		return extra
	}
	return quoted + "!" + extra
}

func sheetsCellString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		if x == float64(int64(x)) && x >= -1e15 && x <= 1e15 {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(x)
	}
}

func sheetsValuesToTable(values [][]any, limit int) ([]string, [][]string, error) {
	if len(values) == 0 {
		return nil, nil, fmt.Errorf("a folha está vazia")
	}
	headerRow := values[0]
	if len(headerRow) == 0 {
		return nil, nil, fmt.Errorf("a primeira linha não tem colunas")
	}
	headers := make([]string, len(headerRow))
	for i, cell := range headerRow {
		h := strings.TrimSpace(sheetsCellString(cell))
		if h == "" {
			h = fmt.Sprintf("col_%d", i+1)
		}
		headers[i] = h
	}
	if limit <= 0 {
		limit = 10000
	}
	rows := make([][]string, 0, min(len(values)-1, limit))
	for _, rec := range values[1:] {
		if len(rows) >= limit {
			break
		}
		empty := true
		out := make([]string, len(headers))
		for i := range headers {
			if i < len(rec) {
				out[i] = sheetsCellString(rec[i])
				if strings.TrimSpace(out[i]) != "" {
					empty = false
				}
			}
		}
		if empty {
			continue
		}
		rows = append(rows, out)
	}
	return headers, rows, nil
}

func (e *Engine) fetchGoogleSheets(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	ref, err := parseGoogleSheetRef(cfg.URL, cfg.Table, cfg.Query)
	if err != nil {
		return nil, nil, err
	}
	if tok := cfg.AuthToken(); tok != "" || strings.TrimSpace(cfg.APIKey) != "" {
		return e.fetchGoogleSheetsAPI(ctx, cfg, ref)
	}
	return e.fetchGoogleSheetsPublic(ctx, cfg, ref)
}

func (e *Engine) pingGoogleSheets(ctx context.Context, cfg SQLConfig) error {
	_, _, err := e.fetchGoogleSheets(ctx, cfg)
	if err != nil && (strings.Contains(err.Error(), "vazia") || strings.Contains(err.Error(), "nenhuma linha")) {
		return nil
	}
	return err
}

func (e *Engine) discoverGoogleSheets(ctx context.Context, cfg SQLConfig) (DiscoverResult, error) {
	if cfg.AuthToken() != "" || strings.TrimSpace(cfg.APIKey) != "" {
		titles, err := e.listGoogleSheetTitles(ctx, cfg)
		if err == nil && len(titles) > 0 {
			return DiscoverResult{Tables: titles}, nil
		}
	}
	if strings.TrimSpace(cfg.Table) != "" {
		return DiscoverResult{Tables: []string{cfg.Table}}, nil
	}
	return DiscoverResult{Tables: []string{"folha"}, Message: "A primeira folha será usada no sync. Indique o nome ou o gid para escolher outra."}, nil
}

func (e *Engine) listGoogleSheetTitles(ctx context.Context, cfg SQLConfig) ([]string, error) {
	ref, err := parseGoogleSheetRef(cfg.URL, cfg.Table, cfg.Query)
	if err != nil {
		return nil, err
	}
	sheets, err := e.listGoogleSheetMeta(ctx, cfg, ref.ID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(sheets))
	for _, s := range sheets {
		if s.Title != "" {
			out = append(out, s.Title)
		}
	}
	return out, nil
}

type googleSheetMeta struct {
	Title string
	GID   int64
}

func (e *Engine) listGoogleSheetMeta(ctx context.Context, cfg SQLConfig, id string) ([]googleSheetMeta, error) {
	u := googleSheetsAPIBase + "/spreadsheets/" + url.PathEscape(id) + "?fields=sheets.properties(sheetId,title)"
	raw, err := e.googleSheetsGET(ctx, cfg, u)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Sheets []struct {
			Properties struct {
				SheetID int64  `json:"sheetId"`
				Title   string `json:"title"`
			} `json:"properties"`
		} `json:"sheets"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("resposta inválida da Sheets API: %w", err)
	}
	out := make([]googleSheetMeta, 0, len(parsed.Sheets))
	for _, s := range parsed.Sheets {
		out = append(out, googleSheetMeta{Title: s.Properties.Title, GID: s.Properties.SheetID})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("a planilha não tem folhas")
	}
	return out, nil
}

func (e *Engine) fetchGoogleSheetsAPI(ctx context.Context, cfg SQLConfig, ref googleSheetRef) ([]string, [][]string, error) {
	title := ref.Title
	if title == "" {
		sheets, err := e.listGoogleSheetMeta(ctx, cfg, ref.ID)
		if err != nil {
			return nil, nil, err
		}
		title = sheets[0].Title
		if ref.GID != "" {
			want, _ := strconv.ParseInt(ref.GID, 10, 64)
			for _, s := range sheets {
				if s.GID == want {
					title = s.Title
					break
				}
			}
		}
	}
	rng := googleSheetsA1(title, ref.A1)
	if rng == "" {
		rng = title
	}
	u := googleSheetsAPIBase + "/spreadsheets/" + url.PathEscape(ref.ID) + "/values/" + url.PathEscape(rng)
	u += "?majorDimension=ROWS&valueRenderOption=UNFORMATTED_VALUE&dateTimeRenderOption=FORMATTED_STRING"
	raw, err := e.googleSheetsGET(ctx, cfg, u)
	if err != nil {
		return nil, nil, err
	}
	var parsed struct {
		Values [][]any `json:"values"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, nil, fmt.Errorf("resposta inválida da Sheets API: %w", err)
	}
	return sheetsValuesToTable(parsed.Values, cfg.RowLimit())
}

func (e *Engine) googleSheetsGET(ctx context.Context, cfg SQLConfig, rawURL string) ([]byte, error) {
	if key := strings.TrimSpace(cfg.APIKey); key != "" {
		sep := "&"
		if !strings.Contains(rawURL, "?") {
			sep = "?"
		}
		rawURL += sep + "key=" + url.QueryEscape(key)
	}
	headers := map[string]string{}
	if tok := cfg.AuthToken(); tok != "" {
		headers["Authorization"] = "Bearer " + tok
	}
	raw, status, err := httpJSON(ctx, http.MethodGet, rawURL, headers, nil, "", "")
	if err != nil {
		return nil, err
	}
	if err := mustOK(status, raw); err != nil {
		if status == 401 || status == 403 {
			return nil, fmt.Errorf("sem permissão na planilha (HTTP %d). Partilhe-a ou verifique o token / chave de API", status)
		}
		if status == 404 {
			return nil, fmt.Errorf("planilha não encontrada. Confirme o URL e se a Sheets API está activa")
		}
		return nil, err
	}
	return raw, nil
}

func (e *Engine) fetchGoogleSheetsPublic(ctx context.Context, cfg SQLConfig, ref googleSheetRef) ([]string, [][]string, error) {
	candidates := publicSheetURLs(ref)
	var last error
	for _, u := range candidates {
		raw, err := downloadPublicSheet(ctx, u)
		if err != nil {
			last = err
			continue
		}
		if looksLikeHTML(raw) {
			last = fmt.Errorf("a planilha não é pública")
			continue
		}
		raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
		headers, rows, err := parseCSV(raw)
		if err != nil {
			last = err
			continue
		}
		if len(rows) > cfg.RowLimit() {
			rows = rows[:cfg.RowLimit()]
		}
		return headers, rows, nil
	}
	if last != nil && strings.Contains(last.Error(), "pública") {
		return nil, nil, fmt.Errorf("a planilha não é pública. Partilhe com «qualquer pessoa com o link» ou indique uma chave de API / token OAuth")
	}
	if last != nil {
		return nil, nil, fmt.Errorf("falha a ler a planilha pública: %w. Partilhe-a ou indique uma chave de API", last)
	}
	return nil, nil, fmt.Errorf("não consegui ler a planilha. Partilhe-a publicamente ou indique uma chave de API")
}

func publicSheetURLs(ref googleSheetRef) []string {
	var out []string
	q := url.Values{}
	q.Set("tqx", "out:csv")
	if ref.Title != "" {
		q.Set("sheet", ref.Title)
	}
	if ref.GID != "" {
		q.Set("gid", ref.GID)
	}
	if !ref.Published {
		out = append(out, googleSheetsDocsBase+"/spreadsheets/d/"+url.PathEscape(ref.ID)+"/gviz/tq?"+q.Encode())
		exp := url.Values{"format": {"csv"}}
		if ref.GID != "" {
			exp.Set("gid", ref.GID)
		} else {
			exp.Set("gid", "0")
		}
		out = append(out, googleSheetsDocsBase+"/spreadsheets/d/"+url.PathEscape(ref.ID)+"/export?"+exp.Encode())
	}
	pub := url.Values{"output": {"csv"}}
	if ref.GID != "" {
		pub.Set("gid", ref.GID)
		pub.Set("single", "true")
	}
	prefix := "/spreadsheets/d/"
	if ref.Published {
		prefix = "/spreadsheets/d/e/"
	}
	out = append(out, googleSheetsDocsBase+prefix+url.PathEscape(ref.ID)+"/pub?"+pub.Encode())
	return out
}

func downloadPublicSheet(ctx context.Context, rawURL string) ([]byte, error) {
	if err := assertHTTPURL(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/csv,text/plain,*/*")
	resp, err := connectorHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 160))
	}
	return raw, nil
}

func looksLikeHTML(raw []byte) bool {
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 {
		return false
	}
	low := bytes.ToLower(trim[:min(len(trim), 256)])
	return bytes.HasPrefix(low, []byte("<!doctype")) || bytes.HasPrefix(low, []byte("<html")) || bytes.Contains(low, []byte("<html"))
}
