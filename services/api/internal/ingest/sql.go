package ingest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/thedobra/thedobra/services/api/internal/connector"
	"github.com/thedobra/thedobra/services/api/internal/cryptoenc"
	"github.com/thedobra/thedobra/services/api/internal/quality"
	"github.com/thedobra/thedobra/services/api/internal/schemax"
	"github.com/thedobra/thedobra/services/api/internal/semantic"
)

type HeaderMap map[string]string

func (h *HeaderMap) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err == nil {
		*h = m
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*h = parseHeaderLines(s)
	return nil
}

type SQLConfig struct {
	Host           string           `json:"host"`
	Port           int              `json:"port"`
	Database       string           `json:"database"`
	User           string           `json:"user"`
	Password       string           `json:"password"`
	SSLMode        string           `json:"ssl_mode"`
	SSL            bool             `json:"ssl"`
	Table          string           `json:"table"`
	Query          string           `json:"query"`
	URL            string           `json:"url"`
	Token          string           `json:"token"`
	APIKey         string           `json:"api_key"`
	Headers        HeaderMap        `json:"headers"`
	Account        string           `json:"account"`
	Warehouse      string           `json:"warehouse"`
	Project        string           `json:"project"`
	Dataset        string           `json:"dataset"`
	Schema         string           `json:"schema"`
	Region         string           `json:"region"`
	Role           string           `json:"role"`
	Catalog        string           `json:"catalog"`
	HTTPPath       string           `json:"http_path"`
	AuthType       string           `json:"auth_type"`
	Topic          string           `json:"topic"`
	Broker         string           `json:"broker"`
	FileName       string           `json:"file_name"`
	StorageMode    string           `json:"storage_mode"`
	Environment    string           `json:"environment"`
	WebhookURL     string           `json:"webhook_url"`
	ClientID       string           `json:"client_id"`
	ClientSecret   string           `json:"client_secret"`
	AccessToken    string           `json:"access_token"`
	Domain         string           `json:"domain"`
	AppKey         string           `json:"app_key"`
	AppSecret      string           `json:"app_secret"`
	DeveloperToken string           `json:"developer_token"`
	RefreshToken   string           `json:"refresh_token"`
	CustomerID     string           `json:"customer_id"`
	AdAccountID    string           `json:"ad_account_id"`
	PropertyID     string           `json:"property_id"`
	PageID         string           `json:"page_id"`
	InstagramID    string           `json:"instagram_business_account_id"`
	LocationID     string           `json:"location_id"`
	SellerID       string           `json:"seller_id"`
	Series         string           `json:"series"`
	Limit          int              `json:"limit"`
	ProjectURL     string           `json:"project_url"`
	ServiceRoleKey string           `json:"service_role_key"`
	AnonKey        string           `json:"anon_key"`
	Selection      *SourceSelection `json:"selection,omitempty"`
}

