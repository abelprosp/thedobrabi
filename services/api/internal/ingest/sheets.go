package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	googleSheetsAPIBase  = "https://sheets.googleapis.com/v4"
	googleSheetsDocsBase = "https://docs.google.com"
)

const sheetsShareHelp = "Partilhe a planilha: Partilhar → Qualquer pessoa com o link → Leitor. Não precisa de chave de API."

var (
	sheetIDTitleRe = regexp.MustCompile(`"sheetId"\s*:\s*(-?\d+)\s*,\s*"title"\s*:\s*"((?:\\.|[^"\\])*)"`)
	sheetTitleIDRe = regexp.MustCompile(`"title"\s*:\s*"((?:\\.|[^"\\])*)"\s*,\s*"sheetId"\s*:\s*(-?\d+)`)
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
	if titles := e.listPublicSheetTitles(ctx, cfg); len(titles) > 0 {
		return DiscoverResult{Tables: titles}, nil
	}
	if strings.TrimSpace(cfg.Table) != "" {
		return DiscoverResult{Tables: []string{cfg.Table}}, nil
	}
	return DiscoverResult{Tables: []string{"folha"}, Message: "A primeira folha será usada no sync. Indique o nome ou o gid no URL para escolher outra."}, nil
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
			return nil, fmt.Errorf("sem permissão na planilha (HTTP %d). %s", status, sheetsShareHelp)
		}
		if status == 404 {
			return nil, fmt.Errorf("planilha não encontrada. Confirme o URL. %s", sheetsShareHelp)
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
		headers, rows, err := parseGoogleSheetBody(raw, cfg.RowLimit())
		if err != nil {
			last = err
			continue
		}
		return headers, rows, nil
	}
	return nil, nil, sheetsPublicErr(last)
}

func sheetsPublicErr(last error) error {
	if last == nil {
		return fmt.Errorf("não consegui ler a planilha. %s", sheetsShareHelp)
	}
	msg := last.Error()
	if strings.Contains(msg, "partilh") || strings.Contains(msg, "pública") || strings.Contains(msg, "acessível") || strings.Contains(msg, "login") {
		return fmt.Errorf("a planilha não está acessível. %s", sheetsShareHelp)
	}
	return fmt.Errorf("não consegui ler a planilha: %w. %s", last, sheetsShareHelp)
}

func publicSheetURLs(ref googleSheetRef) []string {
	id := url.PathEscape(ref.ID)
	prefix := "/spreadsheets/d/"
	if ref.Published {
		prefix = "/spreadsheets/d/e/"
	}
	base := googleSheetsDocsBase + prefix + id

	gviz := url.Values{}
	gviz.Set("tqx", "out:csv")
	gviz.Set("headers", "1")
	if ref.Title != "" {
		gviz.Set("sheet", ref.Title)
	}
	if ref.GID != "" {
		gviz.Set("gid", ref.GID)
	}
	gvizJSON := url.Values{}
	for k, vs := range gviz {
		gvizJSON[k] = append([]string(nil), vs...)
	}
	gvizJSON.Set("tqx", "out:json")

	out := []string{
		base + "/gviz/tq?" + gviz.Encode(),
		base + "/gviz/tq?" + gvizJSON.Encode(),
	}
	if !ref.Published {
		exp := url.Values{"format": {"csv"}, "usp": {"sharing"}}
		if ref.GID != "" {
			exp.Set("gid", ref.GID)
		} else {
			exp.Set("gid", "0")
		}
		out = append(out, googleSheetsDocsBase+"/spreadsheets/d/"+id+"/export?"+exp.Encode())
	}
	pub := url.Values{"output": {"csv"}}
	if ref.GID != "" {
		pub.Set("gid", ref.GID)
		pub.Set("single", "true")
	}
	out = append(out, base+"/pub?"+pub.Encode())
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
	req.Header.Set("Accept", "text/csv,application/json,text/plain,*/*")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	resp, err := sheetsHTTP.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "partilh") || strings.Contains(err.Error(), "login") {
			return nil, err
		}
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if host := resp.Request.URL.Host; strings.Contains(strings.ToLower(host), "accounts.google") {
		return nil, fmt.Errorf("a planilha não está partilhada")
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("a planilha não está partilhada")
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 160))
	}
	return raw, nil
}

var sheetsHTTP = &http.Client{
	Timeout: 25 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 8 {
			return fmt.Errorf("demasiados redireccionamentos")
		}
		if strings.Contains(strings.ToLower(req.URL.Host), "accounts.google") {
			return fmt.Errorf("a planilha não está partilhada")
		}
		return assertHTTPURL(req.URL.String())
	},
}

