package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type ibgeKind int

const (
	ibgeLocalidades ibgeKind = iota
	ibgeSIDRA
	ibgePIBPerCapita
)

type ibgeQuery struct {
	kind    ibgeKind
	path    string
	table   string
	vars    string
	periods string
	locs    string
	classif string
	growth  bool
	wide    bool
}

var ibgeQueries = map[string]ibgeQuery{
	"municipios": {kind: ibgeLocalidades, path: "https://servicodados.ibge.gov.br/api/v1/localidades/municipios"},
	"estados":    {kind: ibgeLocalidades, path: "https://servicodados.ibge.gov.br/api/v1/localidades/estados"},

	// PNAD Contínua trimestral (SIDRA)
	"horas": {
		kind: ibgeSIDRA, table: "6371", vars: "8190", periods: "all", locs: "N3[all]", classif: "2[all]",
	},
	"rendimento": {
		kind: ibgeSIDRA, table: "6472", vars: "5933", periods: "all", locs: "N3[all]",
	},
	"ocupacao": {
		kind: ibgeSIDRA, table: "6466", vars: "4097", periods: "all", locs: "N3[all]",
	},
	"desocupacao": {
		kind: ibgeSIDRA, table: "6468", vars: "4099", periods: "all", locs: "N3[all]",
	},
	"informalidade": {
		kind: ibgeSIDRA, table: "8529", vars: "12466", periods: "all", locs: "N3[all]",
	},
	"forca_trabalho": {
		kind: ibgeSIDRA, table: "6461", vars: "4096", periods: "all", locs: "N3[all]",
	},
	"empregados": {
		kind: ibgeSIDRA, table: "4096", vars: "4090", periods: "all", locs: "N3[all]", classif: "12029[all]",
	},
	"escolaridade": {
		kind: ibgeSIDRA, table: "4095", vars: "4090", periods: "all", locs: "N3[all]", classif: "1568[all]", wide: true,
	},
	"sexo": {
		kind: ibgeSIDRA, table: "4093", vars: "4096|4097|4099|12466", periods: "all", locs: "N3[all]", classif: "2[all]", wide: true,
	},
	"setor": {
		kind: ibgeSIDRA, table: "5434", vars: "4090", periods: "all", locs: "N3[all]", classif: "888[all]", wide: true,
	},

	// Contas nacionais / regionais e população
	"pib_brasil": {
		kind: ibgeSIDRA, table: "6784", vars: "9808|9812|93|9810", periods: "all", locs: "N1[all]",
	},
	"pib_estados": {
		kind: ibgeSIDRA, table: "5938", vars: "37", periods: "all", locs: "N3[all]",
	},
	"pib_municipios": {
		kind: ibgeSIDRA, table: "5938", vars: "37", periods: "-8", locs: "N6[all]", wide: true,
	},
	"pib_per_capita": {
		kind: ibgePIBPerCapita, table: "5938", vars: "37", periods: "all", locs: "N3[all]",
	},
	"pib_per_capita_municipios": {
		kind: ibgePIBPerCapita, table: "5938", vars: "37", periods: "-5", locs: "N6[all]", wide: true,
	},
	"valor_adicionado": {
		kind: ibgeSIDRA, table: "5938", vars: "498|513|517|6575|525", periods: "all", locs: "N3[all]",
	},
	"populacao": {
		kind: ibgeSIDRA, table: "6579", vars: "9324", periods: "all", locs: "N3[all]",
	},
	"populacao_municipios": {
		kind: ibgeSIDRA, table: "6579", vars: "9324", periods: "-12", locs: "N6[all]", wide: true,
	},
	"crescimento_populacional": {
		kind: ibgeSIDRA, table: "6579", vars: "9324", periods: "all", locs: "N3[all]", growth: true,
	},
}

