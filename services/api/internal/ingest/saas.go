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
)

func saasResources(typ string) []string {
	switch typ {
	case "asaas":
		return []string{"customers", "payments"}
	case "conta_azul":
		return []string{"vendas", "pessoas"}
	case "bitrix24":
		return []string{"crm.deal.list", "crm.contact.list"}
	case "omie":
		return []string{"clientes", "pedidos"}
	case "google_ads":
		return []string{"campaigns"}
	case "meta_ads":
		return []string{"insights", "campaigns"}
	case "salesforce":
		return []string{"Account", "Contact", "Opportunity"}
	case "github":
		return []string{"repos"}
	case "stripe":
		return []string{"charges", "customers", "invoices"}
	case "google_analytics":
		return []string{"report", "accounts"}
	case "instagram":
		return []string{"media", "insights"}
	case "facebook":
		return []string{"posts", "insights"}
	case "google_business":
		return []string{"locations", "reviews", "accounts"}
	case "mercado_livre":
		return []string{"me", "orders"}
	case "ibge_censo":
		return []string{"municipios", "populacao", "estados"}
	case "contabilidade":
		return []string{"sgs"}
	case "inflacao":
		return []string{"ipca"}
	case "expectativas":
		return []string{"anuais", "selic"}
	case "cambio":
		return []string{"ultima", "serie", "ptax"}
	default:
		return nil
	}
}

func (e *Engine) pingSaaS(ctx context.Context, typ string, cfg SQLConfig) error {
	_, _, err := e.fetchSaaS(ctx, typ, cfg)
	if err != nil && (strings.Contains(err.Error(), "sem objectos") || strings.Contains(err.Error(), "nenhuma linha")) {
		return nil
	}
	return err
}

func (e *Engine) fetchSaaS(ctx context.Context, typ string, cfg SQLConfig) ([]string, [][]string, error) {
	resource := strings.TrimSpace(cfg.Table)
	switch typ {
	case "asaas":
		return e.fetchAsaas(ctx, cfg, resource)
	case "conta_azul":
		return e.fetchContaAzul(ctx, cfg, resource)
	case "bitrix24":
		return e.fetchBitrix(ctx, cfg, resource)
	case "omie":
		return e.fetchOmie(ctx, cfg, resource)
	case "google_ads":
		return e.fetchGoogleAds(ctx, cfg)
	case "meta_ads":
		return e.fetchMetaAds(ctx, cfg, resource)
	case "salesforce":
		return e.fetchSalesforce(ctx, cfg, resource)
	case "github":
		return e.fetchGitHub(ctx, cfg)
	case "stripe":
		return e.fetchStripe(ctx, cfg, resource)
	case "google_analytics":
		return e.fetchGA(ctx, cfg)
	case "instagram":
		return e.fetchInstagram(ctx, cfg, resource)
	case "facebook":
		return e.fetchFacebook(ctx, cfg, resource)
	case "google_business":
		return e.fetchGoogleBusiness(ctx, cfg, resource)
	case "mercado_livre":
		return e.fetchMercadoLivre(ctx, cfg, resource)
	case "ibge_censo":
		return e.fetchIBGE(ctx, cfg, resource)
	case "contabilidade":
		return e.fetchContabilidade(ctx, cfg)
	case "inflacao":
		return e.fetchInflacao(ctx, cfg)
	case "expectativas":
		return e.fetchExpectativas(ctx, cfg, resource)
	case "cambio":
		return e.fetchCambio(ctx, cfg, resource)
	default:
		if strings.TrimSpace(cfg.URL) != "" {
			return e.fetchJSON(ctx, cfg)
		}
		return nil, nil, fmt.Errorf("conector %s sem fetcher", typ)
	}
}

