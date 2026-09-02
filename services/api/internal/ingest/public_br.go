package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func publicGET(ctx context.Context, cfg SQLConfig, official string) ([]byte, error) {
	u := strings.TrimSpace(cfg.URL)
	if u == "" {
		u = official
	}
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 400 * time.Millisecond):
			}
		}
		raw, status, err := httpJSON(ctx, http.MethodGet, u, map[string]string{"User-Agent": "TheDobra/1.0"}, nil, "", "")
		if err != nil {
			last = err
			continue
		}
		if status >= 500 {
			last = fmt.Errorf("HTTP %d: %s", status, truncate(string(raw), 200))
			continue
		}
		if err := mustOK(status, raw); err != nil {
			return nil, err
		}
		return raw, nil
	}
	if last == nil {
		last = fmt.Errorf("pedido HTTP falhou")
	}
	return nil, last
}

func ibgeOfficialURL(resource string) string {
	switch resource {
	case "estados":
		return "https://servicodados.ibge.gov.br/api/v1/localidades/estados"
	case "populacao":
		// 6579 = população residente estimada. "all" evita o período 2022, que a SIDRA já não publica.
		return "https://servicodados.ibge.gov.br/api/v3/agregados/6579/periodos/all/variaveis/9324?localidades=N3[all]"
	default:
		return "https://servicodados.ibge.gov.br/api/v1/localidades/municipios"
	}
}

func focusOfficialURL(resource string, top int) string {
	if top <= 0 || top > 10000 {
		top = 10000
	}
	entity := "ExpectativasMercadoAnuais"
	if resource == "selic" {
		entity = "ExpectativasMercadoSelic"
	}
	return fmt.Sprintf(
		"https://olinda.bcb.gov.br/olinda/servico/Expectativas/versao/v1/odata/%s?$top=%d&$orderby=Data%%20desc&$format=json",
		entity, top,
	)
}

func (e *Engine) fetchIBGE(ctx context.Context, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	if resource == "" {
		resource = "municipios"
	}
	raw, err := publicGET(ctx, cfg, ibgeOfficialURL(resource))
	if err != nil {
		return nil, nil, err
	}
	if resource == "populacao" {
		return flattenSIDRA(raw, cfg.RowLimit())
	}
	page, err := pickJSONArray(raw, "data")
	if err != nil {
		return nil, nil, err
	}
	return mapsToRows(mapsLimited(flattenNested(page), cfg.RowLimit()))
}

func flattenNested(in []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, m := range in {
		row := map[string]any{}
		for k, v := range m {
			switch t := v.(type) {
			case map[string]any:
				if nome, ok := t["nome"]; ok {
					row[k] = nome
					if id, ok := t["id"]; ok {
						row[k+"_id"] = id
					}
					if sigla, ok := t["sigla"]; ok {
						row[k+"_sigla"] = sigla
					}
				} else {
					b, _ := json.Marshal(t)
					row[k] = string(b)
				}
			default:
				row[k] = v
			}
		}
		out = append(out, row)
	}
	return out
}

func flattenSIDRA(raw []byte, limit int) ([]string, [][]string, error) {
	var root []map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		page, err2 := pickJSONArray(raw, "data", "resultados")
		if err2 != nil {
			return nil, nil, err
		}
		return mapsToRows(mapsLimited(page, limit))
	}
	var maps []map[string]any
	for _, block := range root {
		resultados, _ := block["resultados"].([]any)
		for _, r := range resultados {
			rm, _ := r.(map[string]any)
			series, _ := rm["series"].([]any)
			for _, s := range series {
				sm, _ := s.(map[string]any)
				loc, _ := sm["localidade"].(map[string]any)
				serie, _ := sm["serie"].(map[string]any)
				row := map[string]any{
					"localidade_id":   loc["id"],
					"localidade_nome": loc["nome"],
					"agregado":        block["id"],
				}
				if len(serie) == 0 {
					maps = append(maps, row)
					continue
				}
				for period, val := range serie {
					item := map[string]any{}
					for k, v := range row {
						item[k] = v
					}
					item["periodo"] = period
					item["valor"] = val
					maps = append(maps, item)
					if limit > 0 && len(maps) >= limit {
						return mapsToRows(maps)
					}
				}
			}
		}
	}
	if len(maps) == 0 {
		return nil, nil, fmt.Errorf("SIDRA sem séries")
	}
	return mapsToRows(maps)
}

func sgsOfficialURL(seriesID string) string {
	return fmt.Sprintf("https://api.bcb.gov.br/dados/serie/bcdata.sgs.%s/dados?formato=json", seriesID)
}