func (c SQLConfig) AuthToken() string {
	for _, v := range []string{c.AccessToken, c.Token, c.APIKey} {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (c SQLConfig) RowLimit() int {
	if c.Limit > 0 && c.Limit < 5_000_000 {
		return c.Limit
	}
	return 10000
}

func (c SQLConfig) EffectiveSSLMode() string {
	if c.SSLMode != "" {
		return c.SSLMode
	}
	if c.SSL {
		return "require"
	}
	return "disable"
}

type DiscoverResult struct {
	Tables      []string       `json:"tables"`
	Catalog     []InspectTable `json:"catalog,omitempty"`
	ForeignKeys []InspectFK    `json:"foreign_keys,omitempty"`
	Preview     bool           `json:"preview,omitempty"`
	Message     string         `json:"message,omitempty"`
}

type TestResult struct {
	OK          bool   `json:"ok"`
	Implemented bool   `json:"implemented"`
	Message     string `json:"message"`
}

type SourceDataset struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	RowCount    int64     `json:"row_count"`
	StorageMode string    `json:"storage_mode"`
}

type SourceView struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Type        string          `json:"type"`
	Status      string          `json:"status"`
	LastSyncAt  *time.Time      `json:"last_sync_at"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Implemented bool            `json:"implemented"`
	Preview     bool            `json:"preview"`
	Message     string          `json:"message,omitempty"`
	Config      map[string]any  `json:"config"`
	Datasets    []SourceDataset `json:"datasets"`
}

func RedactConfig(cfg SQLConfig) map[string]any {
	out := map[string]any{
		"host":                 cfg.Host,
		"port":                 cfg.Port,
		"database":             cfg.Database,
		"user":                 cfg.User,
		"ssl":                  cfg.SSL,
		"ssl_mode":             cfg.EffectiveSSLMode(),
		"table":                cfg.Table,
		"query":                cfg.Query,
		"url":                  cfg.URL,
		"account":              cfg.Account,
		"warehouse":            cfg.Warehouse,
		"project":              cfg.Project,
		"dataset":              cfg.Dataset,
		"schema":               cfg.Schema,
		"region":               cfg.Region,
		"role":                 cfg.Role,
		"catalog":              cfg.Catalog,
		"http_path":            cfg.HTTPPath,
		"auth_type":            cfg.AuthType,
		"topic":                cfg.Topic,
		"broker":               cfg.Broker,
		"file_name":            cfg.FileName,
		"storage_mode":         cfg.StorageMode,
		"environment":          cfg.Environment,
		"webhook_url":          cfg.WebhookURL,
		"domain":               cfg.Domain,
		"customer_id":          cfg.CustomerID,
		"ad_account_id":        cfg.AdAccountID,
		"property_id":          cfg.PropertyID,
		"page_id":              cfg.PageID,
		"instagram_id":         cfg.InstagramID,
		"location_id":          cfg.LocationID,
		"seller_id":            cfg.SellerID,
		"series":               cfg.Series,
		"client_id":            cfg.ClientID,
		"project_url":          cfg.ProjectURL,
		"limit":                cfg.RowLimit(),
		"password_set":         cfg.Password != "",
		"token_set":            cfg.Token != "",
		"api_key_set":          cfg.APIKey != "",
		"service_role_key_set": cfg.ServiceRoleKey != "",
		"anon_key_set":         cfg.AnonKey != "",
		"access_token_set":     cfg.AccessToken != "",
		"client_secret_set":    cfg.ClientSecret != "",
		"app_key_set":          cfg.AppKey != "",
		"app_secret_set":       cfg.AppSecret != "",
		"refresh_token_set":    cfg.RefreshToken != "",
		"developer_token_set":  cfg.DeveloperToken != "",
	}
	if cfg.Selection != nil && !cfg.Selection.empty() {
		out["selection"] = cfg.Selection
	}
	if len(cfg.Headers) > 0 {
		keys := make([]string, 0, len(cfg.Headers))
		for k := range cfg.Headers {
			keys = append(keys, k)
		}
		out["header_keys"] = keys
	}
	return out
}

func (e *Engine) SaveDataSource(ctx context.Context, orgID, wsID, userID uuid.UUID, name, typ string, cfg SQLConfig) (uuid.UUID, error) {
	raw, _ := json.Marshal(cfg)
	enc, err := cryptoenc.Encrypt(e.cfg.EncryptionKey, string(raw))
	if err != nil {
		return uuid.Nil, err
	}
	id := uuid.New()
	typ = connector.Canonical(typ)
	_, err = e.pg.Exec(ctx, `
		INSERT INTO data_sources (id, org_id, workspace_id, name, type, config_enc, created_by, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, id, orgID, wsID, name, typ, enc, userID, "idle")
	return id, err
}

