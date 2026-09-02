package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func googleClient(ctx context.Context, token string, scopes ...string) (*http.Client, error) {
	trim := strings.TrimSpace(token)
	if trim == "" {
		return nil, fmt.Errorf("token Google obrigatório")
	}
	if strings.HasPrefix(trim, "{") {
		if len(scopes) == 0 {
			scopes = []string{
				"https://www.googleapis.com/auth/bigquery",
				"https://www.googleapis.com/auth/analytics.readonly",
				"https://www.googleapis.com/auth/adwords",
			}
		}
		conf, err := google.JWTConfigFromJSON([]byte(trim), scopes...)
		if err != nil {
			return nil, fmt.Errorf("JSON da conta de serviço inválido: %w", err)
		}
		return conf.Client(ctx), nil
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: trim})
	return oauth2.NewClient(ctx, ts), nil
}

func (e *Engine) pingBigQuery(ctx context.Context, cfg SQLConfig) error {
	if cfg.Project == "" {
		return fmt.Errorf("projecto obrigatório")
	}
	cli, err := googleClient(ctx, cfg.AuthToken(), "https://www.googleapis.com/auth/bigquery.readonly", "https://www.googleapis.com/auth/bigquery")
	if err != nil {
		return err
	}
	u := fmt.Sprintf("https://bigquery.googleapis.com/bigquery/v2/projects/%s/datasets", urlPath(cfg.Project))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 180))
	}
	return nil
}

func (e *Engine) discoverBigQuery(ctx context.Context, cfg SQLConfig) ([]string, error) {
	if cfg.Project == "" {
		return nil, fmt.Errorf("projecto obrigatório")
	}
	cli, err := googleClient(ctx, cfg.AuthToken(), "https://www.googleapis.com/auth/bigquery.readonly", "https://www.googleapis.com/auth/bigquery")
	if err != nil {
		return nil, err
	}
	ds := cfg.Dataset
	if ds == "" {
		u := fmt.Sprintf("https://bigquery.googleapis.com/bigquery/v2/projects/%s/datasets?maxResults=100", urlPath(cfg.Project))
		var payload struct {
			Datasets []struct {
				ID         string `json:"id"`
				DatasetRef struct {
					DatasetID string `json:"datasetId"`
				} `json:"datasetReference"`
			} `json:"datasets"`
		}
		if err := googleGET(ctx, cli, u, &payload); err != nil {
			return nil, err
		}
		out := []string{}
		for _, d := range payload.Datasets {
			id := d.DatasetRef.DatasetID
			if id == "" {
				id = d.ID
			}
			out = append(out, id)
		}
		return out, nil
	}
	u := fmt.Sprintf("https://bigquery.googleapis.com/bigquery/v2/projects/%s/datasets/%s/tables?maxResults=500", urlPath(cfg.Project), urlPath(ds))
	var payload struct {
		Tables []struct {
			TableRef struct {
				TableID string `json:"tableId"`
			} `json:"tableReference"`
		} `json:"tables"`
	}
	if err := googleGET(ctx, cli, u, &payload); err != nil {
		return nil, err
	}
	out := []string{}
	for _, t := range payload.Tables {
		out = append(out, t.TableRef.TableID)
	}
	return out, nil
}

