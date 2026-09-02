package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (e *Engine) fetchInstagram(ctx context.Context, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	token := cfg.AuthToken()
	ig := strings.TrimSpace(cfg.InstagramID)
	if ig == "" {
		ig = strings.TrimSpace(cfg.AdAccountID)
	}
	if token == "" || ig == "" {
		return nil, nil, fmt.Errorf("access_token e instagram_business_account_id obrigatórios")
	}
	if resource == "" {
		resource = "media"
	}
	if strings.TrimSpace(cfg.URL) != "" && !strings.Contains(cfg.URL, "graph.facebook.com") {
		raw, status, err := httpJSON(ctx, http.MethodGet, cfg.URL, map[string]string{"Authorization": "Bearer " + token}, nil, "", "")
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
	var u string
	if resource == "insights" {
		u = fmt.Sprintf("https://graph.facebook.com/v21.0/%s/insights?metric=impressions,reach,profile_views&period=day&access_token=%s", url.PathEscape(ig), url.QueryEscape(token))
	} else {
		u = fmt.Sprintf("https://graph.facebook.com/v21.0/%s/media?fields=id,caption,media_type,permalink,timestamp,like_count,comments_count&limit=%d&access_token=%s",
			url.PathEscape(ig), min(cfg.RowLimit(), 100), url.QueryEscape(token))
	}
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

func (e *Engine) fetchFacebook(ctx context.Context, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	token := cfg.AuthToken()
	pageID := strings.TrimSpace(cfg.PageID)
	if pageID == "" {
		pageID = strings.TrimSpace(cfg.CustomerID)
	}
	if token == "" || pageID == "" {
		return nil, nil, fmt.Errorf("access_token e page_id obrigatórios")
	}
	if resource == "" {
		resource = "posts"
	}
	if strings.TrimSpace(cfg.URL) != "" && !strings.Contains(cfg.URL, "graph.facebook.com") {
		raw, status, err := httpJSON(ctx, http.MethodGet, cfg.URL, nil, nil, "", "")
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
	var u string
	if resource == "insights" {
		u = fmt.Sprintf("https://graph.facebook.com/v21.0/%s/insights?metric=page_impressions,page_post_engagements&period=day&access_token=%s",
			url.PathEscape(pageID), url.QueryEscape(token))
	} else {
		u = fmt.Sprintf("https://graph.facebook.com/v21.0/%s/posts?fields=id,message,created_time,shares&limit=%d&access_token=%s",
			url.PathEscape(pageID), min(cfg.RowLimit(), 100), url.QueryEscape(token))
	}
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

func (e *Engine) fetchGoogleBusiness(ctx context.Context, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	token := cfg.AuthToken()
	if token == "" {
		return nil, nil, fmt.Errorf("access_token do Google Meu Negócio obrigatório — a API não é pública")
	}
	if resource == "" {
		resource = "locations"
	}
	acct := strings.TrimSpace(cfg.Account)
	if acct != "" && !strings.HasPrefix(acct, "accounts/") {
		acct = "accounts/" + acct
	}
	loc := strings.TrimSpace(cfg.LocationID)
	headers := map[string]string{"Authorization": "Bearer " + token}
	if strings.TrimSpace(cfg.URL) != "" {
		raw, status, err := httpJSON(ctx, http.MethodGet, cfg.URL, headers, nil, "", "")
		if err != nil {
			return nil, nil, err
		}
		if err := mustOK(status, raw); err != nil {
			return nil, nil, fmt.Errorf("Google Meu Negócio recusou (%s). Verifique o token, account_id e location_id", err.Error())
		}
		page, err := pickJSONArray(raw, "locations", "reviews", "accounts")
		if err != nil {
			return nil, nil, err
		}
		return mapsToRows(mapsLimited(page, cfg.RowLimit()))
	}
	var u string
	switch resource {
	case "accounts":
		u = "https://mybusinessaccountmanagement.googleapis.com/v1/accounts"
	case "reviews":
		if acct == "" || loc == "" {
			return nil, nil, fmt.Errorf("account_id e location_id obrigatórios para avaliações")
		}
		if !strings.Contains(loc, "locations/") {
			loc = "locations/" + loc
		}
		u = fmt.Sprintf("https://mybusiness.googleapis.com/v4/%s/%s/reviews", acct, loc)
	default:
		if acct == "" {
			u = "https://mybusinessaccountmanagement.googleapis.com/v1/accounts"
			resource = "accounts"
		} else {
			u = "https://mybusinessbusinessinformation.googleapis.com/v1/" + acct + "/locations?readMask=name,title,storefrontAddress"
		}
	}
	raw, status, err := httpJSON(ctx, http.MethodGet, u, headers, nil, "", "")
	if err != nil {
		return nil, nil, err
	}
	if err := mustOK(status, raw); err != nil {
		return nil, nil, fmt.Errorf("Google Meu Negócio recusou (%s). Cole um access token válido com âmbito business.manage", err.Error())
	}
	page, err := pickJSONArray(raw, "locations", "reviews", "accounts")
	if err != nil {
		return nil, nil, err
	}
	return mapsToRows(mapsLimited(page, cfg.RowLimit()))
}

func (e *Engine) fetchMercadoLivre(ctx context.Context, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	token := cfg.AuthToken()
	if token == "" {
		return nil, nil, fmt.Errorf("access_token do Mercado Livre obrigatório")
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if base == "" {
		base = "https://api.mercadolibre.com"
	}
	headers := map[string]string{"Authorization": "Bearer " + token}
	if resource == "" {
		resource = "orders"
	}
	seller := strings.TrimSpace(cfg.SellerID)
	if seller == "" {
		seller = strings.TrimSpace(cfg.CustomerID)
	}
	if seller == "" || resource == "me" {
		meURL := base + "/users/me"
		raw, status, err := httpJSON(ctx, http.MethodGet, meURL, headers, nil, "", "")
		if err != nil {
			return nil, nil, err
		}
		if err := mustOK(status, raw); err != nil {
			return nil, nil, err
		}
		if resource == "me" {
			page, err := pickJSONArray(raw)
			if err != nil {
				return nil, nil, err
			}
			return mapsToRows(page)
		}
		var me struct {
			ID json.Number `json:"id"`
		}
		if err := json.Unmarshal(raw, &me); err == nil {
			seller = me.ID.String()
		}
	}
	if seller == "" {
		return nil, nil, fmt.Errorf("seller_id em falta e /users/me não devolveu id")
	}
	u := fmt.Sprintf("%s/orders/search?seller=%s", base, url.QueryEscape(seller))
	raw, status, err := httpJSON(ctx, http.MethodGet, u, headers, nil, "", "")
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