func (e *Engine) Discover(ctx context.Context, orgID, wsID, sourceID uuid.UUID) (DiscoverResult, error) {
	typ, cfg, err := e.loadSource(ctx, orgID, wsID, sourceID)
	if err != nil {
		return DiscoverResult{}, err
	}
	typ = connector.Canonical(typ)
	switch typ {
	case "postgres", "mysql", "mariadb", "sqlserver":
		if catalog, fks, err := e.inspectSQL(ctx, typ, cfg); err == nil {
			return inspectToDiscover(catalog, fks), nil
		}
		tables, err := e.discoverSQL(ctx, typ, cfg)
		if err != nil {
			return DiscoverResult{}, err
		}
		return DiscoverResult{Tables: tables}, nil
	case "oracle", "redshift", "snowflake", "odbc":
		tables, err := e.discoverSQL(ctx, typ, cfg)
		if err != nil {
			return DiscoverResult{}, err
		}
		return DiscoverResult{Tables: tables}, nil
	case "supabase":
		if catalog, fks, err := e.inspectSupabase(ctx, cfg); err == nil {
			return inspectToDiscover(catalog, fks), nil
		}
		tables, err := e.discoverSupabase(ctx, cfg)
		if err != nil {
			return DiscoverResult{}, err
		}
		return DiscoverResult{Tables: tables}, nil
	case "mongodb":
		tables, err := e.discoverMongo(ctx, cfg)
		if err != nil {
			return DiscoverResult{}, err
		}
		return DiscoverResult{Tables: tables}, nil
	case "bigquery":
		tables, err := e.discoverBigQuery(ctx, cfg)
		if err != nil {
			return DiscoverResult{}, err
		}
		return DiscoverResult{Tables: tables}, nil
	case "databricks":
		tables, err := e.discoverDatabricks(ctx, cfg)
		if err != nil {
			return DiscoverResult{}, err
		}
		return DiscoverResult{Tables: tables}, nil
	case "rest", "url", "odata", "json", "webhook":
		if strings.TrimSpace(cfg.URL) == "" {
			if cfg.FileName != "" {
				return DiscoverResult{Tables: []string{cfg.FileName}}, nil
			}
			return DiscoverResult{Tables: []string{"records"}}, nil
		}
		if _, _, err := e.fetchJSON(ctx, cfg); err != nil {
			return DiscoverResult{}, err
		}
		name := "records"
		if cfg.FileName != "" {
			name = cfg.FileName
		}
		return DiscoverResult{Tables: []string{name}}, nil
	case "csv", "xlsx", "parquet", "pdf":
		if cfg.FileName != "" {
			return DiscoverResult{Tables: []string{cfg.FileName}}, nil
		}
		if strings.TrimSpace(cfg.URL) != "" {
			return DiscoverResult{Tables: []string{"remote_file"}}, nil
		}
		return DiscoverResult{Tables: []string{}, Message: "Carregue um ficheiro para ingerir este conector."}, nil
	case "kafka":
		if cfg.Topic != "" {
			return DiscoverResult{Tables: []string{cfg.Topic}}, nil
		}
		return DiscoverResult{Tables: []string{"messages"}}, nil
	case "mqtt":
		if cfg.Topic != "" {
			return DiscoverResult{Tables: []string{cfg.Topic}}, nil
		}
		return DiscoverResult{Tables: []string{"messages"}}, nil
	default:
		if res := saasResources(typ); len(res) > 0 {
			return DiscoverResult{Tables: res}, nil
		}
		return DiscoverResult{Tables: []string{"records"}}, nil
	}
}

func (e *Engine) SyncSource(ctx context.Context, orgID, wsID, userID, sourceID uuid.UUID, tableOrQuery, datasetName string) (Result, error) {
	return e.SyncSourceWithSelection(ctx, orgID, wsID, userID, sourceID, tableOrQuery, datasetName, nil)
}

func (e *Engine) SyncSourceWithSelection(ctx context.Context, orgID, wsID, userID, sourceID uuid.UUID, tableOrQuery, datasetName string, sel *SourceSelection) (Result, error) {
	typ, cfg, err := e.loadSource(ctx, orgID, wsID, sourceID)
	if err != nil {
		return Result{}, err
	}
	typ = connector.Canonical(typ)
	if sel != nil {
		cfg.Selection = sel
		_ = e.SaveSelection(ctx, orgID, wsID, sourceID, sel)
	}
	if tableOrQuery != "" {
		cfg.Table = tableOrQuery
		if looksLikeSelect(tableOrQuery) {
			cfg.Query = tableOrQuery
		}
		cfg.Selection = nil
	}
	name := datasetName
	if name == "" && cfg.Selection != nil {
		name = cfg.Selection.datasetName()
	}
	if name == "" {
		name = cfg.Table
	}
	if name == "" {
		if it := connector.ByID(typ); it != nil {
			name = it.Label
		} else {
			name = typ
		}
	}

	if cfg.Selection != nil && len(cfg.Selection.Tables) > 1 && len(cfg.Selection.Joins) == 0 {
		return e.syncSelectionSeparate(ctx, orgID, wsID, userID, sourceID, typ, cfg)
	}

	headers, rows, err := e.fetchSourceRows(ctx, typ, cfg)
	if err != nil {
		return Result{}, err
	}
	return e.finishSync(ctx, orgID, wsID, userID, sourceID, typ, name, headers, rows)
}

func (e *Engine) syncSelectionSeparate(ctx context.Context, orgID, wsID, userID, sourceID uuid.UUID, typ string, cfg SQLConfig) (Result, error) {
	var last Result
	for i, t := range cfg.Selection.Tables {
		one := cfg
		sel := SourceSelection{Tables: []SelectedTable{t}}
		one.Selection = &sel
		headers, rows, err := e.fetchSourceRows(ctx, typ, one)
		if err != nil {
			return Result{}, err
		}
		name := HumanizeIdent(t.Name)
		last, err = e.finishSync(ctx, orgID, wsID, userID, sourceID, typ, name, headers, rows)
		if err != nil {
			return Result{}, err
		}
		_ = i
	}
	return last, nil
}