func (e *Engine) readBigQuery(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	if cfg.Project == "" {
		return nil, nil, fmt.Errorf("projecto obrigatório")
	}
	cli, err := googleClient(ctx, cfg.AuthToken(), "https://www.googleapis.com/auth/bigquery.readonly", "https://www.googleapis.com/auth/bigquery")
	if err != nil {
		return nil, nil, err
	}
	q := strings.TrimSpace(cfg.Query)
	if q == "" {
		if cfg.Table == "" {
			return nil, nil, fmt.Errorf("indique dataset.tabela ou uma consulta")
		}
		ident := cfg.Table
		if cfg.Dataset != "" && !strings.Contains(ident, ".") {
			ident = cfg.Dataset + "." + ident
		}
		q = fmt.Sprintf("SELECT * FROM `%s.%s` LIMIT %d", cfg.Project, ident, cfg.RowLimit())
		if strings.Count(ident, ".") >= 1 && strings.Contains(ident, cfg.Project) {
			q = fmt.Sprintf("SELECT * FROM `%s` LIMIT %d", ident, cfg.RowLimit())
		}
	}
	body, _ := json.Marshal(map[string]any{"query": q, "useLegacySql": false, "maxResults": cfg.RowLimit()})
	u := fmt.Sprintf("https://bigquery.googleapis.com/bigquery/v2/projects/%s/queries", urlPath(cfg.Project))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := cli.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var parsed struct {
		Schema struct {
			Fields []struct {
				Name string `json:"name"`
			} `json:"fields"`
		} `json:"schema"`
		Rows []struct {
			F []struct {
				V any `json:"v"`
			} `json:"f"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, nil, err
	}
	headers := make([]string, len(parsed.Schema.Fields))
	for i, f := range parsed.Schema.Fields {
		headers[i] = f.Name
	}
	out := make([][]string, 0, len(parsed.Rows))
	for _, r := range parsed.Rows {
		rec := make([]string, len(headers))
		for i := range headers {
			if i < len(r.F) && r.F[i].V != nil {
				rec[i] = fmt.Sprint(r.F[i].V)
			}
		}
		out = append(out, rec)
	}
	return headers, out, nil
}

func googleGET(ctx context.Context, cli *http.Client, rawURL string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 180))
	}
	return json.Unmarshal(body, dest)
}

func urlPath(s string) string {
	return strings.Trim(s, "/")
}

func (e *Engine) pingDatabricks(ctx context.Context, cfg SQLConfig) error {
	host := strings.TrimPrefix(strings.TrimSpace(cfg.Host), "https://")
	host = strings.TrimPrefix(host, "http://")
	if host == "" {
		return fmt.Errorf("anfitrião obrigatório")
	}
	if cfg.AuthToken() == "" {
		return fmt.Errorf("token obrigatório")
	}
	u := "https://" + host + "/api/2.0/sql/warehouses"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.AuthToken())
	resp, err := connectorHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 180))
	}
	return nil
}

func databricksWarehouseID(cfg SQLConfig) string {
	p := cfg.HTTPPath
	if i := strings.LastIndex(p, "/"); i >= 0 && i < len(p)-1 {
		return p[i+1:]
	}
	return cfg.Catalog
}

func (e *Engine) databricksExec(ctx context.Context, cfg SQLConfig, statement string) ([]string, [][]string, error) {
	host := strings.TrimPrefix(strings.TrimSpace(cfg.Host), "https://")
	host = strings.TrimPrefix(host, "http://")
	wid := databricksWarehouseID(cfg)
	if wid == "" {
		return nil, nil, fmt.Errorf("http_path ou warehouse_id obrigatório")
	}
	payload, _ := json.Marshal(map[string]any{
		"warehouse_id": wid,
		"statement":    statement,
		"wait_timeout": "50s",
		"disposition":  "INLINE",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+host+"/api/2.0/sql/statements", bytes.NewReader(payload))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.AuthToken())
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 60 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var parsed struct {
		Status struct {
			State string `json:"state"`
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		} `json:"status"`
		Manifest struct {
			Schema struct {
				Columns []struct {
					Name string `json:"name"`
				} `json:"columns"`
			} `json:"schema"`
		} `json:"manifest"`
		Result struct {
			DataArray [][]any `json:"data_array"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, nil, err
	}
	if parsed.Status.State != "" && parsed.Status.State != "SUCCEEDED" {
		msg := parsed.Status.Error.Message
		if msg == "" {
			msg = parsed.Status.State
		}
		return nil, nil, fmt.Errorf("databricks: %s", msg)
	}
	headers := make([]string, len(parsed.Manifest.Schema.Columns))
	for i, c := range parsed.Manifest.Schema.Columns {
		headers[i] = c.Name
	}
	out := make([][]string, 0, len(parsed.Result.DataArray))
	for _, r := range parsed.Result.DataArray {
		rec := make([]string, len(headers))
		for i := range headers {
			if i < len(r) && r[i] != nil {
				rec[i] = fmt.Sprint(r[i])
			}
		}
		out = append(out, rec)
	}
	return headers, out, nil
}

func (e *Engine) discoverDatabricks(ctx context.Context, cfg SQLConfig) ([]string, error) {
	q := "SHOW TABLES"
	if cfg.Schema != "" {
		q = "SHOW TABLES IN " + cfg.Schema
	}
	headers, rows, err := e.databricksExec(ctx, cfg, q)
	if err != nil {
		return nil, err
	}
	out := []string{}
	nameIdx := 0
	for i, h := range headers {
		if strings.EqualFold(h, "tableName") || strings.EqualFold(h, "table_name") || strings.EqualFold(h, "name") {
			nameIdx = i
		}
	}
	for _, r := range rows {
		if nameIdx < len(r) && r[nameIdx] != "" {
			out = append(out, r[nameIdx])
		}
	}
	return out, nil
}

func (e *Engine) readDatabricks(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	q := strings.TrimSpace(cfg.Query)
	if q == "" {
		if cfg.Table == "" {
			return nil, nil, fmt.Errorf("indique uma tabela ou consulta")
		}
		q = fmt.Sprintf("SELECT * FROM %s LIMIT %d", cfg.Table, cfg.RowLimit())
	}
	return e.databricksExec(ctx, cfg, q)
}