func httpJSON(ctx context.Context, method, rawURL string, headers map[string]string, body []byte, basicUser, basicPass string) ([]byte, int, error) {
	if err := assertHTTPURL(rawURL); err != nil {
		return nil, 0, err
	}
	var rdr io.Reader
	if len(body) > 0 {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rdr)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if basicUser != "" || basicPass != "" {
		req.SetBasicAuth(basicUser, basicPass)
	}
	resp, err := connectorHTTP.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("pedido HTTP falhou: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

func mustOK(status int, raw []byte) error {
	if status < 200 || status >= 300 {
		return fmt.Errorf("HTTP %d: %s", status, truncate(string(raw), 200))
	}
	return nil
}

func pickJSONArray(raw []byte, keys ...string) ([]map[string]any, error) {
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 {
		return nil, fmt.Errorf("resposta vazia")
	}
	if trim[0] == '[' {
		var arr []map[string]any
		if err := json.Unmarshal(trim, &arr); err != nil {
			headers, rows, err2 := parseJSON(trim)
			if err2 != nil {
				return nil, err
			}
			return rowsToMaps(headers, rows), nil
		}
		return arr, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(trim, &obj); err != nil {
		return nil, err
	}
	for _, k := range keys {
		if arr, ok := obj[k].([]any); ok {
			out := make([]map[string]any, 0, len(arr))
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					out = append(out, m)
				}
			}
			return out, nil
		}
	}
	return []map[string]any{obj}, nil
}

func rowsToMaps(headers []string, rows [][]string) []map[string]any {
	out := make([]map[string]any, len(rows))
	for i, r := range rows {
		m := map[string]any{}
		for j, h := range headers {
			if j < len(r) {
				m[h] = r[j]
			}
		}
		out[i] = m
	}
	return out
}

func mapsLimited(maps []map[string]any, limit int) []map[string]any {
	if limit > 0 && len(maps) > limit {
		return maps[:limit]
	}
	return maps
}

func (e *Engine) fetchAsaas(ctx context.Context, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	key := strings.TrimSpace(cfg.APIKey)
	if key == "" {
		key = cfg.AuthToken()
	}
	if key == "" {
		return nil, nil, fmt.Errorf("api_key Asaas obrigatória")
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if base == "" {
		if strings.EqualFold(cfg.Environment, "prod") || strings.EqualFold(cfg.Environment, "production") {
			base = "https://api.asaas.com/v3"
		} else {
			base = "https://sandbox.asaas.com/api/v3"
		}
	}
	if resource == "" {
		resource = "customers"
	}
	if resource != "customers" && resource != "payments" {
		resource = "customers"
	}
	limit := cfg.RowLimit()
	if limit > 100 {
		limit = 100
	}
	var all []map[string]any
	offset := 0
	headers := map[string]string{"access_token": key, "User-Agent": "TheDobra/1.0"}
	for {
		u := fmt.Sprintf("%s/%s?limit=%d&offset=%d", base, resource, limit, offset)
		raw, status, err := httpJSON(ctx, http.MethodGet, u, headers, nil, "", "")
		if err != nil {
			return nil, nil, err
		}
		if err := mustOK(status, raw); err != nil {
			return nil, nil, err
		}
		page, err := pickJSONArray(raw, "data")
		if err != nil {
			return nil, nil, err
		}
		all = append(all, page...)
		var meta struct {
			HasMore bool `json:"hasMore"`
		}
		_ = json.Unmarshal(raw, &meta)
		offset += len(page)
		if !meta.HasMore || len(page) == 0 || offset >= cfg.RowLimit() {
			break
		}
	}
	return mapsToRows(mapsLimited(all, cfg.RowLimit()))
}

func (e *Engine) fetchContaAzul(ctx context.Context, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	token := cfg.AuthToken()
	if token == "" && cfg.ClientID != "" && cfg.ClientSecret != "" {
		t, err := contaAzulToken(ctx, cfg)
		if err != nil {
			return nil, nil, err
		}
		token = t
	}
	if token == "" {
		return nil, nil, fmt.Errorf("access_token ou client_id/secret da Conta Azul obrigatórios")
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if base == "" {
		base = "https://api.contaazul.com"
	}
	if resource == "" {
		resource = "vendas"
	}
	path := "/v1/vendas"
	if resource == "pessoas" || resource == "clientes" {
		path = "/v1/pessoas"
	}
	u := base + path + "?pagina=1&tamanho_pagina=50"
	raw, status, err := httpJSON(ctx, http.MethodGet, u, map[string]string{"Authorization": "Bearer " + token}, nil, "", "")
	if err != nil {
		return nil, nil, err
	}
	if err := mustOK(status, raw); err != nil {
		return nil, nil, err
	}
	page, err := pickJSONArray(raw, "itens", "items", "data", "content")
	if err != nil {
		return nil, nil, err
	}
	return mapsToRows(mapsLimited(page, cfg.RowLimit()))
}

func contaAzulToken(ctx context.Context, cfg SQLConfig) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.contaazul.com/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := connectorHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("oauth Conta Azul HTTP %d: %s", resp.StatusCode, truncate(string(raw), 180))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("oauth Conta Azul sem access_token")
	}
	return tok.AccessToken, nil
}

func (e *Engine) fetchBitrix(ctx context.Context, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	if resource == "" {
		resource = "crm.deal.list"
	}
	base := strings.TrimSpace(cfg.WebhookURL)
	if base == "" && cfg.Domain != "" && cfg.AuthToken() != "" {
		dom := strings.TrimPrefix(strings.TrimPrefix(cfg.Domain, "https://"), "http://")
		base = fmt.Sprintf("https://%s/rest/%s/", strings.TrimRight(dom, "/"), cfg.AuthToken())
	}
	if base == "" {
		return nil, nil, fmt.Errorf("webhook_url ou domínio + access_token do Bitrix24 obrigatórios")
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	method := resource
	if !strings.Contains(method, ".") {
		method = "crm.deal.list"
	}
	var all []map[string]any
	start := 0
	for {
		u := fmt.Sprintf("%s%s.json?start=%d", base, method, start)
		raw, status, err := httpJSON(ctx, http.MethodGet, u, nil, nil, "", "")
		if err != nil {
			return nil, nil, err
		}
		if err := mustOK(status, raw); err != nil {
			return nil, nil, err
		}
		page, err := pickJSONArray(raw, "result")
		if err != nil {
			return nil, nil, err
		}
		all = append(all, page...)
		var meta struct {
			Next int `json:"next"`
		}
		_ = json.Unmarshal(raw, &meta)
		if meta.Next == 0 || len(page) == 0 || len(all) >= cfg.RowLimit() {
			break
		}
		start = meta.Next
	}
	return mapsToRows(mapsLimited(all, cfg.RowLimit()))
}

func (e *Engine) fetchOmie(ctx context.Context, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	if cfg.AppKey == "" || cfg.AppSecret == "" {
		return nil, nil, fmt.Errorf("app_key e app_secret Omie obrigatórios")
	}
	call := "ListarClientes"
	endpoint := strings.TrimSpace(cfg.URL)
	if endpoint == "" {
		endpoint = "https://app.omie.com.br/api/v1/geral/clientes/"
	}
	if resource == "pedidos" {
		call = "ListarPedidos"
		if strings.TrimSpace(cfg.URL) == "" {
			endpoint = "https://app.omie.com.br/api/v1/produtos/pedido/"
		}
	}
	body, _ := json.Marshal(map[string]any{
		"call":       call,
		"app_key":    cfg.AppKey,
		"app_secret": cfg.AppSecret,
		"param":      []map[string]any{{"pagina": 1, "registros_por_pagina": min(cfg.RowLimit(), 500)}},
	})
	raw, status, err := httpJSON(ctx, http.MethodPost, endpoint, nil, body, "", "")
	if err != nil {
		return nil, nil, err
	}
	if err := mustOK(status, raw); err != nil {
		return nil, nil, err
	}
	page, err := pickJSONArray(raw, "clientes_cadastro", "pedido_venda_produto", "clientes", "pedidos")
	if err != nil {
		return nil, nil, err
	}
	return mapsToRows(mapsLimited(page, cfg.RowLimit()))
}

func (e *Engine) fetchGoogleAds(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	cid := strings.ReplaceAll(cfg.CustomerID, "-", "")
	if cid == "" {
		return nil, nil, fmt.Errorf("customer_id Google Ads obrigatório")
	}
	if cfg.DeveloperToken == "" {
		return nil, nil, fmt.Errorf("developer_token Google Ads obrigatório")
	}
	token := cfg.AuthToken()
	if token == "" && cfg.RefreshToken != "" && cfg.ClientID != "" && cfg.ClientSecret != "" {
		t, err := googleRefresh(ctx, cfg)
		if err != nil {
			return nil, nil, err
		}
		token = t
	}
	if token == "" {
		return nil, nil, fmt.Errorf("access_token ou refresh_token Google Ads obrigatório")
	}
	query := strings.TrimSpace(cfg.Query)
	if query == "" {
		query = fmt.Sprintf("SELECT campaign.id, campaign.name, metrics.impressions, metrics.clicks, metrics.cost_micros FROM campaign LIMIT %d", min(cfg.RowLimit(), 1000))
	}
	payload, _ := json.Marshal(map[string]any{"query": query})
	u := fmt.Sprintf("https://googleads.googleapis.com/v18/customers/%s/googleAds:search", cid)
	raw, status, err := httpJSON(ctx, http.MethodPost, u, map[string]string{
		"Authorization":     "Bearer " + token,
		"developer-token":   cfg.DeveloperToken,
		"login-customer-id": cid,
	}, payload, "", "")
	if err != nil {
		return nil, nil, err
	}
	if err := mustOK(status, raw); err != nil {
		return nil, nil, err
	}
	page, err := pickJSONArray(raw, "results")
	if err != nil {
		return nil, nil, err
	}
	return mapsToRows(mapsLimited(page, cfg.RowLimit()))
}

func googleRefresh(ctx context.Context, cfg SQLConfig) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("refresh_token", cfg.RefreshToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := connectorHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("refresh Google HTTP %d: %s", resp.StatusCode, truncate(string(raw), 180))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("refresh Google sem access_token")
	}
	return tok.AccessToken, nil
}

func (e *Engine) fetchMetaAds(ctx context.Context, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	token := cfg.AuthToken()
	if token == "" {
		return nil, nil, fmt.Errorf("access_token Meta Ads obrigatório")
	}
	acct := strings.TrimSpace(cfg.AdAccountID)
	if acct == "" {
		return nil, nil, fmt.Errorf("ad_account_id obrigatório")
	}
	if !strings.HasPrefix(acct, "act_") {
		acct = "act_" + acct
	}
	if resource == "" {
		resource = "insights"
	}
	fields := "campaign_name,impressions,spend,clicks,date_start,date_stop"
	path := acct + "/insights"
	if resource == "campaigns" {
		fields = "id,name,status,objective"
		path = acct + "/campaigns"
	}
	u := fmt.Sprintf("https://graph.facebook.com/v21.0/%s?fields=%s&limit=%d&access_token=%s", path, url.QueryEscape(fields), min(cfg.RowLimit(), 500), url.QueryEscape(token))
	raw, status, err := httpJSON(ctx, http.MethodGet, u, nil, nil, "", "")
	if err != nil {
		return nil, nil, err
	}
	if err := mustOK(status, raw); err != nil {
		return nil, nil, err
	}
	page, err := pickJSONArray(raw, "data")
	if err != nil {
		return nil, nil, err
	}
	return mapsToRows(mapsLimited(page, cfg.RowLimit()))
}

func (e *Engine) fetchSalesforce(ctx context.Context, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	inst := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if inst == "" {
		return nil, nil, fmt.Errorf("instance URL Salesforce obrigatório")
	}
	token := cfg.AuthToken()
	if token == "" {
		return nil, nil, fmt.Errorf("token Salesforce obrigatório")
	}
	soql := strings.TrimSpace(cfg.Query)
	if soql == "" {
		obj := resource
		if obj == "" {
			obj = "Account"
		}
		switch strings.ToLower(obj) {
		case "opportunity":
			soql = fmt.Sprintf("SELECT Id, Name, Amount, StageName, CloseDate FROM Opportunity LIMIT %d", min(cfg.RowLimit(), 2000))
		case "contact":
			soql = fmt.Sprintf("SELECT Id, Name, Email, Phone FROM Contact LIMIT %d", min(cfg.RowLimit(), 2000))
		default:
			soql = fmt.Sprintf("SELECT Id, Name FROM %s LIMIT %d", obj, min(cfg.RowLimit(), 2000))
		}
	}
	u := inst + "/services/data/v59.0/query?q=" + url.QueryEscape(soql)
	raw, status, err := httpJSON(ctx, http.MethodGet, u, map[string]string{"Authorization": "Bearer " + token}, nil, "", "")
	if err != nil {
		return nil, nil, err
	}
	if err := mustOK(status, raw); err != nil {
		return nil, nil, err
	}
	page, err := pickJSONArray(raw, "records")
	if err != nil {
		return nil, nil, err
	}
	return mapsToRows(mapsLimited(page, cfg.RowLimit()))
}

func (e *Engine) fetchGitHub(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	token := cfg.AuthToken()
	if token == "" {
		return nil, nil, fmt.Errorf("token GitHub obrigatório")
	}
	u := strings.TrimSpace(cfg.URL)
	if u == "" {
		u = "https://api.github.com/user/repos?per_page=100"
	}
	raw, status, err := httpJSON(ctx, http.MethodGet, u, map[string]string{
		"Authorization": "Bearer " + token,
		"Accept":        "application/vnd.github+json",
		"User-Agent":    "TheDobra",
	}, nil, "", "")
	if err != nil {
		return nil, nil, err
	}
	if err := mustOK(status, raw); err != nil {
		return nil, nil, err
	}
	page, err := pickJSONArray(raw, "items")
	if err != nil {
		return nil, nil, err
	}
	return mapsToRows(mapsLimited(page, cfg.RowLimit()))
}

func (e *Engine) fetchStripe(ctx context.Context, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	key := cfg.APIKey
	if key == "" {
		key = cfg.AuthToken()
	}
	if key == "" {
		return nil, nil, fmt.Errorf("chave secreta Stripe obrigatória")
	}
	if resource == "" {
		resource = "charges"
	}
	u := fmt.Sprintf("https://api.stripe.com/v1/%s?limit=%d", resource, min(cfg.RowLimit(), 100))
	raw, status, err := httpJSON(ctx, http.MethodGet, u, nil, nil, key, "")
	if err != nil {
		return nil, nil, err
	}
	if err := mustOK(status, raw); err != nil {
		return nil, nil, err
	}
	page, err := pickJSONArray(raw, "data")
	if err != nil {
		return nil, nil, err
	}
	return mapsToRows(mapsLimited(page, cfg.RowLimit()))
}

func (e *Engine) fetchGA(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	token := cfg.AuthToken()
	if token == "" && cfg.RefreshToken != "" {
		t, err := googleRefresh(ctx, cfg)
		if err != nil {
			return nil, nil, err
		}
		token = t
	}
	if token == "" {
		return nil, nil, fmt.Errorf("token Google Analytics obrigatório")
	}
	prop := strings.TrimSpace(cfg.PropertyID)
	if prop == "" {
		prop = strings.TrimSpace(cfg.Project)
	}
	if prop == "" {
		raw, status, err := httpJSON(ctx, http.MethodGet, "https://analyticsadmin.googleapis.com/v1beta/accounts", map[string]string{"Authorization": "Bearer " + token}, nil, "", "")
		if err != nil {
			return nil, nil, err
		}
		if err := mustOK(status, raw); err != nil {
			return nil, nil, err
		}
		page, err := pickJSONArray(raw, "accounts")
		if err != nil {
			return nil, nil, err
		}
		return mapsToRows(mapsLimited(page, cfg.RowLimit()))
	}
	if !strings.HasPrefix(prop, "properties/") {
		prop = "properties/" + prop
	}
	body, _ := json.Marshal(map[string]any{
		"dateRanges": []map[string]string{{"startDate": "28daysAgo", "endDate": "today"}},
		"dimensions": []map[string]string{{"name": "date"}},
		"metrics":    []map[string]string{{"name": "sessions"}, {"name": "activeUsers"}},
		"limit":      strconv.Itoa(min(cfg.RowLimit(), 10000)),
	})
	u := fmt.Sprintf("https://analyticsdata.googleapis.com/v1beta/%s:runReport", prop)
	raw, status, err := httpJSON(ctx, http.MethodPost, u, map[string]string{"Authorization": "Bearer " + token}, body, "", "")
	if err != nil {
		return nil, nil, err
	}
	if err := mustOK(status, raw); err != nil {
		return nil, nil, err
	}
	var report struct {
		DimensionHeaders []struct {
			Name string `json:"name"`
		} `json:"dimensionHeaders"`
		MetricHeaders []struct {
			Name string `json:"name"`
		} `json:"metricHeaders"`
		Rows []struct {
			DimensionValues []struct {
				Value string `json:"value"`
			} `json:"dimensionValues"`
			MetricValues []struct {
				Value string `json:"value"`
			} `json:"metricValues"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, nil, err
	}
	headers := []string{}
	for _, h := range report.DimensionHeaders {
		headers = append(headers, h.Name)
	}
	for _, h := range report.MetricHeaders {
		headers = append(headers, h.Name)
	}
	out := make([][]string, 0, len(report.Rows))
	for _, r := range report.Rows {
		rec := make([]string, len(headers))
		i := 0
		for _, d := range r.DimensionValues {
			if i < len(rec) {
				rec[i] = d.Value
				i++
			}
		}
		for _, m := range r.MetricValues {
			if i < len(rec) {
				rec[i] = m.Value
				i++
			}
		}
		out = append(out, rec)
	}
	return headers, out, nil
}