func (e *Engine) SaveSelection(ctx context.Context, orgID, wsID, sourceID uuid.UUID, sel *SourceSelection) error {
	typ, cfg, err := e.loadSource(ctx, orgID, wsID, sourceID)
	if err != nil {
		return err
	}
	_ = typ
	cfg.Selection = sel
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	enc, err := cryptoenc.Encrypt(e.cfg.EncryptionKey, string(raw))
	if err != nil {
		return err
	}
	_, err = e.pg.Exec(ctx, `UPDATE data_sources SET config_enc=$1, updated_at=now() WHERE id=$2 AND org_id=$3 AND workspace_id=$4`,
		enc, sourceID, orgID, wsID)
	return err
}

func (e *Engine) fetchSourceRows(ctx context.Context, typ string, cfg SQLConfig) ([]string, [][]string, error) {
	if GuidedSQLType(typ) && cfg.Selection != nil && !cfg.Selection.empty() && typ != "supabase" {
		applied, err := applySelection(typ, cfg)
		if err != nil {
			return nil, nil, err
		}
		cfg = applied
	}
	switch typ {
	case "postgres", "mysql", "mariadb", "sqlserver", "oracle", "redshift", "snowflake", "odbc":
		return e.readSQL(ctx, typ, cfg)
	case "supabase":
		return e.fetchSupabase(ctx, cfg)
	case "mongodb":
		return e.readMongo(ctx, cfg)
	case "bigquery":
		return e.readBigQuery(ctx, cfg)
	case "databricks":
		return e.readDatabricks(ctx, cfg)
	case "rest", "url", "odata", "json", "webhook":
		if typ == "odata" {
			return e.fetchOData(ctx, cfg)
		}
		return e.fetchJSON(ctx, cfg)
	case "csv", "xlsx", "parquet", "pdf":
		return e.readRemoteFile(ctx, typ, cfg)
	case "kafka":
		return e.readKafka(ctx, cfg)
	case "mqtt":
		return e.readMQTT(ctx, cfg)
	default:
		if strings.TrimSpace(cfg.URL) != "" && !saasHasNativeURL(typ) {
			return e.fetchJSON(ctx, cfg)
		}
		return e.fetchSaaS(ctx, typ, cfg)
	}
}

func (e *Engine) SourceMeta(ctx context.Context, orgID, wsID, sourceID uuid.UUID) (typ, table string, err error) {
	typ, cfg, err := e.loadSource(ctx, orgID, wsID, sourceID)
	if err != nil {
		return "", "", err
	}
	return connector.Canonical(typ), cfg.Table, nil
}

func (e *Engine) LatestDatasetForSource(ctx context.Context, orgID, wsID, sourceID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := e.pg.QueryRow(ctx, `
		SELECT id FROM datasets
		WHERE org_id=$1 AND workspace_id=$2 AND data_source_id=$3 AND status='ready'
		ORDER BY updated_at DESC LIMIT 1
	`, orgID, wsID, sourceID).Scan(&id)
	return id, err
}

func (e *Engine) DatasetSource(ctx context.Context, orgID, wsID, datasetID uuid.UUID) (uuid.UUID, string, error) {
	var src *uuid.UUID
	var name string
	err := e.pg.QueryRow(ctx, `
		SELECT data_source_id, name FROM datasets WHERE id=$1 AND org_id=$2 AND workspace_id=$3
	`, datasetID, orgID, wsID).Scan(&src, &name)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("conjunto não encontrado")
	}
	if src == nil || *src == uuid.Nil {
		return uuid.Nil, name, fmt.Errorf("conjunto sem conector associado")
	}
	return *src, name, nil
}

type RefreshResult struct {
	DatasetID uuid.UUID `json:"dataset_id"`
	Name      string    `json:"name"`
	RowCount  int64     `json:"row_count"`
	Created   bool      `json:"created"`
	Mode      string    `json:"mode"`
}