func ibgeResourceNames() []string {
	keys := make([]string, 0, len(ibgeQueries))
	for k := range ibgeQueries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func ibgeLookup(resource string) (ibgeQuery, error) {
	if resource == "" {
		resource = "municipios"
	}
	q, ok := ibgeQueries[resource]
	if !ok {
		return ibgeQuery{}, fmt.Errorf("recurso IBGE %q desconhecido; use um de: %s", resource, strings.Join(ibgeResourceNames(), ", "))
	}
	return q, nil
}

func ibgeOfficialURL(resource string) string {
	q, err := ibgeLookup(resource)
	if err != nil {
		return ibgeQueries["municipios"].path
	}
	return ibgeURL(q)
}

func ibgeURL(q ibgeQuery) string {
	if q.kind == ibgeLocalidades {
		return q.path
	}
	periods := q.periods
	if periods == "" {
		periods = "all"
	}
	vars := q.vars
	if vars == "" {
		vars = "allxp"
	}
	u := fmt.Sprintf(
		"https://servicodados.ibge.gov.br/api/v3/agregados/%s/periodos/%s/variaveis/%s?localidades=%s",
		q.table, periods, vars, url.QueryEscape(q.locs),
	)
	if q.classif != "" {
		u += "&classificacao=" + url.QueryEscape(q.classif)
	}
	return u
}

func ibgePopURL(q ibgeQuery) string {
	return ibgeURL(ibgeQuery{kind: ibgeSIDRA, table: "6579", vars: "9324", periods: q.periods, locs: q.locs})
}

func ibgeLimit(cfg SQLConfig, q ibgeQuery) int {
	if cfg.Limit > 0 {
		return cfg.RowLimit()
	}
	if q.wide {
		return 200000
	}
	return cfg.RowLimit()
}

func (e *Engine) fetchIBGE(ctx context.Context, cfg SQLConfig, resource string) ([]string, [][]string, error) {
	q, err := ibgeLookup(resource)
	if err != nil {
		return nil, nil, err
	}
	if q.kind == ibgePIBPerCapita && strings.TrimSpace(cfg.URL) == "" {
		return e.fetchIBGEPIBPerCapita(ctx, cfg, q)
	}
	raw, err := publicGET(ctx, cfg, ibgeURL(q))
	if err != nil {
		return nil, nil, err
	}
	if q.kind == ibgeLocalidades {
		page, err := pickJSONArray(raw, "data")
		if err != nil {
			return nil, nil, err
		}
		return mapsToRows(mapsLimited(flattenNested(page), ibgeLimit(cfg, q)))
	}
	maps, err := flattenSIDRAMaps(raw, ibgeLimit(cfg, q))
	if err != nil {
		return nil, nil, err
	}
	if q.growth {
		maps = attachYoY(maps)
	}
	return mapsToRows(maps)
}

func (e *Engine) fetchIBGEPIBPerCapita(ctx context.Context, cfg SQLConfig, q ibgeQuery) ([]string, [][]string, error) {
	pibRaw, err := publicGET(ctx, cfg, ibgeURL(q))
	if err != nil {
		return nil, nil, err
	}
	popRaw, err := publicGET(ctx, cfg, ibgePopURL(q))
	if err != nil {
		return nil, nil, err
	}
	lim := ibgeLimit(cfg, q)
	pib, err := flattenSIDRAMaps(pibRaw, lim)
	if err != nil {
		return nil, nil, err
	}
	pop, err := flattenSIDRAMaps(popRaw, lim)
	if err != nil {
		return nil, nil, err
	}
	return mapsToRows(mapsLimited(joinPIBPerCapita(pib, pop), lim))
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
	maps, err := flattenSIDRAMaps(raw, limit)
	if err != nil {
		return nil, nil, err
	}
	return mapsToRows(maps)
}

func flattenSIDRAMaps(raw []byte, limit int) ([]map[string]any, error) {
	var root []map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		page, err2 := pickJSONArray(raw, "data", "resultados")
		if err2 != nil {
			return nil, err
		}
		return mapsLimited(page, limit), nil
	}
	var maps []map[string]any
	for _, block := range root {
		resultados, _ := block["resultados"].([]any)
		for _, r := range resultados {
			rm, _ := r.(map[string]any)
			classNome, catID, catNome := sidraClassificacao(rm)
			series, _ := rm["series"].([]any)
			for _, s := range series {
				sm, _ := s.(map[string]any)
				loc, _ := sm["localidade"].(map[string]any)
				serie, _ := sm["serie"].(map[string]any)
				nivel, _ := loc["nivel"].(map[string]any)
				row := map[string]any{
					"localidade_id":   loc["id"],
					"localidade_nome": loc["nome"],
					"variavel_id":     block["id"],
					"variavel":        block["variavel"],
					"unidade":         block["unidade"],
					"agregado":        block["id"],
				}
				if nivel != nil {
					row["nivel"] = nivel["id"]
					row["nivel_nome"] = nivel["nome"]
				}
				if classNome != "" {
					row["classificacao"] = classNome
					row["categoria_id"] = catID
					row["categoria"] = catNome
				}
				if len(serie) == 0 {
					maps = append(maps, row)
					continue
				}
				for period, val := range serie {
					if sidraMissing(val) {
						continue
					}
					item := map[string]any{}
					for k, v := range row {
						item[k] = v
					}
					item["periodo"] = period
					item["valor"] = val
					maps = append(maps, item)
					if limit > 0 && len(maps) >= limit {
						return maps, nil
					}
				}
			}
		}
	}
	if len(maps) == 0 {
		return nil, fmt.Errorf("SIDRA sem séries")
	}
	return maps, nil
}

