package config

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	Env            string
	HTTPAddr       string
	PublicURL      string
	WebOrigin      string
	JWTSecret      []byte
	EncryptionKey  []byte
	PostgresDSN    string
	RedisAddr      string
	ClickHouseAddr string
	ClickHouseDB   string
	ClickHouseUser string
	ClickHousePass string
	MinioEndpoint  string
	MinioAccess    string
	MinioSecret    string
	MinioBucket    string
	MinioSSL       bool
	KafkaBrokers   []string
	OpenAIKey      string
	OpenAIBaseURL  string
	OpenAIModel    string
	QueryTimeout   time.Duration
	QueryRowLimit  int

	GoogleClientID     string
	GoogleClientSecret string
	GitHubClientID     string
	GitHubClientSecret string
	OIDCIssuer         string
	OIDCClientID       string
	OIDCClientSecret   string

	StripeSecret          string
	StripeWebhookSecret   string
	StripePriceStarter    string
	StripePriceGrowth     string
	StripePriceBusiness   string
	StripePriceEnterprise string

	SMTPHost     string
	SMTPUser     string
	SMTPPass     string
	SMTPFrom     string
	SlackWebhook string
	AlertWebhook string
	AlertEmail   string
}

func Load() Config {
	enc := getenv("ENCRYPTION_KEY", "thedobra-dev-enc-key-32bytes-ok!")
	if len(enc) < 32 {
		enc = (enc + "thedobra-dev-enc-key-32bytes-ok!")[:32]
	}
	return Config{
		Env:                   getenv("APP_ENV", "development"),
		HTTPAddr:              getenv("APP_HTTP_ADDR", ":8080"),
		PublicURL:             getenv("APP_PUBLIC_URL", "http://localhost:8080"),
		WebOrigin:             getenv("WEB_ORIGIN", "http://localhost:3010"),
		JWTSecret:             []byte(getenv("JWT_SECRET", "thedobra-dev-jwt-secret-change-me-32b")),
		EncryptionKey:         []byte(enc[:32]),
		PostgresDSN:           getenv("POSTGRES_DSN", "postgres://thedobra:thedobra@localhost:5432/thedobra?sslmode=disable"),
		RedisAddr:             getenv("REDIS_ADDR", "localhost:6379"),
		ClickHouseAddr:        getenv("CLICKHOUSE_ADDR", "localhost:9009"),
		ClickHouseDB:          getenv("CLICKHOUSE_DATABASE", "thedobra"),
		ClickHouseUser:        getenv("CLICKHOUSE_USER", "thedobra"),
		ClickHousePass:        getenv("CLICKHOUSE_PASSWORD", "thedobra"),
		MinioEndpoint:         getenv("MINIO_ENDPOINT", "localhost:9010"),
		MinioAccess:           getenv("MINIO_ACCESS_KEY", "thedobra"),
		MinioSecret:           getenv("MINIO_SECRET_KEY", "thedobra-secret"),
		MinioBucket:           getenv("MINIO_BUCKET", "thedobra"),
		MinioSSL:              getenv("MINIO_USE_SSL", "false") == "true",
		KafkaBrokers:          split(getenv("KAFKA_BROKERS", "localhost:9092")),
		OpenAIKey:             os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:         getenv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIModel:           getenv("OPENAI_MODEL", "gpt-4o-mini"),
		QueryTimeout:          25 * time.Second,
		QueryRowLimit:         10000,
		GoogleClientID:        os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:    os.Getenv("GOOGLE_CLIENT_SECRET"),
		GitHubClientID:        os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret:    os.Getenv("GITHUB_CLIENT_SECRET"),
		OIDCIssuer:            os.Getenv("OIDC_ISSUER"),
		OIDCClientID:          os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret:      os.Getenv("OIDC_CLIENT_SECRET"),
		StripeSecret:          os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret:   os.Getenv("STRIPE_WEBHOOK_SECRET"),
		StripePriceStarter:    os.Getenv("STRIPE_PRICE_STARTER"),
		StripePriceGrowth:     os.Getenv("STRIPE_PRICE_GROWTH"),
		StripePriceBusiness:   os.Getenv("STRIPE_PRICE_BUSINESS"),
		StripePriceEnterprise: os.Getenv("STRIPE_PRICE_ENTERPRISE"),
		SMTPHost:              os.Getenv("SMTP_HOST"),
		SMTPUser:              os.Getenv("SMTP_USER"),
		SMTPPass:              os.Getenv("SMTP_PASS"),
		SMTPFrom:              getenv("SMTP_FROM", "TheDobra <noreply@thedobra.dev>"),
		SlackWebhook:          os.Getenv("SLACK_WEBHOOK_URL"),
		AlertWebhook:          os.Getenv("ALERT_WEBHOOK_URL"),
		AlertEmail:            os.Getenv("ALERT_EMAIL"),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func split(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