func (e *Engine) RefreshSource(ctx context.Context, orgID, wsID, userID, sourceID uuid.UUID, tableOrQuery, datasetName string) (RefreshResult, error) {
	typ, cfg, err := e.loadSource(ctx, orgID, wsID, sourceID)
	if err != nil {
		return RefreshResult{}, err
	}
	typ = connector.Canonical(typ)
	if tableOrQuery != "" {
		cfg.Table = tableOrQuery
		if looksLikeSelect(tableOrQuery) {
			cfg.Query = tableOrQuery
		}
		cfg.Selection = nil
	}
	name := datasetName
	if name == "" && cfg.Selection != nil {
		name = cfg.Selection.datasetName()
	}
	if name == "" {
		name = cfg.Table
	}
	if name == "" {
		if it := connector.ByID(typ); it != nil {
			name = it.Label
		} else {
			name = typ
		}
	}
	if cfg.Selection != nil && len(cfg.Selection.Tables) > 1 && len(cfg.Selection.Joins) == 0 {
		res, err := e.syncSelectionSeparate(ctx, orgID, wsID, userID, sourceID, typ, cfg)
		if err != nil {
			return RefreshResult{}, err
		}
		return RefreshResult{DatasetID: res.DatasetID, Name: res.Name, RowCount: res.RowCount, Created: true, Mode: "full"}, nil
	}
	headers, rows, err := e.fetchSourceRows(ctx, typ, cfg)
	if err != nil {
		return RefreshResult{}, err
	}
	if len(headers) == 0 {
		return RefreshResult{}, fmt.Errorf("nenhuma coluna devolvida — verifique o recurso e as credenciais")
	}
	if len(rows) == 0 {
		return RefreshResult{}, fmt.Errorf("nenhuma linha devolvida — verifique credenciais, permissões e o recurso escolhido")
	}
	if existing, err := e.LatestDatasetForSource(ctx, orgID, wsID, sourceID); err == nil && existing != uuid.Nil {
		n, err := e.ReplaceDataset(ctx, orgID, wsID, existing, headers, rows)
		if err != nil {
			return RefreshResult{}, err
		}
		e.LinkDataset(ctx, orgID, sourceID, existing)
		return RefreshResult{DatasetID: existing, Name: name, RowCount: n, Created: false, Mode: "full"}, nil
	}
	res, err := e.finishSync(ctx, orgID, wsID, userID, sourceID, typ, name, headers, rows)
	if err != nil {
		return RefreshResult{}, err
	}
	return RefreshResult{DatasetID: res.DatasetID, Name: res.Name, RowCount: res.RowCount, Created: true, Mode: "full"}, nil
}

func looksLikeSelect(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(strings.ToLower(s)), "select")
}

func saasHasNativeURL(typ string) bool {
	switch typ {
	case "asaas", "conta_azul", "bitrix24", "omie", "salesforce", "instagram", "facebook", "google_business", "mercado_livre", "ibge_censo", "inflacao", "expectativas", "cambio", "contabilidade":
		return true
	}
	return false
}

func (e *Engine) finishSync(ctx context.Context, orgID, wsID, userID, sourceID uuid.UUID, typ, name string, headers []string, rows [][]string) (Result, error) {
	if len(headers) == 0 {
		return Result{}, fmt.Errorf("nenhuma coluna devolvida — verifique o recurso e as credenciais")
	}
	if len(rows) == 0 {
		return Result{}, fmt.Errorf("nenhuma linha devolvida — verifique credenciais, permissões e o recurso escolhido")
	}
	res, err := e.ingestRows(ctx, orgID, wsID, userID, name, typ, headers, rows)
	if err != nil {
		return Result{}, err
	}
	e.LinkDataset(ctx, orgID, sourceID, res.DatasetID)
	_ = e.writeLake(ctx, orgID, wsID, res.DatasetID, name, headers, rows)
	return res, nil
}

func (e *Engine) SyncSQL(ctx context.Context, orgID, wsID, userID, sourceID uuid.UUID, tableOrQuery, datasetName string) (Result, error) {
	typ, cfg, err := e.loadSource(ctx, orgID, wsID, sourceID)
	if err != nil {
		return Result{}, err
	}
	if tableOrQuery != "" {
		cfg.Table = tableOrQuery
	}
	headers, rows, err := e.readSQL(ctx, typ, cfg)
	if err != nil {
		return Result{}, err
	}
	name := datasetName
	if name == "" {
		name = cfg.Table
		if name == "" {
			name = "sql_import"
		}
	}
	res, err := e.ingestRows(ctx, orgID, wsID, userID, name, typ, headers, rows)
	if err != nil {
		return Result{}, err
	}
	_, _ = e.pg.Exec(ctx, `UPDATE datasets SET data_source_id=$2 WHERE id=$1`, res.DatasetID, sourceID)
	_, _ = e.pg.Exec(ctx, `UPDATE data_sources SET last_sync_at=now(), status='synced' WHERE id=$1`, sourceID)
	_ = e.writeLake(ctx, orgID, wsID, res.DatasetID, name, headers, rows)
	return res, nil
}

