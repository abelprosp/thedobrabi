package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

func supabaseProjectURL(cfg SQLConfig) string {
	u := strings.TrimSpace(cfg.ProjectURL)
	if u == "" {
		raw := strings.TrimSpace(cfg.URL)
		low := strings.ToLower(raw)
		if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
			u = raw
		}
	}
	u = strings.TrimRight(u, "/")
	u = strings.TrimSuffix(u, "/rest/v1")
	return strings.TrimRight(u, "/")
}

func supabaseRESTKey(cfg SQLConfig) string {
	for _, v := range []string{cfg.ServiceRoleKey, cfg.AnonKey, cfg.APIKey, cfg.AccessToken, cfg.Token} {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func supabaseDBHostFromProject(projectURL string) string {
	u, err := url.Parse(strings.TrimSpace(projectURL))
	if err != nil || u.Host == "" {
		return ""
	}
	host := u.Hostname()
	if host == "" {
		return ""
	}
	if strings.HasSuffix(host, ".supabase.co") && !strings.HasPrefix(host, "db.") {
		return "db." + host
	}
	return host
}

func supabasePreparedSQL(cfg SQLConfig) SQLConfig {
	if strings.TrimSpace(cfg.Host) == "" {
		cfg.Host = supabaseDBHostFromProject(supabaseProjectURL(cfg))
	}
	if cfg.Port == 0 {
		cfg.Port = 5432
	}
	if strings.TrimSpace(cfg.Database) == "" {
		cfg.Database = "postgres"
	}
	if strings.TrimSpace(cfg.User) == "" {
		cfg.User = "postgres"
	}
	if strings.TrimSpace(cfg.SSLMode) == "" && !cfg.SSL {
		cfg.SSLMode = "require"
	}
	return cfg
}

func supabaseUsePostgres(cfg SQLConfig) bool {
	if strings.TrimSpace(cfg.Password) == "" {
		return false
	}
	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		host = supabaseDBHostFromProject(supabaseProjectURL(cfg))
	}
	return host != ""
}

func supabaseUseREST(cfg SQLConfig) bool {
	return supabaseProjectURL(cfg) != "" && supabaseRESTKey(cfg) != ""
}

func supabaseRESTTable(table string) string {
	t := strings.TrimSpace(table)
	if i := strings.LastIndex(t, "."); i >= 0 {
		t = t[i+1:]
	}
	return t
}

func supabaseRESTHeaders(key string) map[string]string {
	return map[string]string{
		"apikey":        key,
		"Authorization": "Bearer " + key,
		"Accept":        "application/json",
	}
}

func supabaseModeError() error {
	return fmt.Errorf("indique o anfitrião e a senha da base (Postgres) ou o URL do projecto e a service_role_key/anon_key (API REST /rest/v1/)")
}

func (e *Engine) discoverSupabase(ctx context.Context, cfg SQLConfig) ([]string, error) {
	if supabaseUsePostgres(cfg) {
		return e.discoverSupabaseSQL(ctx, cfg)
	}
	if supabaseUseREST(cfg) {
		return e.discoverSupabaseREST(ctx, cfg)
	}
	return nil, supabaseModeError()
}

func (e *Engine) discoverSupabaseSQL(ctx context.Context, cfg SQLConfig) ([]string, error) {
	cfg = supabasePreparedSQL(cfg)
	conn, err := pgx.Connect(ctx, postgresDSN(cfg))
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)
	rows, err := conn.Query(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE' ORDER BY table_name LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, "public."+name)
	}
	if out == nil {
		out = []string{}
	}
	return out, rows.Err()
}

func (e *Engine) discoverSupabaseREST(ctx context.Context, cfg SQLConfig) ([]string, error) {
	base := supabaseProjectURL(cfg)
	key := supabaseRESTKey(cfg)
	u := base + "/rest/v1/"
	headers := supabaseRESTHeaders(key)
	headers["Accept"] = "application/openapi+json, application/json"
	raw, status, err := httpJSON(ctx, http.MethodGet, u, headers, nil, "", "")
	if err != nil {
		return nil, err
	}
	if err := mustOK(status, raw); err != nil {
		return nil, err
	}
	tables := parsePostgRESTOpenAPI(raw)
	if t := supabaseRESTTable(cfg.Table); t != "" {
		found := false
		for _, x := range tables {
			if x == t {
				found = true
				break
			}
		}
		if !found {
			tables = append([]string{t}, tables...)
		}
	}
	if len(tables) == 0 {
		if t := supabaseRESTTable(cfg.Table); t != "" {
			return []string{t}, nil
		}
		return nil, fmt.Errorf("nenhuma tabela no OpenAPI /rest/v1/ — indique o campo tabela")
	}
	return tables, nil
}

