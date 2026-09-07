package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var protheusResources = map[string]string{
	"customers": "customerCustomer",
	"orders":    "salesOrder",
	"products":  "product",
	"invoices":  "salesInvoice",
	"vendors":   "vendor",
}

var sapB1Resources = map[string]string{
	"BusinessPartners": "BusinessPartners",
	"Orders":           "Orders",
	"Items":            "Items",
	"Invoices":         "Invoices",
	"PurchaseOrders":   "PurchaseOrders",
	"JournalEntries":   "JournalEntries",
}

var seniorResources = map[string]string{
	"employees":    "hcm/v1/employees",
	"payroll":      "hcm/v1/payroll",
	"vacations":    "hcm/v1/vacations",
	"departments":  "hcm/v1/departments",
	"positions":    "hcm/v1/positions",
}

func (e *Engine) fetchERP(ctx context.Context, typ string, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	resource = strings.TrimSpace(resource)
	if resource == "" {
		resource = erpDefaultResource(typ)
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if base == "" {
		return nil, nil, fmt.Errorf("URL da API obrigatória para %s", typ)
	}
	limit := cfg.RowLimit()
	if limit <= 0 {
		limit = 5000
	}
	headers := erpAuthHeaders(typ, cfg)
	if typ == "sap_b1" {
		session, err := e.sapB1Session(ctx, cfg, base)
		if err != nil {
			return nil, nil, err
		}
		if session != "" {
			headers["Cookie"] = "B1SESSION=" + session
		}
	}
	var collected []map[string]any
	pageSize := 200
	if pageSize > limit {
		pageSize = limit
	}
	for skip := 0; skip < limit; skip += pageSize {
		endpoint := erpEndpoint(typ, base, resource, skip, pageSize, cfg.Query)
		raw, status, err := httpJSON(ctx, http.MethodGet, endpoint, headers, nil, cfg.User, cfg.Password)
		if err != nil {
			return nil, nil, err
		}
		if err := mustOK(status, raw); err != nil {
			return nil, nil, fmt.Errorf("%s %s: %w", typ, resource, err)
		}
		page, err := pickJSONArray(raw, "value", "items", "data", "results", "customers", "orders", "employees", "content")
		if err != nil {
			if skip == 0 {
				return nil, nil, err
			}
			break
		}
		if len(page) == 0 {
			break
		}
		collected = append(collected, page...)
		if len(collected) >= limit || len(page) < pageSize {
			break
		}
	}
	if len(collected) > limit {
		collected = collected[:limit]
	}
	return mapsToRows(mapsLimited(collected, limit))
}

func erpDefaultResource(typ string) string {
	switch typ {
	case "totvs_protheus":
		return "customers"
	case "sap_b1":
		return "BusinessPartners"
	default:
		return "employees"
	}
}

func erpAuthHeaders(typ string, cfg SQLConfig) map[string]string {
	headers := map[string]string{"Accept": "application/json"}
	token := strings.TrimSpace(cfg.AuthToken())
	if token == "" {
		token = strings.TrimSpace(cfg.Token)
	}
	if token == "" {
		token = strings.TrimSpace(cfg.APIKey)
	}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	if typ == "totvs_protheus" || typ == "senior" {
		headers["tenantid"] = strings.TrimSpace(cfg.Database)
	}
	return headers
}

func erpEndpoint(typ, base, resource string, skip, pageSize int, extraQuery string) string {
	path := erpResourcePath(typ, resource)
	endpoint := base
	low := strings.ToLower(base)
	if !strings.Contains(low, strings.ToLower(path)) && !strings.HasSuffix(low, strings.ToLower(resource)) {
		switch typ {
		case "sap_b1":
			if !strings.Contains(low, "/b1s/") {
				endpoint = base + "/b1s/v1/" + path
			} else {
				endpoint = base + "/" + path
			}
		default:
			endpoint = base + "/" + strings.TrimPrefix(path, "/")
		}
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	q := u.Query()
	switch typ {
	case "sap_b1":
		q.Set("$top", strconv.Itoa(pageSize))
		if skip > 0 {
			q.Set("$skip", strconv.Itoa(skip))
		}
	case "totvs_protheus":
		q.Set("page", strconv.Itoa(skip/pageSize+1))
		q.Set("pageSize", strconv.Itoa(pageSize))
	default:
		q.Set("offset", strconv.Itoa(skip))
		q.Set("limit", strconv.Itoa(pageSize))
	}
	if extra := strings.TrimSpace(extraQuery); extra != "" {
		if extraVals, err := url.ParseQuery(strings.TrimPrefix(extra, "?")); err == nil {
			for k, vs := range extraVals {
				for _, v := range vs {
					q.Add(k, v)
				}
			}
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func erpResourcePath(typ, resource string) string {
	switch typ {
	case "totvs_protheus":
		if mapped, ok := protheusResources[resource]; ok {
			return mapped
		}
	case "sap_b1":
		if mapped, ok := sapB1Resources[resource]; ok {
			return mapped
		}
	case "senior":
		if mapped, ok := seniorResources[resource]; ok {
			return mapped
		}
	}
	return resource
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
	if out.SessionID == "" {
		return "", fmt.Errorf("SAP B1 login sem SessionId")
	}
	return out.SessionID, nil
}