func (e *Engine) ingestRows(ctx context.Context, orgID, wsID, userID uuid.UUID, name, kind string, headers []string, rows [][]string) (Result, error) {
	if len(headers) == 0 {
		return Result{}, fmt.Errorf("no columns")
	}
	cols := make([]schemax.Column, len(headers))
	names := schemax.UniqueNames(headers)
	sampleN := min(len(rows), 2000)
	for i, n := range names {
		values := make([]string, 0, sampleN)
		for _, row := range rows[:sampleN] {
			if i < len(row) {
				values = append(values, row[i])
			}
		}
		cols[i] = schemax.Column{Name: n, SourceName: headers[i], Type: schemax.InferType(values)}
	}
	q := quality.Analyze(cols, rows[:min(len(rows), 50000)])
	for i := range cols {
		cols[i].Role = schemax.GuessRole(cols[i])
	}
	dsID := uuid.New()
	table := "ds_" + strings.ReplaceAll(dsID.String(), "-", "")
	slug := schemax.SanitizeIdent(name) + "-" + dsID.String()[:8]
	schemaJSON, _ := json.Marshal(cols)
	qualityJSON, _ := json.Marshal(q)
	jobID := uuid.New()

	_, err := e.pg.Exec(ctx, `
		INSERT INTO datasets (id, org_id, workspace_id, name, slug, clickhouse_table, schema_json, quality_score, quality_json, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'ingesting')
	`, dsID, orgID, wsID, name, slug, table, schemaJSON, q.Score, qualityJSON)
	if err != nil {
		return Result{}, err
	}
	_, _ = e.pg.Exec(ctx, `INSERT INTO ingestion_jobs (id, org_id, workspace_id, dataset_id, kind, status, started_at) VALUES ($1,$2,$3,$4,$5,'running',now())`,
		jobID, orgID, wsID, dsID, kind)

	if err := e.createTable(ctx, table, cols); err != nil {
		e.fail(ctx, dsID, jobID, err)
		return Result{}, err
	}
	inserted, err := e.insertRows(ctx, table, orgID, cols, rows)
	if err != nil {
		e.fail(ctx, dsID, jobID, err)
		return Result{}, err
	}
	model := semantic.Suggest(name, cols)
	model.DatasetID = dsID.String()
	modelJSON, _ := json.Marshal(model)
	_, err = e.pg.Exec(ctx, `INSERT INTO semantic_models (org_id, workspace_id, dataset_id, name, status, model_json) VALUES ($1,$2,$3,$4,'published',$5)`,
		orgID, wsID, dsID, model.Name, modelJSON)
	if err != nil {
		e.fail(ctx, dsID, jobID, err)
		return Result{}, err
	}
	_, _ = e.pg.Exec(ctx, `UPDATE datasets SET status='ready', row_count=$2, updated_at=now() WHERE id=$1`, dsID, inserted)
	_, _ = e.pg.Exec(ctx, `UPDATE ingestion_jobs SET status='completed', finished_at=now() WHERE id=$1`, jobID)
	_ = userID
	return Result{DatasetID: dsID, JobID: jobID, Name: name, Table: table, RowCount: inserted, Schema: cols, Quality: q, Semantic: model, Status: "ready"}, nil
}

func (e *Engine) readSQL(ctx context.Context, typ string, cfg SQLConfig) ([]string, [][]string, error) {
	if typ == "odbc" {
		typ = detectODBCType(cfg)
		if strings.TrimSpace(cfg.URL) != "" && (typ == "postgres" || typ == "redshift") {
			cfg.URL = strings.TrimSpace(cfg.URL)
		}
	}
	q := cfg.Query
	if q == "" {
		if cfg.Table == "" {
			return nil, nil, fmt.Errorf("indique uma tabela ou uma consulta SELECT")
		}
		if !tableIdentOK(cfg.Table) {
			return nil, nil, fmt.Errorf("invalid table name")
		}
		q = selectStarSQL(typ, cfg.Table, cfg.RowLimit())
	} else if !strings.Contains(strings.ToLower(q), " limit ") && !strings.Contains(strings.ToLower(q), " top ") {
		q = applySQLLimit(typ, q, cfg.RowLimit())
	}
	if !safeSelect(q) {
		return nil, nil, fmt.Errorf("only read-only SELECT is allowed")
	}

	switch typ {
	case "postgres", "redshift":
		conn, err := pgx.Connect(ctx, postgresDSN(cfg))
		if err != nil {
			return nil, nil, err
		}
		defer conn.Close(ctx)
		rows, err := conn.Query(ctx, q)
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close()
		fds := rows.FieldDescriptions()
		headers := make([]string, len(fds))
		for i, f := range fds {
			headers[i] = string(f.Name)
		}
		var out [][]string
		lim := cfg.RowLimit()
		for rows.Next() {
			vals, err := rows.Values()
			if err != nil {
				return nil, nil, err
			}
			rec := make([]string, len(vals))
			for i, v := range vals {
				rec[i] = stringify(v)
			}
			out = append(out, rec)
			if len(out) >= lim {
				break
			}
		}
		return headers, out, rows.Err()
	default:
		db, _, err := openSQL(typ, cfg)
		if err != nil {
			return nil, nil, err
		}
		defer db.Close()
		return scanSQLQuery(ctx, db, q, cfg.RowLimit())
	}
}