func parseGoogleSheetBody(raw []byte, limit int) ([]string, [][]string, error) {
	raw = bytes.TrimPrefix(bytes.TrimSpace(raw), []byte{0xEF, 0xBB, 0xBF})
	if len(raw) == 0 {
		return nil, nil, fmt.Errorf("resposta vazia")
	}
	if js, ok := extractGvizJSON(raw); ok {
		return parseGvizTable(js, limit)
	}
	if looksLikeGoogleGate(raw) {
		return nil, nil, fmt.Errorf("a planilha não está partilhada")
	}
	if looksLikeHTML(raw) {
		return nil, nil, fmt.Errorf("a planilha não está partilhada")
	}
	headers, rows, err := parseCSV(raw)
	if err != nil {
		return nil, nil, err
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return headers, rows, nil
}

func extractGvizJSON(raw []byte) ([]byte, bool) {
	s := bytes.TrimSpace(raw)
	s = bytes.TrimPrefix(s, []byte("/*O_o*/"))
	s = bytes.TrimSpace(s)
	const prefix = "google.visualization.Query.setResponse("
	i := bytes.Index(s, []byte(prefix))
	if i < 0 {
		if len(s) > 0 && s[0] == '{' && bytes.Contains(s, []byte(`"table"`)) {
			return s, true
		}
		return nil, false
	}
	s = bytes.TrimSpace(s[i+len(prefix):])
	s = bytes.TrimSuffix(s, []byte(";"))
	s = bytes.TrimSpace(s)
	s = bytes.TrimSuffix(s, []byte(")"))
	return s, true
}

func parseGvizTable(raw []byte, limit int) ([]string, [][]string, error) {
	var parsed struct {
		Status string `json:"status"`
		Errors []struct {
			Reason          string `json:"reason"`
			Message         string `json:"message"`
			DetailedMessage string `json:"detailed_message"`
		} `json:"errors"`
		Table struct {
			Cols []struct {
				ID    string `json:"id"`
				Label string `json:"label"`
			} `json:"cols"`
			Rows []struct {
				C []struct {
					V any    `json:"v"`
					F string `json:"f"`
				} `json:"c"`
			} `json:"rows"`
		} `json:"table"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, nil, fmt.Errorf("resposta gviz inválida")
	}
	st := strings.ToLower(strings.TrimSpace(parsed.Status))
	if st == "error" || (st == "warning" && len(parsed.Table.Cols) == 0) {
		msg := "a planilha não está acessível"
		if len(parsed.Errors) > 0 {
			if parsed.Errors[0].DetailedMessage != "" {
				msg = parsed.Errors[0].DetailedMessage
			} else if parsed.Errors[0].Message != "" {
				msg = parsed.Errors[0].Message
			}
			reason := strings.ToLower(parsed.Errors[0].Reason)
			if strings.Contains(reason, "access") || strings.Contains(reason, "denied") || strings.Contains(reason, "not_logged") {
				return nil, nil, fmt.Errorf("a planilha não está partilhada")
			}
		}
		return nil, nil, fmt.Errorf("%s", msg)
	}
	if len(parsed.Table.Cols) == 0 {
		return nil, nil, fmt.Errorf("a folha está vazia")
	}
	headers := make([]string, len(parsed.Table.Cols))
	for i, c := range parsed.Table.Cols {
		h := strings.TrimSpace(c.Label)
		if h == "" {
			h = strings.TrimSpace(c.ID)
		}
		if h == "" {
			h = fmt.Sprintf("col_%d", i+1)
		}
		headers[i] = h
	}
	values := make([][]any, 0, len(parsed.Table.Rows)+1)
	headerVals := make([]any, len(headers))
	for i, h := range headers {
		headerVals[i] = h
	}
	values = append(values, headerVals)
	for _, row := range parsed.Table.Rows {
		rec := make([]any, len(headers))
		for i := range headers {
			if i >= len(row.C) {
				continue
			}
			cell := row.C[i]
			if s, ok := cell.V.(string); ok && strings.HasPrefix(s, "Date(") && cell.F != "" {
				rec[i] = cell.F
				continue
			}
			if cell.V == nil && cell.F != "" {
				rec[i] = cell.F
				continue
			}
			rec[i] = cell.V
		}
		values = append(values, rec)
	}
	return sheetsValuesToTable(values, limit)
}

func (e *Engine) listPublicSheetTitles(ctx context.Context, cfg SQLConfig) []string {
	ref, err := parseGoogleSheetRef(cfg.URL, cfg.Table, cfg.Query)
	if err != nil {
		return nil
	}
	prefix := "/spreadsheets/d/"
	if ref.Published {
		prefix = "/spreadsheets/d/e/"
	}
	candidates := []string{
		googleSheetsDocsBase + prefix + url.PathEscape(ref.ID) + "/htmlview",
		googleSheetsDocsBase + prefix + url.PathEscape(ref.ID) + "/pubhtml",
	}
	for _, u := range candidates {
		raw, err := downloadPublicSheet(ctx, u)
		if err != nil || looksLikeGoogleGate(raw) {
			continue
		}
		if titles := parseSheetTitlesHTML(raw); len(titles) > 0 {
			return titles
		}
	}
	return nil
}

func parseSheetTitlesHTML(raw []byte) []string {
	seen := map[string]bool{}
	var out []string
	add := func(title string) {
		title = unescapeJSONString(strings.TrimSpace(title))
		if title == "" || seen[title] {
			return
		}
		seen[title] = true
		out = append(out, title)
	}
	for _, m := range sheetIDTitleRe.FindAllSubmatch(raw, -1) {
		add(string(m[2]))
	}
	for _, m := range sheetTitleIDRe.FindAllSubmatch(raw, -1) {
		add(string(m[1]))
	}
	return out
}

func unescapeJSONString(s string) string {
	s = strings.ReplaceAll(s, `\/`, `/`)
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}

func looksLikeGoogleGate(raw []byte) bool {
	if !looksLikeHTML(raw) {
		return false
	}
	n := min(len(raw), 4096)
	low := bytes.ToLower(raw[:n])
	return bytes.Contains(low, []byte("accounts.google")) ||
		bytes.Contains(low, []byte("servicelogin")) ||
		bytes.Contains(low, []byte("signin")) ||
		bytes.Contains(low, []byte("sign in")) ||
		bytes.Contains(low, []byte("google-signin"))
}

func looksLikeHTML(raw []byte) bool {
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 {
		return false
	}
	low := bytes.ToLower(trim[:min(len(trim), 256)])
	return bytes.HasPrefix(low, []byte("<!doctype")) || bytes.HasPrefix(low, []byte("<html")) || bytes.Contains(low, []byte("<html"))
}