func sgsRangeURL(seriesID string, start, end time.Time) string {
	return fmt.Sprintf(
		"https://api.bcb.gov.br/dados/serie/bcdata.sgs.%s/dados?formato=json&dataInicial=%s&dataFinal=%s",
		seriesID, start.Format("02/01/2006"), end.Format("02/01/2006"),
	)
}

func (e *Engine) fetchBCBSGS(ctx context.Context, cfg SQLConfig, seriesID string) ([]string, [][]string, error) {
	seriesID = strings.TrimSpace(seriesID)
	if seriesID == "" {
		return nil, nil, fmt.Errorf("série SGS obrigatória")
	}
	raw, err := publicGET(ctx, cfg, sgsOfficialURL(seriesID))
	if err != nil && strings.TrimSpace(cfg.URL) == "" {
		end := time.Now()
		raw, err = publicGET(ctx, cfg, sgsRangeURL(seriesID, end.AddDate(-20, 0, 0), end))
	}
	if err != nil {
		return nil, nil, err
	}
	page, err := pickJSONArray(raw, "value", "data")
	if err != nil {
		return nil, nil, err
	}
	if len(page) == 0 {
		return nil, nil, fmt.Errorf("série SGS %s sem observações", seriesID)
	}
	for i := range page {
		page[i]["serie"] = seriesID
	}
	return mapsToRows(mapsLimited(page, cfg.RowLimit()))
}

func (e *Engine) fetchInflacao(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	sid := strings.TrimSpace(cfg.Series)
	if sid == "" {
		sid = "433"
	}
	sid = strings.Split(sid, ",")[0]
	return e.fetchBCBSGS(ctx, cfg, strings.TrimSpace(sid))
}

func (e *Engine) fetchContabilidade(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	if strings.TrimSpace(cfg.URL) != "" {
		raw, err := publicGET(ctx, cfg, cfg.URL)
		if err != nil {
			return nil, nil, err
		}
		if strings.Contains(strings.ToLower(cfg.URL), ".ofx") || strings.Contains(string(raw[:min(len(raw), 40)]), "OFX") {
			return parseOFX(raw)
		}
		page, err := pickJSONArray(raw, "value", "data")
		if err != nil {
			return parseCSV(raw)
		}
		return mapsToRows(mapsLimited(page, cfg.RowLimit()))
	}
	rawSeries := strings.TrimSpace(cfg.Series)
	if rawSeries == "" {
		rawSeries = "24363,2203"
	}
	var all []map[string]any
	for _, sid := range strings.Split(rawSeries, ",") {
		sid = strings.TrimSpace(sid)
		if sid == "" {
			continue
		}
		h, rows, err := e.fetchBCBSGS(ctx, SQLConfig{Limit: cfg.Limit}, sid)
		if err != nil {
			return nil, nil, err
		}
		_ = h
		for _, r := range rows {
			m := map[string]any{}
			for i, col := range h {
				if i < len(r) {
					m[col] = r[i]
				}
			}
			all = append(all, m)
		}
	}
	if len(all) == 0 {
		return nil, nil, fmt.Errorf("sem séries BCB — carregue um CSV/OFX ou indique IDs SGS")
	}
	return mapsToRows(mapsLimited(all, cfg.RowLimit()))
}

func (e *Engine) fetchExpectativas(ctx context.Context, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	raw, err := publicGET(ctx, cfg, focusOfficialURL(resource, cfg.RowLimit()))
	if err != nil {
		return nil, nil, err
	}
	page, err := pickJSONArray(raw, "value")
	if err != nil {
		return nil, nil, err
	}
	if len(page) == 0 {
		return nil, nil, fmt.Errorf("Focus Olinda sem linhas")
	}
	return mapsToRows(mapsLimited(page, cfg.RowLimit()))
}

func awesomeAPIURL() string {
	return "https://economia.awesomeapi.com.br/json/last/USD-BRL,EUR-BRL"
}

func ptaxOfficialURL(start, end time.Time, top int) string {
	if top <= 0 || top > 10000 {
		top = 10000
	}
	return fmt.Sprintf(
		"https://olinda.bcb.gov.br/olinda/servico/PTAX/versao/v1/odata/CotacaoDolarPeriodo(dataInicial=@i,dataFinalCotacao=@f)?@i='%s'&@f='%s'&$top=%d&$orderby=dataHoraCotacao%%20desc&$format=json",
		start.Format("01-02-2006"), end.Format("01-02-2006"), top,
	)
}