func (e *Engine) loadSource(ctx context.Context, orgID, wsID, id uuid.UUID) (string, SQLConfig, error) {
	var typ, enc string
	err := e.pg.QueryRow(ctx, `SELECT type, config_enc FROM data_sources WHERE id=$1 AND org_id=$2 AND workspace_id=$3`,
		id, orgID, wsID).Scan(&typ, &enc)
	if err != nil {
		return "", SQLConfig{}, fmt.Errorf("data source not found")
	}
	plain, err := cryptoenc.Decrypt(e.cfg.EncryptionKey, enc)
	if err != nil {
		return "", SQLConfig{}, err
	}
	var cfg SQLConfig
	if err := json.Unmarshal([]byte(plain), &cfg); err != nil {
		return "", SQLConfig{}, err
	}
	return typ, cfg, nil
}

func postgresDSN(c SQLConfig) string {
	port := c.Port
	if port == 0 {
		port = 5432
	}
	ssl := c.EffectiveSSLMode()
	if strings.TrimSpace(c.URL) != "" && strings.Contains(strings.ToLower(c.URL), "postgres") {
		return c.URL
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", urlQueryEscape(c.User), urlQueryEscape(c.Password), c.Host, port, c.Database, ssl)
}

func urlQueryEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func mysqlDSN(c SQLConfig) string {
	port := c.Port
	if port == 0 {
		port = 3306
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4", c.User, c.Password, c.Host, port, c.Database)
}

func safeSelect(q string) bool {
	s := strings.TrimSpace(strings.ToLower(q))
	if !strings.HasPrefix(s, "select") {
		return false
	}
	banned := []string{"insert", "update", "delete", "drop", "alter", "truncate", "grant", "revoke", "copy ", ";"}
	for _, b := range banned {
		if strings.Contains(s, b) {
			return false
		}
	}
	return true
}

func tableIdentOK(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, " ;\"'") {
		return false
	}
	return true
}

func (e *Engine) readSQLBound(ctx context.Context, typ string, cfg SQLConfig, q string, arg string) ([]string, [][]string, error) {
	switch typ {
	case "postgres", "supabase":
		if typ == "supabase" {
			cfg = supabasePreparedSQL(cfg)
		}
		conn, err := pgx.Connect(ctx, postgresDSN(cfg))
		if err != nil {
			return nil, nil, err
		}
		defer conn.Close(ctx)
		rows, err := conn.Query(ctx, q, arg)
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close()
		fds := rows.FieldDescriptions()
		headers := make([]string, len(fds))
		for i, f := range fds {
			headers[i] = string(f.Name)
		}
		var out [][]string
		for rows.Next() {
			vals, err := rows.Values()
			if err != nil {
				return nil, nil, err
			}
			rec := make([]string, len(vals))
			for i, v := range vals {
				rec[i] = stringify(v)
			}
			out = append(out, rec)
			if len(out) >= 100000 {
				break
			}
		}
		return headers, out, rows.Err()
	case "mysql":
		db, err := sql.Open("mysql", mysqlDSN(cfg))
		if err != nil {
			return nil, nil, err
		}
		defer db.Close()
		rows, err := db.QueryContext(ctx, q, arg)
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close()
		headers, err := rows.Columns()
		if err != nil {
			return nil, nil, err
		}
		var out [][]string
		for rows.Next() {
			raw := make([]sql.NullString, len(headers))
			ptrs := make([]any, len(headers))
			for i := range raw {
				ptrs[i] = &raw[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				return nil, nil, err
			}
			rec := make([]string, len(headers))
			for i, v := range raw {
				if v.Valid {
					rec[i] = v.String
				}
			}
			out = append(out, rec)
		}
		return headers, out, rows.Err()
	default:
		return nil, nil, fmt.Errorf("CDC incremental só cobre PostgreSQL e MySQL")
	}
}

func stringify(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case time.Time:
		return t.UTC().Format(time.RFC3339)
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(t)
	}
}

func parseHeaderLines(s string) HeaderMap {
	out := HeaderMap{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out
}

func (e *Engine) GetDataSource(ctx context.Context, orgID, wsID, id uuid.UUID) (SourceView, error) {
	var v SourceView
	err := e.pg.QueryRow(ctx, `
		SELECT id, name, type, status, last_sync_at, created_at, updated_at
		FROM data_sources WHERE id=$1 AND org_id=$2 AND workspace_id=$3
	`, id, orgID, wsID).Scan(&v.ID, &v.Name, &v.Type, &v.Status, &v.LastSyncAt, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return SourceView{}, fmt.Errorf("fonte não encontrada")
	}
	_, cfg, err := e.loadSource(ctx, orgID, wsID, id)
	if err == nil {
		v.Config = RedactConfig(cfg)
	} else {
		v.Config = map[string]any{}
	}
	v.Implemented = connector.Implemented(v.Type)
	v.Preview = !v.Implemented || v.Status == "preview"
	if v.Preview {
		v.Message = connector.PreviewMessage(v.Type)
	}
	rows, err := e.pg.Query(ctx, `
		SELECT id, name, status, row_count, COALESCE(storage_mode, 'import')
		FROM datasets WHERE data_source_id=$1 AND org_id=$2 AND workspace_id=$3 ORDER BY updated_at DESC
	`, id, orgID, wsID)
	if err != nil {
		v.Datasets = []SourceDataset{}
		return v, nil
	}
	defer rows.Close()
	v.Datasets = []SourceDataset{}
	for rows.Next() {
		var d SourceDataset
		if err := rows.Scan(&d.ID, &d.Name, &d.Status, &d.RowCount, &d.StorageMode); err != nil {
			return v, err
		}
		v.Datasets = append(v.Datasets, d)
	}
	return v, rows.Err()
}

func (e *Engine) DeleteDataSource(ctx context.Context, orgID, wsID, id uuid.UUID) error {
	tag, err := e.pg.Exec(ctx, `DELETE FROM data_sources WHERE id=$1 AND org_id=$2 AND workspace_id=$3`, id, orgID, wsID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("fonte não encontrada")
	}
	return nil
}

func (e *Engine) TestConnection(ctx context.Context, orgID, wsID, id uuid.UUID) (TestResult, error) {
	typ, cfg, err := e.loadSource(ctx, orgID, wsID, id)
	if err != nil {
		return TestResult{}, err
	}
	typ = connector.Canonical(typ)
	label := typ
	if it := connector.ByID(typ); it != nil {
		label = it.Label
	}
	if err := e.pingSource(ctx, typ, cfg); err != nil {
		return TestResult{}, fmt.Errorf("falha a ligar a %s: %w", label, err)
	}
	return TestResult{OK: true, Implemented: true, Message: "Ligação " + label + " bem sucedida."}, nil
}

func (e *Engine) pingSource(ctx context.Context, typ string, cfg SQLConfig) error {
	switch typ {
	case "postgres", "redshift":
		conn, err := pgx.Connect(ctx, postgresDSN(cfg))
		if err != nil {
			return err
		}
		defer conn.Close(ctx)
		return conn.Ping(ctx)
	case "supabase":
		return e.pingSupabase(ctx, cfg)
	case "mysql", "mariadb", "sqlserver", "oracle", "snowflake":
		db, dsn, err := openSQL(typ, cfg)
		if err != nil {
			return err
		}
		_ = dsn
		defer db.Close()
		return db.PingContext(ctx)
	case "odbc":
		return e.pingSource(ctx, detectODBCType(cfg), cfg)
	case "mongodb":
		return e.pingMongo(ctx, cfg)
	case "bigquery":
		return e.pingBigQuery(ctx, cfg)
	case "databricks":
		return e.pingDatabricks(ctx, cfg)
	case "rest", "url", "odata", "webhook", "json":
		if strings.TrimSpace(cfg.URL) == "" {
			return fmt.Errorf("URL obrigatório")
		}
		return e.pingHTTP(ctx, cfg)
	case "csv", "xlsx", "parquet", "pdf":
		if cfg.FileName == "" && strings.TrimSpace(cfg.URL) == "" {
			return fmt.Errorf("carregue um ficheiro ou indique um URL")
		}
		return nil
	case "kafka":
		return e.pingKafka(cfg)
	case "mqtt":
		return e.pingMQTT(cfg)
	default:
		return e.pingSaaS(ctx, typ, cfg)
	}
}

func (e *Engine) LinkDataset(ctx context.Context, orgID, sourceID, datasetID uuid.UUID) {
	_, _ = e.pg.Exec(ctx, `UPDATE datasets SET data_source_id=$1 WHERE id=$2 AND org_id=$3`, sourceID, datasetID, orgID)
	_, _ = e.pg.Exec(ctx, `UPDATE data_sources SET last_sync_at=now(), status='synced', updated_at=now() WHERE id=$1 AND org_id=$2`, sourceID, orgID)
}