func sidraClassificacao(rm map[string]any) (nome, catID, catNome string) {
	list, _ := rm["classificacoes"].([]any)
	var names, ids, labels []string
	for _, c := range list {
		cm, _ := c.(map[string]any)
		if n := strings.TrimSpace(fmt.Sprint(cm["nome"])); n != "" && n != "<nil>" {
			names = append(names, n)
		}
		cat, _ := cm["categoria"].(map[string]any)
		for id, label := range cat {
			ids = append(ids, id)
			labels = append(labels, fmt.Sprint(label))
		}
	}
	return strings.Join(names, " | "), strings.Join(ids, " | "), strings.Join(labels, " | ")
}

func sidraMissing(v any) bool {
	s := strings.TrimSpace(fmt.Sprint(v))
	return s == "" || s == "-" || s == "..." || s == "<nil>"
}

func attachYoY(maps []map[string]any) []map[string]any {
	type key struct{ loc, variavel, categoria, periodo string }
	byKey := map[key]float64{}
	for _, m := range maps {
		n, ok := parseSIDRANumber(m["valor"])
		if !ok {
			continue
		}
		byKey[key{fmt.Sprint(m["localidade_id"]), fmt.Sprint(m["variavel_id"]), fmt.Sprint(m["categoria_id"]), fmt.Sprint(m["periodo"])}] = n
	}
	out := make([]map[string]any, 0, len(maps))
	for _, m := range maps {
		item := map[string]any{}
		for k, v := range m {
			item[k] = v
		}
		cur, ok := parseSIDRANumber(m["valor"])
		prevP := prevSIDRAPeriod(fmt.Sprint(m["periodo"]))
		prev, okPrev := byKey[key{fmt.Sprint(m["localidade_id"]), fmt.Sprint(m["variavel_id"]), fmt.Sprint(m["categoria_id"]), prevP}]
		if ok && okPrev && prev != 0 {
			item["valor_anterior"] = strconv.FormatFloat(prev, 'f', -1, 64)
			item["crescimento_pct"] = strconv.FormatFloat(((cur-prev)/absFloat(prev))*100, 'f', 4, 64)
		}
		out = append(out, item)
	}
	return out
}

func joinPIBPerCapita(pib, pop []map[string]any) []map[string]any {
	popBy := map[string]float64{}
	for _, m := range pop {
		n, ok := parseSIDRANumber(m["valor"])
		if !ok {
			continue
		}
		k := fmt.Sprint(m["localidade_id"]) + "|" + sidraYear(fmt.Sprint(m["periodo"]))
		popBy[k] = n
	}
	var out []map[string]any
	for _, m := range pib {
		pibVal, ok := parseSIDRANumber(m["valor"])
		if !ok {
			continue
		}
		year := sidraYear(fmt.Sprint(m["periodo"]))
		k := fmt.Sprint(m["localidade_id"]) + "|" + year
		pessoas, ok := popBy[k]
		if !ok || pessoas == 0 {
			continue
		}
		// SIDRA 5938 variável 37 está em mil reais.
		perCapita := (pibVal * 1000) / pessoas
		out = append(out, map[string]any{
			"localidade_id":   m["localidade_id"],
			"localidade_nome": m["localidade_nome"],
			"periodo":         m["periodo"],
			"ano":             year,
			"pib_mil_reais":   strconv.FormatFloat(pibVal, 'f', -1, 64),
			"populacao":       strconv.FormatFloat(pessoas, 'f', -1, 64),
			"pib_per_capita":  strconv.FormatFloat(perCapita, 'f', 2, 64),
			"unidade":         "Reais",
			"variavel":        "PIB per capita a preços correntes",
		})
	}
	if len(out) == 0 {
		return pib
	}
	return out
}

func parseSIDRANumber(v any) (float64, bool) {
	if sidraMissing(v) {
		return 0, false
	}
	s := strings.ReplaceAll(strings.TrimSpace(fmt.Sprint(v)), ",", ".")
	n, err := strconv.ParseFloat(s, 64)
	return n, err == nil
}

func prevSIDRAPeriod(p string) string {
	p = strings.TrimSpace(p)
	n, err := strconv.Atoi(p)
	if err != nil {
		return ""
	}
	switch len(p) {
	case 6:
		return strconv.Itoa(n - 100)
	case 4:
		return strconv.Itoa(n - 1)
	default:
		return ""
	}
}

func sidraYear(p string) string {
	p = strings.TrimSpace(p)
	if len(p) >= 4 {
		return p[:4]
	}
	return p
}

func absFloat(n float64) float64 {
	if n < 0 {
		return -n
	}
	return n
}
