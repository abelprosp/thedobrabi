package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

var connectorHTTP = &http.Client{
	Timeout: 20 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("demasiados redireccionamentos")
		}
		if err := assertHTTPURL(req.URL.String()); err != nil {
			return err
		}
		return nil
	},
}

func assertHTTPURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("URL inválido")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("apenas http/https são permitidos")
	}
	return nil
}

func (e *Engine) pingHTTP(ctx context.Context, cfg SQLConfig) error {
	_, _, err := e.doHTTP(ctx, cfg, http.MethodGet)
	return err
}

func (e *Engine) fetchJSON(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	body, ctype, err := e.doHTTP(ctx, cfg, http.MethodGet)
	if err != nil {
		return nil, nil, err
	}
	headers, rows, err := parseJSON(body)
	if err != nil {
		return nil, nil, fmt.Errorf("a resposta não é um array JSON simples (%s): %w", ctype, err)
	}
	return headers, rows, nil
}

func (e *Engine) fetchOData(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	u := strings.TrimSpace(cfg.URL)
	if u == "" {
		return nil, nil, fmt.Errorf("URL obrigatório")
	}
	var all []map[string]any
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		if seen[u] {
			break
		}
		seen[u] = true
		cfg.URL = u
		body, _, err := e.doHTTP(ctx, cfg, http.MethodGet)
		if err != nil {
			return nil, nil, err
		}
		page, next, err := parseODataPage(body)
		if err != nil {
			return nil, nil, err
		}
		all = append(all, page...)
		if next == "" || len(all) >= cfg.RowLimit() {
			break
		}
		nextURL, err := url.Parse(next)
		if err != nil {
			break
		}
		if !nextURL.IsAbs() {
			base, _ := url.Parse(u)
			nextURL = base.ResolveReference(nextURL)
		}
		u = nextURL.String()
	}
	return mapsToRows(mapsLimited(all, cfg.RowLimit()))
}

func parseODataPage(raw []byte) ([]map[string]any, string, error) {
	trim := bytes.TrimSpace(raw)
	if len(trim) > 0 && trim[0] == '[' {
		arr, err := pickJSONArray(trim)
		return arr, "", err
	}
	var obj map[string]any
	if err := json.Unmarshal(trim, &obj); err != nil {
		return nil, "", err
	}
	next, _ := obj["@odata.nextLink"].(string)
	if next == "" {
		next, _ = obj["odata.nextLink"].(string)
	}
	arr, err := pickJSONArray(trim, "value", "data", "results")
	return arr, next, err
}

func (e *Engine) doHTTP(ctx context.Context, cfg SQLConfig, method string) ([]byte, string, error) {
	rawURL := strings.TrimSpace(cfg.URL)
	if rawURL == "" {
		return nil, "", fmt.Errorf("URL obrigatório")
	}
	if err := assertHTTPURL(rawURL); err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	token := cfg.AuthToken()
	if token != "" && req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range cfg.Headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	resp, err := connectorHTTP.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("pedido HTTP falhou: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 180))
	}
	return body, resp.Header.Get("Content-Type"), nil
}

func (e *Engine) SyncHTTP(ctx context.Context, orgID, wsID, userID, sourceID uuid.UUID, typ string, cfg SQLConfig, datasetName string) (Result, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return Result{}, fmt.Errorf("este conector não tem URL — carregue um ficheiro JSON ou configure o endpoint")
	}
	headers, rows, err := e.fetchJSON(ctx, cfg)
	if err != nil {
		return Result{}, err
	}
	name := datasetName
	if name == "" {
		name = cfg.FileName
	}
	if name == "" {
		name = "api_import"
	}
	res, err := e.ingestRows(ctx, orgID, wsID, userID, name, typ, headers, rows)
	if err != nil {
		return Result{}, err
	}
	e.LinkDataset(ctx, orgID, sourceID, res.DatasetID)
	_ = e.writeLake(ctx, orgID, wsID, res.DatasetID, headers, rows)
	return res, nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
