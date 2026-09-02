package connector

import "strings"

// Static Data Connector Hub catalog. Every type has a real sync path
// (SQL driver, file parser, or REST fetcher) that materializes a ClickHouse dataset.

type Field struct {
	Key         string        `json:"key"`
	Label       string        `json:"label"`
	Type        string        `json:"type"` // text, password, number, url, textarea, checkbox, file, select
	Required    bool          `json:"required,omitempty"`
	Placeholder string        `json:"placeholder,omitempty"`
	Default     string        `json:"default,omitempty"`
	Hint        string        `json:"hint,omitempty"`
	Options     []FieldOption `json:"options,omitempty"`
	Accept      string        `json:"accept,omitempty"`
}

type FieldOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Item struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Group       string   `json:"group"`
	GroupLabel  string   `json:"group_label"`
	Description string   `json:"description"`
	Implemented bool     `json:"implemented"`
	Preview     bool     `json:"preview"`
	Message     string   `json:"message,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	Icon        string   `json:"icon,omitempty"`
	Fields      []Field  `json:"fields"`
}

type Group struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

const (
	GroupDatabases = "databases"
	GroupFiles     = "files"
	GroupBrasil    = "negocio_brasil"
	GroupAds       = "publicidade"
	GroupSocial    = "redes_crm"
	GroupEcon      = "dados_economicos"
	GroupCloud     = "cloud"
	GroupWeb       = "web"
	GroupStreaming = "streaming"
)

var groupLabels = map[string]string{
	GroupDatabases: "Bases de dados",
	GroupFiles:     "Ficheiros",
	GroupBrasil:    "Negócio Brasil",
	GroupAds:       "Publicidade",
	GroupSocial:    "Redes e CRM",
	GroupEcon:      "Dados económicos / Brasil",
	GroupCloud:     "Cloud/SaaS",
	GroupWeb:       "Web/API",
	GroupStreaming: "Streaming",
}

// typeAliases maps user-facing / typo ids onto catalog ids.
var typeAliases = map[string]string{
	"sql":                "sqlserver",
	"mssql":              "sqlserver",
	"ms_sql":             "sqlserver",
	"excel":              "xlsx",
	"xls":                "xlsx",
	"postgresql":         "postgres",
	"pg":                 "postgres",
	"assas":              "asaas",
	"contaazul":          "conta_azul",
	"metaads":            "meta_ads",
	"meta":               "meta_ads",
	"facebook_ads":       "meta_ads",
	"facebookads":        "meta_ads",
	"googleads":          "google_ads",
	"ga":                 "google_analytics",
	"ga4":                "google_analytics",
	"mariadb":            "mariadb",
	"http":               "rest",
	"fb":                 "facebook",
	"gmb":                "google_business",
	"google_meu_negocio": "google_business",
	"googlemeunegocio":   "google_business",
	"gsheets":            "google_sheets",
	"sheets":             "google_sheets",
	"planilha":           "google_sheets",
	"googlesheets":       "google_sheets",
	"google-sheets":      "google_sheets",
	"formulario":         "manual",
	"form":               "manual",
	"enter_data":         "manual",
	"planilha_manual":    "manual",
	"manual_entry":       "manual",
	"mercadolivre":       "mercado_livre",
	"ml":                 "mercado_livre",
	"ibge":               "ibge_censo",
	"censo":              "ibge_censo",
	"ipca":               "inflacao",
	"forex":              "cambio",
	"ptax":               "cambio",
	"focus":              "expectativas",
	"ofx":                "contabilidade",
	"supabase.co":        "supabase",
}

func Canonical(typ string) string {
	t := strings.TrimSpace(strings.ToLower(typ))
	if canon, ok := typeAliases[t]; ok {
		return canon
	}
	return t
}

func Groups() []Group {
	return []Group{
		{ID: GroupDatabases, Label: groupLabels[GroupDatabases]},
		{ID: GroupFiles, Label: groupLabels[GroupFiles]},
		{ID: GroupBrasil, Label: groupLabels[GroupBrasil]},
		{ID: GroupAds, Label: groupLabels[GroupAds]},
		{ID: GroupSocial, Label: groupLabels[GroupSocial]},
		{ID: GroupEcon, Label: groupLabels[GroupEcon]},
		{ID: GroupCloud, Label: groupLabels[GroupCloud]},
		{ID: GroupWeb, Label: groupLabels[GroupWeb]},
		{ID: GroupStreaming, Label: groupLabels[GroupStreaming]},
	}
}

func brandIcon(id string) string {
	switch id {
	case "asaas", "conta_azul", "ibge_censo", "odata", "inflacao", "expectativas", "cambio":
		return "/connectors/" + id + ".png"
	default:
		return "/connectors/" + id + ".svg"
	}
}

func Catalog() []Item {
	items := []Item{
		sqlItem("postgres", "PostgreSQL", "5432", "Sincronização nativa de tabelas e consultas SELECT.", []string{"pg", "postgresql"}),
		{
			ID: "supabase", Label: "Supabase", Group: GroupDatabases, GroupLabel: groupLabels[GroupDatabases],
			Description: "Postgres gerido (pgx, SSL obrigatório) ou API PostgREST /rest/v1/.", Implemented: true, Preview: false,
			Aliases: []string{"supabase.co"},
			Fields: append(nameField(), []Field{
				{Key: "project_url", Label: "URL do projecto", Type: "url", Placeholder: "https://xxxx.supabase.co", Hint: "Opcional. Usado na API REST e, se o anfitrião estiver vazio, para derivar db.xxxx.supabase.co."},
				{Key: "host", Label: "Anfitrião", Type: "text", Placeholder: "db.xxxx.supabase.co", Hint: "Ligação Postgres (preferida quando preenchida com a senha da base)."},
				{Key: "port", Label: "Porta", Type: "number", Default: "5432"},
				{Key: "database", Label: "Base de dados", Type: "text", Default: "postgres"},
				{Key: "user", Label: "Utilizador", Type: "text", Default: "postgres"},
				{Key: "password", Label: "Senha da base", Type: "password", Hint: "Database password do projecto (Settings → Database), não a anon key."},
				{Key: "ssl_mode", Label: "Modo SSL", Type: "select", Default: "require", Options: []FieldOption{
					{Value: "require", Label: "Obrigatório"},
					{Value: "verify-full", Label: "Verificar certificado"},
					{Value: "disable", Label: "Desligado"},
				}},
				{Key: "service_role_key", Label: "Service role key", Type: "password", Hint: "Opcional. Para GET /rest/v1/ (cabeçalhos apikey e Authorization Bearer)."},
				{Key: "anon_key", Label: "Anon key", Type: "password", Hint: "Alternativa à service role key — a RLS do projecto aplica-se."},
				{Key: "table", Label: "Tabela (opcional)", Type: "text", Placeholder: "public.vendas", Hint: "Usada no primeiro sync. No REST, indique o nome da tabela exposta no PostgREST."},
				limitField(),
			}...),
		},
		sqlItem("mysql", "MySQL", "3306", "Sincronização nativa de tabelas e consultas SELECT.", nil),
		sqlItem("mariadb", "MariaDB", "3306", "Mesmo protocolo MySQL — listagem de tabelas e SELECT.", []string{"maria"}),
		sqlItem("sqlserver", "SQL Server", "1433", "SQL genérico (Microsoft SQL Server). Listagem de tabelas e SELECT.", []string{"SQL", "mssql", "sql"}),
		sqlItem("oracle", "Oracle", "1521", "Ligação go-ora (sem CGO). Listagem ALL_TABLES e SELECT.", nil),
		sqlItem("redshift", "Amazon Redshift", "5439", "Protocolo PostgreSQL (pgx) na porta 5439.", []string{"aws_redshift"}),
		{
			ID: "snowflake", Label: "Snowflake", Group: GroupDatabases, GroupLabel: groupLabels[GroupDatabases],
			Description: "Warehouse Snowflake via driver nativo.", Implemented: true, Preview: false,
			Fields: append(nameField(), []Field{
				{Key: "account", Label: "Conta", Type: "text", Required: true, Placeholder: "xy12345.eu-central-1"},
				{Key: "user", Label: "Utilizador", Type: "text", Required: true},
				{Key: "password", Label: "Senha", Type: "password", Required: true},
				{Key: "warehouse", Label: "Warehouse", Type: "text"},
				{Key: "database", Label: "Base de dados", Type: "text", Required: true},
				{Key: "schema", Label: "Esquema", Type: "text", Default: "PUBLIC"},
				{Key: "role", Label: "Role", Type: "text"},
				{Key: "table", Label: "Tabela (opcional)", Type: "text"},
				limitField(),
			}...),
		},
		{
			ID: "bigquery", Label: "BigQuery", Group: GroupDatabases, GroupLabel: groupLabels[GroupDatabases],
			Description: "Google BigQuery via REST (token OAuth ou JSON de conta de serviço).", Implemented: true, Preview: false,
			Fields: append(nameField(), []Field{
				{Key: "project", Label: "Projecto", Type: "text", Required: true},
				{Key: "dataset", Label: "Dataset", Type: "text"},
				{Key: "table", Label: "Tabela (opcional)", Type: "text"},
				{Key: "token", Label: "Token OAuth ou JSON da conta de serviço", Type: "textarea", Required: true, Hint: "Cole um access token ou a chave JSON. O sync corre SELECT * LIMIT n."},
				limitField(),
			}...),
		},
		{
			ID: "databricks", Label: "Databricks", Group: GroupDatabases, GroupLabel: groupLabels[GroupDatabases],
			Description: "SQL Warehouse via Statement Execution API.", Implemented: true, Preview: false,
			Fields: append(nameField(), []Field{
				{Key: "host", Label: "Anfitrião", Type: "text", Required: true, Placeholder: "adb-xxxx.azuredatabricks.net"},
				{Key: "http_path", Label: "HTTP path", Type: "text", Placeholder: "/sql/1.0/warehouses/…", Hint: "Usado para obter o warehouse_id."},
				{Key: "token", Label: "Token", Type: "password", Required: true},
				{Key: "catalog", Label: "Catálogo", Type: "text"},
				{Key: "schema", Label: "Esquema", Type: "text"},
				{Key: "table", Label: "Tabela (opcional)", Type: "text"},
				limitField(),
			}...),
		},
		{
			ID: "mongodb", Label: "MongoDB", Group: GroupDatabases, GroupLabel: groupLabels[GroupDatabases],
			Description: "Lista colecções e faz Find com limite.", Implemented: true, Preview: false,
			Fields: append(nameField(), []Field{
				{Key: "host", Label: "Anfitrião", Type: "text", Required: true, Default: "localhost"},
				{Key: "port", Label: "Porta", Type: "number", Default: "27017"},
				{Key: "database", Label: "Base de dados", Type: "text", Required: true},
				{Key: "user", Label: "Utilizador", Type: "text"},
				{Key: "password", Label: "Senha", Type: "password"},
				{Key: "table", Label: "Colecção (opcional)", Type: "text"},
				{Key: "url", Label: "URI mongodb:// (opcional)", Type: "text", Placeholder: "mongodb://localhost:27017"},
				limitField(),
			}...),
		},
		{
			ID: "odbc", Label: "ODBC / DSN", Group: GroupDatabases, GroupLabel: groupLabels[GroupDatabases],
			Description: "Tenta postgres, mysql ou sqlserver conforme o prefixo da connection string.", Implemented: true, Preview: false,
			Fields: append(nameField(), []Field{
				{Key: "url", Label: "Connection string", Type: "textarea", Required: true, Placeholder: "postgres://… ou mysql://… ou sqlserver://…"},
				{Key: "user", Label: "Utilizador", Type: "text"},
				{Key: "password", Label: "Senha", Type: "password"},
				{Key: "table", Label: "Tabela (opcional)", Type: "text"},
				limitField(),
			}...),
		},

		{
			ID: "manual", Label: "Planilha manual", Group: GroupFiles, GroupLabel: groupLabels[GroupFiles],
			Description: "Crie a sua própria tabela: defina colunas e preencha os dados num formulário, sem ligação externa.",
			Implemented: true, Preview: false,
			Aliases: []string{"formulario", "form", "enter_data", "planilha_manual", "manual_entry"},
			Fields: append(nameField(), []Field{
				{Key: "table", Label: "Nome da tabela", Type: "text", Placeholder: "Vendas da loja", Hint: "Como a planilha aparece nos conjuntos e no Ask TheDobra."},
			}...),
		},
		fileItem("csv", "CSV", ".csv,.tsv,text/csv", "Upload e ingestão nativa.", []string{"tsv"}),
		fileItem("xlsx", "Excel", ".xlsx,.xls,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/vnd.ms-excel", "Upload .xlsx/.xls e ingestão da primeira folha.", []string{"Excel", "xls", "excel"}),
		{
			ID: "google_sheets", Label: "Google Sheets", Group: GroupFiles, GroupLabel: groupLabels[GroupFiles],
			Description: "Cole o link da planilha. Partilhe com «qualquer pessoa com o link» — sem chave de API.", Implemented: true, Preview: false,
			Aliases: []string{"gsheets", "planilha", "sheets", "googlesheets"},
			Fields: append(nameField(), []Field{
				{Key: "url", Label: "Link da planilha", Type: "text", Required: true, Placeholder: "https://docs.google.com/spreadsheets/d/…", Hint: "Cole o URL completo (ou só o ID). A planilha precisa de estar partilhada com «qualquer pessoa com o link» em modo Leitor."},
				{Key: "table", Label: "Folha (opcional)", Type: "text", Placeholder: "Página1", Hint: "Vazio = primeira folha. Pode usar o nome ou o gid do URL (#gid=123)."},
			}...),
		},
		fileItem("json", "JSON / NDJSON", ".json,.ndjson,application/json", "Ingere um array de objectos ou NDJSON.", nil),
		fileItem("parquet", "Parquet", ".parquet", "Lê colunas do ficheiro Parquet para um conjunto.", nil),
		fileItem("pdf", "PDF", ".pdf,application/pdf", "Extrai tabelas simples ou linhas de texto.", nil),

		{
			ID: "rest", Label: "REST JSON", Group: GroupWeb, GroupLabel: groupLabels[GroupWeb],
			Description: "GET a um endpoint JSON (array, data, value ou results).", Implemented: true, Preview: false,
			Fields: append(nameField(), httpFields(true)...),
		},
		{
			ID: "url", Label: "URL JSON", Group: GroupWeb, GroupLabel: groupLabels[GroupWeb],
			Description: "URL público ou autenticado que devolve JSON.", Implemented: true, Preview: false,
			Fields: append(nameField(), httpFields(true)...),
		},
		{
			ID: "odata", Label: "OData", Group: GroupWeb, GroupLabel: groupLabels[GroupWeb],
			Description: "Segue @odata.nextLink e lê a propriedade value.", Implemented: true, Preview: false,
			Fields: append(nameField(), httpFields(true)...),
		},

		{
			ID: "asaas", Label: "Asaas", Group: GroupBrasil, GroupLabel: groupLabels[GroupBrasil],
			Description: "Pagamentos BR — clientes e cobranças via API v3.", Implemented: true, Preview: false,
			Aliases: []string{"assas"},
			Fields: append(nameField(), []Field{
				{Key: "api_key", Label: "Chave de API", Type: "password", Required: true, Hint: "Enviada no cabeçalho access_token."},
				{Key: "environment", Label: "Ambiente", Type: "select", Default: "sandbox", Options: []FieldOption{
					{Value: "sandbox", Label: "Sandbox"},
					{Value: "prod", Label: "Produção"},
				}},
				{Key: "webhook_url", Label: "Webhook (opcional)", Type: "url", Placeholder: "https://…"},
				{Key: "url", Label: "URL da API (opcional)", Type: "url", Hint: "Substitui a base sandbox/produção. Se apontar para um JSON, o sync faz GET directo."},
				{Key: "table", Label: "Recurso", Type: "select", Default: "customers", Options: []FieldOption{
					{Value: "customers", Label: "Clientes (/customers)"},
					{Value: "payments", Label: "Cobranças (/payments)"},
				}},
				limitField(),
			}...),
		},
		{
			ID: "conta_azul", Label: "Conta Azul", Group: GroupBrasil, GroupLabel: groupLabels[GroupBrasil],
			Description: "ERP BR — vendas e clientes (API v1, Bearer).", Implemented: true, Preview: false,
			Aliases: []string{"contaazul"},
			Fields: append(nameField(), []Field{
				{Key: "client_id", Label: "Client ID", Type: "text", Required: true},
				{Key: "client_secret", Label: "Client secret", Type: "password", Required: true},
				{Key: "access_token", Label: "Access token (opcional)", Type: "password", Hint: "Se vazio, tenta OAuth client_credentials."},
				{Key: "url", Label: "URL da API (opcional)", Type: "url", Placeholder: "https://api.contaazul.com"},
				{Key: "table", Label: "Recurso", Type: "select", Default: "vendas", Options: []FieldOption{
					{Value: "vendas", Label: "Vendas"},
					{Value: "pessoas", Label: "Clientes / pessoas"},
				}},
				limitField(),
			}...),
		},
		{
			ID: "bitrix24", Label: "Bitrix24", Group: GroupBrasil, GroupLabel: groupLabels[GroupBrasil],
			Description: "CRM — deals e contactos via webhook de entrada.", Implemented: true, Preview: false,
			Fields: append(nameField(), []Field{
				{Key: "webhook_url", Label: "URL do webhook", Type: "url", Placeholder: "https://xxx.bitrix24.com.br/rest/1/xxx/", Hint: "Ou preencha domínio + token."},
				{Key: "domain", Label: "Domínio", Type: "text", Placeholder: "empresa.bitrix24.com.br"},
				{Key: "access_token", Label: "Access token", Type: "password"},
				{Key: "table", Label: "Método", Type: "select", Default: "crm.deal.list", Options: []FieldOption{
					{Value: "crm.deal.list", Label: "Negócios (crm.deal.list)"},
					{Value: "crm.contact.list", Label: "Contactos (crm.contact.list)"},
				}},
				limitField(),
			}...),
		},
		{
			ID: "omie", Label: "Omie", Group: GroupBrasil, GroupLabel: groupLabels[GroupBrasil],
			Description: "ERP BR — ListarClientes / ListarPedidos com app_key e app_secret.", Implemented: true, Preview: false,
			Fields: append(nameField(), []Field{
				{Key: "app_key", Label: "App key", Type: "password", Required: true},
				{Key: "app_secret", Label: "App secret", Type: "password", Required: true},
				{Key: "url", Label: "URL da API (opcional)", Type: "url", Placeholder: "https://app.omie.com.br/api/v1/geral/clientes/"},
				{Key: "table", Label: "Recurso", Type: "select", Default: "clientes", Options: []FieldOption{
					{Value: "clientes", Label: "Clientes (ListarClientes)"},
					{Value: "pedidos", Label: "Pedidos (ListarPedidos)"},
				}},
				limitField(),
			}...),
		},
		{
			ID: "google_ads", Label: "Google Ads", Group: GroupAds, GroupLabel: groupLabels[GroupAds],
			Description: "Campanhas via Google Ads API (GAQL search).", Implemented: true, Preview: false,
			Aliases: []string{"googleads"},
			Fields: append(nameField(), []Field{
				{Key: "developer_token", Label: "Developer token", Type: "password", Required: true},
				{Key: "client_id", Label: "Client ID", Type: "text"},
				{Key: "client_secret", Label: "Client secret", Type: "password"},
				{Key: "refresh_token", Label: "Refresh token", Type: "password"},
				{Key: "access_token", Label: "Access token (opcional)", Type: "password", Hint: "Se colar um access token, o refresh não é necessário."},
				{Key: "customer_id", Label: "Customer ID", Type: "text", Required: true, Placeholder: "123-456-7890"},
				limitField(),
			}...),
		},
		{
			ID: "meta_ads", Label: "Meta Ads", Group: GroupAds, GroupLabel: groupLabels[GroupAds],
			Description: "Facebook Ads Insights (Graph API).", Implemented: true, Preview: false,
			Aliases: []string{"metaads", "facebook_ads"},
			Fields: append(nameField(), []Field{
				{Key: "access_token", Label: "Access token", Type: "password", Required: true},
				{Key: "ad_account_id", Label: "Ad account ID", Type: "text", Required: true, Placeholder: "act_123 ou 123", Hint: "O prefixo act_ é acrescentado se faltar."},
				{Key: "table", Label: "Recurso", Type: "select", Default: "insights", Options: []FieldOption{
					{Value: "insights", Label: "Insights"},
					{Value: "campaigns", Label: "Campanhas"},
				}},
				limitField(),
			}...),
		},

		{
			ID: "instagram", Label: "Instagram", Group: GroupSocial, GroupLabel: groupLabels[GroupSocial],
			Description: "Graph API — media e insights da conta business.", Implemented: true, Preview: false,
			Fields: append(nameField(), []Field{
				{Key: "access_token", Label: "Access token", Type: "password", Required: true},
				{Key: "instagram_business_account_id", Label: "Instagram business account ID", Type: "text", Required: true, Placeholder: "17841…"},
				{Key: "table", Label: "Recurso", Type: "select", Default: "media", Options: []FieldOption{
					{Value: "media", Label: "Media"},
					{Value: "insights", Label: "Insights"},
				}},
				{Key: "url", Label: "URL Graph (opcional)", Type: "url"},
				limitField(),
			}...),
		},
		{
			ID: "facebook", Label: "Facebook", Group: GroupSocial, GroupLabel: groupLabels[GroupSocial],
			Description: "Graph API da página — posts e insights.", Implemented: true, Preview: false,
			Aliases: []string{"fb", "facebook_page"},
			Fields: append(nameField(), []Field{
				{Key: "access_token", Label: "Access token", Type: "password", Required: true},
				{Key: "page_id", Label: "Page ID", Type: "text", Required: true},
				{Key: "table", Label: "Recurso", Type: "select", Default: "posts", Options: []FieldOption{
					{Value: "posts", Label: "Publicações"},
					{Value: "insights", Label: "Insights"},
				}},
				{Key: "url", Label: "URL Graph (opcional)", Type: "url"},
				limitField(),
			}...),
		},
		{
			ID: "google_business", Label: "Google Meu Negócio", Group: GroupSocial, GroupLabel: groupLabels[GroupSocial],
			Description: "Business Profile — locais e avaliações (Bearer).", Implemented: true, Preview: false,
			Aliases: []string{"gmb", "google_meu_negocio"},
			Fields: append(nameField(), []Field{
				{Key: "access_token", Label: "Access token", Type: "password", Required: true, Hint: "Sem token a API recusa — cole um Bearer válido."},
				{Key: "account", Label: "Account ID", Type: "text", Placeholder: "accounts/123"},
				{Key: "location_id", Label: "Location ID", Type: "text", Placeholder: "locations/456"},
				{Key: "table", Label: "Recurso", Type: "select", Default: "locations", Options: []FieldOption{
					{Value: "locations", Label: "Locais"},
					{Value: "reviews", Label: "Avaliações"},
					{Value: "accounts", Label: "Contas"},
				}},
				limitField(),
			}...),
		},
		{
			ID: "salesforce", Label: "Salesforce", Group: GroupSocial, GroupLabel: groupLabels[GroupSocial],
			Description: "SOQL REST — Account, Opportunity e Contact com Bearer.", Implemented: true, Preview: false,
			Fields: append(nameField(), []Field{
				{Key: "url", Label: "Instance URL", Type: "url", Required: true, Placeholder: "https://suaorg.my.salesforce.com"},
				{Key: "token", Label: "Access token", Type: "password", Required: true},
				{Key: "table", Label: "Objecto", Type: "select", Default: "Account", Options: []FieldOption{
					{Value: "Account", Label: "Account"},
					{Value: "Opportunity", Label: "Opportunity"},
					{Value: "Contact", Label: "Contact"},
				}},
				{Key: "query", Label: "SOQL (opcional)", Type: "textarea", Placeholder: "SELECT Id, Name FROM Account LIMIT 200"},
				limitField(),
			}...),
		},
		{
			ID: "mercado_livre", Label: "Mercado Livre", Group: GroupSocial, GroupLabel: groupLabels[GroupSocial],
			Description: "Utilizador autenticado e pesquisa de encomendas.", Implemented: true, Preview: false,
			Aliases: []string{"mercadolivre", "ml"},
			Fields: append(nameField(), []Field{
				{Key: "access_token", Label: "Access token", Type: "password", Required: true},
				{Key: "seller_id", Label: "User / seller ID (opcional)", Type: "text", Hint: "Se vazio, usa GET /users/me."},
				{Key: "table", Label: "Recurso", Type: "select", Default: "orders", Options: []FieldOption{
					{Value: "me", Label: "Utilizador (/users/me)"},
					{Value: "orders", Label: "Encomendas (/orders/search)"},
				}},
				{Key: "url", Label: "URL da API (opcional)", Type: "url", Placeholder: "https://api.mercadolibre.com"},
				limitField(),
			}...),
		},
		{
			ID: "ibge_censo", Label: "Censo IBGE", Group: GroupEcon, GroupLabel: groupLabels[GroupEcon],
			Description: "API pública SIDRA/localidades — municípios, UFs ou população estimada (agregado 6579, todos os períodos).", Implemented: true, Preview: false,
			Aliases: []string{"ibge", "censo"},
			Fields: append(nameField(), []Field{
				{Key: "table", Label: "Recurso", Type: "select", Default: "municipios", Options: []FieldOption{
					{Value: "municipios", Label: "Municípios"},
					{Value: "populacao", Label: "População estimada (SIDRA 6579)"},
					{Value: "estados", Label: "Estados"},
				}},
				{Key: "url", Label: "URL IBGE (opcional)", Type: "url"},
				limitField(),
			}...),
		},
		{
			ID: "contabilidade", Label: "Contabilidade", Group: GroupEcon, GroupLabel: groupLabels[GroupEcon],
			Description: "Upload CSV/OFX ou séries BCB SGS (estatísticas contabilísticas).", Implemented: true, Preview: false,
			Aliases: []string{"ofx"},
			Fields: append(nameField(), []Field{
				{Key: "file", Label: "Ficheiro CSV / OFX", Type: "file", Accept: ".csv,.ofx,.qfx,text/csv"},
				{Key: "series", Label: "Séries BCB SGS", Type: "text", Default: "24363,2203", Hint: "IDs separados por vírgula. Predefinido: crédito e operações de crédito."},
				{Key: "url", Label: "URL da série (opcional)", Type: "url"},
				limitField(),
			}...),
		},
		{
			ID: "inflacao", Label: "Inflação (IPCA)", Group: GroupEcon, GroupLabel: groupLabels[GroupEcon],
			Description: "IPCA via BCB SGS 433 (JSON público).", Implemented: true, Preview: false,
			Aliases: []string{"ipca"},
			Fields: append(nameField(), []Field{
				{Key: "series", Label: "Série SGS", Type: "text", Default: "433", Hint: "433 = IPCA mensal. 13522 = IPCA-15."},
				{Key: "url", Label: "URL (opcional)", Type: "url"},
				limitField(),
			}...),
		},
		{
			ID: "expectativas", Label: "Expectativa de mercado", Group: GroupEcon, GroupLabel: groupLabels[GroupEcon],
			Description: "BCB Focus (Olinda OData) — ExpectativasMercadoAnuais, mais recentes primeiro.", Implemented: true, Preview: false,
			Aliases: []string{"focus"},
			Fields: append(nameField(), []Field{
				{Key: "table", Label: "Recurso", Type: "select", Default: "anuais", Options: []FieldOption{
					{Value: "anuais", Label: "Expectativas anuais"},
					{Value: "selic", Label: "Selic"},
				}},
				{Key: "url", Label: "URL Olinda (opcional)", Type: "url"},
				limitField(),
			}...),
		},
		{
			ID: "cambio", Label: "Câmbio em tempo real", Group: GroupEcon, GroupLabel: groupLabels[GroupEcon],
			Description: "AwesomeAPI (USD/EUR) com fallback BCB SGS 1 e PTAX (coluna _fonte; _fallback se a API principal cair).", Implemented: true, Preview: false,
			Aliases: []string{"ptax", "forex"},
			Fields: append(nameField(), []Field{
				{Key: "table", Label: "Fonte", Type: "select", Default: "ultima", Options: []FieldOption{
					{Value: "ultima", Label: "Última cotação (AwesomeAPI)"},
					{Value: "serie", Label: "Série USD BCB SGS 1"},
					{Value: "ptax", Label: "PTAX (Olinda)"},
				}},
				{Key: "url", Label: "URL (opcional)", Type: "url"},
				limitField(),
			}...),
		},
		{
			ID: "google_analytics", Label: "Google Analytics", Group: GroupCloud, GroupLabel: groupLabels[GroupCloud],
			Description: "GA4 Data API (runReport) ou listagem de contas Admin.", Implemented: true, Preview: false,
			Aliases: []string{"ga", "ga4"},
			Fields: append(nameField(), []Field{
				{Key: "token", Label: "Access token ou JSON da conta de serviço", Type: "textarea", Required: true},
				{Key: "property_id", Label: "Property ID (GA4)", Type: "text", Placeholder: "properties/123456789", Hint: "Se vazio, lista contas via Admin API."},
				{Key: "client_id", Label: "Client ID (opcional)", Type: "text"},
				{Key: "client_secret", Label: "Client secret (opcional)", Type: "password"},
				{Key: "refresh_token", Label: "Refresh token (opcional)", Type: "password"},
				limitField(),
			}...),
		},
		{
			ID: "github", Label: "GitHub", Group: GroupCloud, GroupLabel: groupLabels[GroupCloud],
			Description: "Repositórios do utilizador autenticado.", Implemented: true, Preview: false,
			Fields: append(nameField(), []Field{
				{Key: "token", Label: "Personal access token", Type: "password", Required: true},
				{Key: "url", Label: "URL (opcional)", Type: "url", Placeholder: "https://api.github.com/user/repos"},
				limitField(),
			}...),
		},
		{
			ID: "stripe", Label: "Stripe", Group: GroupCloud, GroupLabel: groupLabels[GroupCloud],
			Description: "Charges (Basic auth com chave secreta sk_).", Implemented: true, Preview: false,
			Fields: append(nameField(), []Field{
				{Key: "api_key", Label: "Chave secreta", Type: "password", Required: true, Placeholder: "sk_…"},
				{Key: "table", Label: "Recurso", Type: "select", Default: "charges", Options: []FieldOption{
					{Value: "charges", Label: "Charges"},
					{Value: "customers", Label: "Customers"},
					{Value: "invoices", Label: "Invoices"},
				}},
				limitField(),
			}...),
		},

		{
			ID: "kafka", Label: "Kafka", Group: GroupStreaming, GroupLabel: groupLabels[GroupStreaming],
			Description: "Lê as últimas mensagens de um tópico.", Implemented: true, Preview: false,
			Fields: append(nameField(), []Field{
				{Key: "broker", Label: "Broker", Type: "text", Required: true, Placeholder: "localhost:9092"},
				{Key: "topic", Label: "Tópico", Type: "text", Required: true},
				{Key: "token", Label: "SASL / token (opcional)", Type: "password"},
				{Key: "user", Label: "Utilizador SASL", Type: "text"},
				{Key: "password", Label: "Senha SASL", Type: "password"},
				limitField(),
			}...),
		},
		{
			ID: "mqtt", Label: "MQTT", Group: GroupStreaming, GroupLabel: groupLabels[GroupStreaming],
			Description: "Subscreve o tópico durante 3s e ingere as mensagens.", Implemented: true, Preview: false,
			Fields: append(nameField(), []Field{
				{Key: "broker", Label: "Broker", Type: "text", Required: true, Placeholder: "tcp://localhost:1883"},
				{Key: "topic", Label: "Tópico", Type: "text", Required: true, Default: "#"},
				{Key: "user", Label: "Utilizador", Type: "text"},
				{Key: "password", Label: "Senha", Type: "password"},
				limitField(),
			}...),
		},
		{
			ID: "webhook", Label: "Webhook", Group: GroupStreaming, GroupLabel: groupLabels[GroupStreaming],
			Description: "GET ao URL configurado (últimos payloads JSON) ou ingestão REST.", Implemented: true, Preview: false,
			Fields: append(nameField(), []Field{
				{Key: "url", Label: "URL dos últimos eventos", Type: "url", Required: true, Placeholder: "https://…/events"},
				{Key: "token", Label: "Segredo / Bearer", Type: "password"},
				{Key: "headers", Label: "Cabeçalhos extra", Type: "textarea", Placeholder: "X-Custom: valor"},
			}...),
		},
	}
	for i := range items {
		items[i].Icon = brandIcon(items[i].ID)
	}
	return items
}

func ByID(id string) *Item {
	id = Canonical(id)
	for _, it := range Catalog() {
		if it.ID == id {
			item := it
			return &item
		}
	}
	return nil
}

func Implemented(typ string) bool {
	it := ByID(typ)
	return it != nil && it.Implemented
}

func Known(typ string) bool {
	return ByID(typ) != nil
}

func PreviewMessage(typ string) string {
	it := ByID(typ)
	if it != nil && it.Message != "" {
		return it.Message
	}
	label := typ
	if it != nil {
		label = it.Label
	}
	return "Falha a sincronizar " + label + ". Verifique credenciais, anfitrião e o recurso escolhido."
}

func nameField() []Field {
	return []Field{{Key: "name", Label: "Nome da ligação", Type: "text", Required: true, Placeholder: "Produção"}}
}

func limitField() Field {
	return Field{Key: "limit", Label: "Limite de linhas", Type: "number", Default: "10000"}
}

func sqlItem(id, label, port, msg string, aliases []string) Item {
	return Item{
		ID: id, Label: label, Group: GroupDatabases, GroupLabel: groupLabels[GroupDatabases],
		Description: msg, Implemented: true, Preview: false, Aliases: aliases,
		Fields: append(nameField(), []Field{
			{Key: "host", Label: "Anfitrião", Type: "text", Required: true, Default: "localhost"},
			{Key: "port", Label: "Porta", Type: "number", Default: port},
			{Key: "database", Label: "Base de dados", Type: "text", Required: true},
			{Key: "user", Label: "Utilizador", Type: "text", Required: true},
			{Key: "password", Label: "Senha", Type: "password"},
			{Key: "ssl", Label: "SSL", Type: "checkbox"},
			{Key: "ssl_mode", Label: "Modo SSL", Type: "select", Default: "disable", Options: []FieldOption{
				{Value: "disable", Label: "Desligado"},
				{Value: "require", Label: "Obrigatório"},
				{Value: "verify-full", Label: "Verificar certificado"},
			}},
			{Key: "table", Label: "Tabela (opcional)", Type: "text", Placeholder: "public.vendas", Hint: "Usada no primeiro sync se não descobrir tabelas."},
			{Key: "query", Label: "Consulta SELECT (opcional)", Type: "textarea", Placeholder: "SELECT * FROM vendas LIMIT 1000"},
			limitField(),
		}...),
	}
}

func fileItem(id, label, accept, desc string, aliases []string) Item {
	return Item{
		ID: id, Label: label, Group: GroupFiles, GroupLabel: groupLabels[GroupFiles],
		Description: desc, Implemented: true, Preview: false, Aliases: aliases,
		Fields: append(nameField(), []Field{
			{Key: "file", Label: "Ficheiro", Type: "file", Required: true, Accept: accept},
			{Key: "url", Label: "URL do ficheiro (opcional)", Type: "url", Hint: "Em alternativa ao upload, um URL público HTTP(S)."},
		}...),
	}
}

func httpFields(urlRequired bool) []Field {
	return []Field{
		{Key: "url", Label: "URL", Type: "url", Required: urlRequired, Placeholder: "https://api.exemplo.com/dados"},
		{Key: "token", Label: "Bearer token", Type: "password"},
		{Key: "api_key", Label: "Chave de API", Type: "password"},
		{Key: "headers", Label: "Cabeçalhos extra", Type: "textarea", Placeholder: "Accept: application/json\nX-Custom: valor", Hint: "Um cabeçalho por linha, no formato Nome: valor."},
		limitField(),
	}
}