func parsePostgRESTOpenAPI(raw []byte) []string {
	var spec struct {
		Paths map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil || spec.Paths == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for p := range spec.Paths {
		p = strings.TrimSpace(p)
		p = strings.TrimPrefix(p, "/")
		if p == "" || strings.Contains(p, "{") || strings.HasPrefix(p, "rpc/") || strings.Contains(p, "/") {
			continue
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func (e *Engine) fetchSupabase(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	if supabaseUsePostgres(cfg) {
		cfg = supabasePreparedSQL(cfg)
		if cfg.Selection != nil && !cfg.Selection.empty() {
			applied, err := applySelection("postgres", cfg)
			if err != nil {
				return nil, nil, err
			}
			cfg = applied
		}
		return e.readSQL(ctx, "postgres", cfg)
	}
	if supabaseUseREST(cfg) {
		return e.fetchSupabaseREST(ctx, cfg)
	}
	return nil, nil, supabaseModeError()
}

func (e *Engine) fetchSupabaseREST(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	if cfg.Selection != nil && !cfg.Selection.empty() {
		return e.fetchSupabaseRESTSelection(ctx, cfg)
	}
	table := supabaseRESTTable(cfg.Table)
	if table == "" {
		return nil, nil, fmt.Errorf("indique uma tabela para o sync REST do Supabase")
	}
	if !tableIdentOK(table) {
		return nil, nil, fmt.Errorf("nome de tabela inválido")
	}
	return e.fetchSupabaseRESTTable(ctx, cfg, table, nil)
}

func (e *Engine) fetchSupabaseRESTSelection(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	sel := cfg.Selection
	if len(sel.Tables) == 1 {
		t := sel.Tables[0]
		return e.fetchSupabaseRESTTable(ctx, cfg, t.Name, t.Columns)
	}
	type packed struct {
		key     string
		headers []string
		rows    [][]string
	}
	byKey := map[string]packed{}
	var order []packed
	for _, t := range sel.Tables {
		headers, rows, err := e.fetchSupabaseRESTTable(ctx, cfg, t.Name, t.Columns)
		if err != nil {
			return nil, nil, err
		}
		p := packed{key: t.Key(), headers: headers, rows: rows}
		order = append(order, p)
		byKey[p.key] = p
		byKey[t.Name] = p
	}
	left := order[0]
	joined := map[string]bool{order[0].key: true}
	pending := append([]SelectedJoin(nil), sel.Joins...)
	for len(pending) > 0 {
		progress := false
		next := pending[:0]
		for _, j := range pending {
			lp, okL := byKey[j.LeftTable]
			rp, okR := byKey[j.RightTable]
			if !okL || !okR {
				return nil, nil, fmt.Errorf("cruzamento refere uma lista que não foi escolhida")
			}
			allLeft := j.Match == "all_left" || j.Match == "left"
			leftIn, rightIn := joined[lp.key], joined[rp.key]
			if leftIn && rightIn {
				progress = true
				continue
			}
			if !leftIn && !rightIn {
				next = append(next, j)
				continue
			}
			var headers []string
			var rows [][]string
			var err error
			var add packed
			if leftIn {
				headers, rows, err = joinInMemory(left.headers, left.rows, j.LeftColumn, rp.headers, rp.rows, j.RightColumn, allLeft)
				add = rp
			} else {
				headers, rows, err = joinInMemory(left.headers, left.rows, j.RightColumn, lp.headers, lp.rows, j.LeftColumn, allLeft)
				add = lp
			}
			if err != nil {
				return nil, nil, err
			}
			left.headers, left.rows = headers, rows
			joined[add.key] = true
			progress = true
		}
		if !progress {
			return nil, nil, fmt.Errorf("não foi possível ligar todas as listas — verifique os cruzamentos")
		}
		pending = next
	}
	return left.headers, left.rows, nil
}

func (e *Engine) fetchSupabaseRESTTable(ctx context.Context, cfg SQLConfig, table string, cols []string) ([]string, [][]string, error) {
	table = supabaseRESTTable(table)
	if table == "" {
		return nil, nil, fmt.Errorf("indique uma tabela para o sync REST do Supabase")
	}
	if !tableIdentOK(table) {
		return nil, nil, fmt.Errorf("nome de tabela inválido")
	}
	lim := cfg.RowLimit()
	if lim > 500 {
		lim = 500
	}
	base := supabaseProjectURL(cfg)
	key := supabaseRESTKey(cfg)
	sel := restSelectParam(cols)
	u := fmt.Sprintf("%s/rest/v1/%s?select=%s&limit=%d", base, url.PathEscape(table), url.QueryEscape(sel), lim)
	raw, status, err := httpJSON(ctx, http.MethodGet, u, supabaseRESTHeaders(key), nil, "", "")
	if err != nil {
		return nil, nil, err
	}
	if err := mustOK(status, raw); err != nil {
		return nil, nil, err
	}
	maps, err := pickJSONArray(raw, "data", "value", "results")
	if err != nil {
		return nil, nil, err
	}
	return mapsToRows(mapsLimited(maps, lim))
}

func (e *Engine) pingSupabase(ctx context.Context, cfg SQLConfig) error {
	if supabaseUsePostgres(cfg) {
		cfg = supabasePreparedSQL(cfg)
		conn, err := pgx.Connect(ctx, postgresDSN(cfg))
		if err != nil {
			return err
		}
		defer conn.Close(ctx)
		return conn.Ping(ctx)
	}
	if supabaseUseREST(cfg) {
		base := supabaseProjectURL(cfg)
		key := supabaseRESTKey(cfg)
		u := base + "/rest/v1/"
		if t := supabaseRESTTable(cfg.Table); t != "" && tableIdentOK(t) {
			u = fmt.Sprintf("%s/rest/v1/%s?select=*&limit=1", base, url.PathEscape(t))
		}
		raw, status, err := httpJSON(ctx, http.MethodGet, u, supabaseRESTHeaders(key), nil, "", "")
		if err != nil {
			return err
		}
		return mustOK(status, raw)
	}
	return supabaseModeError()
}
