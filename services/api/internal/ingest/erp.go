package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (e *Engine) fetchERP(ctx context.Context, typ string, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	resource = strings.TrimSpace(resource)
	if resource == "" {
		switch typ {
		case "totvs_protheus":
			resource = "customers"
		case "sap_b1":
			resource = "BusinessPartners"
		default:
			resource = "employees"
		}
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if base == "" {
		return nil, nil, fmt.Errorf("URL da API obrigatória para %s", typ)
	}
	token := cfg.AuthToken()
	if token == "" {
		token = strings.TrimSpace(cfg.Token)
	}
	if token == "" {
		token = strings.TrimSpace(cfg.APIKey)
	}
	headers := map[string]string{}
	endpoint := base
	switch typ {
	case "totvs_protheus":
		if !strings.Contains(strings.ToLower(base), resource) {
			endpoint = base + "/" + strings.TrimPrefix(resource, "/")
		}
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	case "sap_b1":
		session, err := e.sapB1Session(ctx, cfg, base)
		if err != nil {
			return nil, nil, err
		}
		if session != "" {
			headers["Cookie"] = "B1SESSION=" + session
		} else if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
		if !strings.Contains(base, "/b1s/") && !strings.HasSuffix(strings.ToLower(base), strings.ToLower(resource)) {
			endpoint = base + "/b1s/v1/" + resource
		} else if !strings.Contains(strings.ToLower(base), strings.ToLower(resource)) {
			endpoint = base + "/" + resource
		}
	case "senior":
		if !strings.Contains(strings.ToLower(base), strings.ToLower(resource)) {
			endpoint = base + "/" + strings.TrimPrefix(resource, "/")
		}
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	default:
		if !strings.HasSuffix(strings.ToLower(base), strings.ToLower(resource)) {
			endpoint = base + "/" + resource
		}
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	}
	if q := strings.TrimSpace(cfg.Query); q != "" && !strings.Contains(endpoint, "?") {
		endpoint += "?" + q
	}
	raw, status, err := httpJSON(ctx, http.MethodGet, endpoint, headers, nil, cfg.User, cfg.Password)
	if err != nil {
		return nil, nil, err
	}
	if err := mustOK(status, raw); err != nil {
		return nil, nil, err
	}
	page, err := pickJSONArray(raw, "value", "items", "data", "results", "customers", "orders", "employees")
	if err != nil {
		return nil, nil, err
	}
	return mapsToRows(mapsLimited(page, cfg.RowLimit()))
}

func (e *Engine) sapB1Session(ctx context.Context, cfg SQLConfig, base string) (string, error) {
	if cfg.User == "" || cfg.Password == "" {
		return "", nil
	}
	loginURL := strings.TrimRight(base, "/")
	if !strings.Contains(strings.ToLower(loginURL), "login") {
		if strings.Contains(loginURL, "/b1s/") {
			loginURL = strings.TrimRight(loginURL, "/") + "/Login"
		} else {
			loginURL = loginURL + "/b1s/v1/Login"
		}
	}
	company := strings.TrimSpace(cfg.Database)
	if company == "" {
		company = strings.TrimSpace(cfg.Account)
	}
	body, _ := json.Marshal(map[string]any{
		"CompanyDB": company,
		"UserName":  cfg.User,
		"Password":  cfg.Password,
	})
	raw, status, err := httpJSON(ctx, http.MethodPost, loginURL, map[string]string{"Content-Type": "application/json"}, body, "", "")
	if err != nil {
		return "", err
	}
	if status >= 300 {
		return "", fmt.Errorf("SAP B1 login HTTP %d: %s", status, strings.TrimSpace(string(raw)))
	}
	var out struct {
		SessionID string `json:"SessionId"`
	}
	_ = json.Unmarshal(raw, &out)
	if out.SessionID == "" {
		var alt map[string]any
		_ = json.Unmarshal(raw, &alt)
		if v, ok := alt["SessionId"].(string); ok {
			out.SessionID = v
		}
	}
	return out.SessionID, nil
}