func tagFonte(headers []string, rows [][]string, fonte string, fallback bool) ([]string, [][]string) {
	outH := append(append([]string{}, headers...), "_fonte")
	if fallback {
		outH = append(outH, "_fallback")
	}
	out := make([][]string, len(rows))
	for i, r := range rows {
		rec := make([]string, len(outH))
		copy(rec, r)
		rec[len(headers)] = fonte
		if fallback {
			rec[len(headers)+1] = "true"
		}
		out[i] = rec
	}
	return outH, out
}

func (e *Engine) fetchAwesomeAPI(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	raw, err := publicGET(ctx, cfg, awesomeAPIURL())
	if err != nil {
		return nil, nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		page, err2 := pickJSONArray(raw)
		if err2 != nil {
			return nil, nil, err
		}
		return mapsToRows(page)
	}
	var maps []map[string]any
	for code, v := range obj {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		m["par"] = code
		maps = append(maps, m)
	}
	if len(maps) == 0 {
		return nil, nil, fmt.Errorf("AwesomeAPI sem cotações")
	}
	return mapsToRows(maps)
}

func (e *Engine) fetchCambio(ctx context.Context, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	if resource == "" {
		resource = "ultima"
	}
	switch resource {
	case "serie":
		h, rows, err := e.fetchBCBSGS(ctx, cfg, orDefault(cfg.Series, "1"))
		if err != nil {
			return nil, nil, err
		}
		h, rows = tagFonte(h, rows, "bcb_sgs", false)
		return h, rows, nil
	case "ptax":
		end := time.Now()
		start := end.AddDate(0, 0, -90)
		raw, err := publicGET(ctx, cfg, ptaxOfficialURL(start, end, cfg.RowLimit()))
		if err == nil {
			page, perr := pickJSONArray(raw, "value")
			if perr == nil && len(page) > 0 {
				h, rows, merr := mapsToRows(mapsLimited(page, cfg.RowLimit()))
				if merr != nil {
					return nil, nil, merr
				}
				h, rows = tagFonte(h, rows, "ptax", false)
				return h, rows, nil
			}
			err = fmt.Errorf("PTAX sem cotações no período")
		}
		h, rows, err2 := e.fetchAwesomeAPI(ctx, SQLConfig{Limit: cfg.Limit})
		if err2 == nil {
			h, rows = tagFonte(h, rows, "awesomeapi", true)
			return h, rows, nil
		}
		h, rows, err3 := e.fetchBCBSGS(ctx, SQLConfig{Limit: cfg.Limit}, "1")
		if err3 == nil {
			h, rows = tagFonte(h, rows, "bcb_sgs", true)
			return h, rows, nil
		}
		return nil, nil, fmt.Errorf("PTAX indisponível (%v); fallbacks também falharam: %w", err, err3)
	default:
		h, rows, err := e.fetchAwesomeAPI(ctx, cfg)
		if err == nil {
			h, rows = tagFonte(h, rows, "awesomeapi", false)
			return h, rows, nil
		}
		h, rows, err2 := e.fetchBCBSGS(ctx, SQLConfig{Limit: cfg.Limit}, "1")
		if err2 != nil {
			return nil, nil, fmt.Errorf("AwesomeAPI indisponível (%v); fallback SGS 1 falhou: %w", err, err2)
		}
		h, rows = tagFonte(h, rows, "bcb_sgs", true)
		return h, rows, nil
	}
}

func orDefault(v, d string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return d
	}
	return strings.Split(v, ",")[0]
}

func parseOFX(raw []byte) ([]string, [][]string, error) {
	s := string(raw)
	headers := []string{"type", "posted", "amount", "fitid", "memo", "name"}
	var rows [][]string
	chunks := strings.Split(s, "<STMTTRN>")
	for i, c := range chunks {
		if i == 0 {
			continue
		}
		end := strings.Index(strings.ToUpper(c), "</STMTTRN>")
		if end >= 0 {
			c = c[:end]
		}
		row := []string{
			ofxTag(c, "TRNTYPE"),
			ofxTag(c, "DTPOSTED"),
			ofxTag(c, "TRNAMT"),
			ofxTag(c, "FITID"),
			ofxTag(c, "MEMO"),
			ofxTag(c, "NAME"),
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, nil, fmt.Errorf("OFX sem transacções STMTTRN")
	}
	return headers, rows, nil
}

func ofxTag(block, tag string) string {
	up := strings.ToUpper(block)
	key := "<" + tag + ">"
	i := strings.Index(up, key)
	if i < 0 {
		return ""
	}
	rest := block[i+len(key):]
	if j := strings.Index(rest, "<"); j >= 0 {
		rest = rest[:j]
	}
	return strings.TrimSpace(rest)
}
