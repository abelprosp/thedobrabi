package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultJWTSecret     = "thedobra-dev-jwt-secret-change-me-32b"
	defaultEncryptionKey = "thedobra-dev-enc-key-32bytes-ok!"
	encPadSuffix         = "thedobra-dev-enc-key-32bytes-ok!"
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
	RedisPassword  string
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

	SMTPHost        string
	SMTPPort        int
	SMTPUser        string
	SMTPPass        string
	SMTPFrom        string
	SlackWebhook    string
	AlertWebhook    string
	AlertEmail      string
	WhatsAppWebhook string

	// RequireEmailVerified rejects password login when email_verified_at is null.
	// Defaults to true when APP_ENV is production/prod; override with REQUIRE_EMAIL_VERIFIED.
	RequireEmailVerified bool

	// encKeyPadded is true when ENCRYPTION_KEY was shorter than 32 bytes and padded.
	encKeyPadded bool
	// encKeyRaw is the original ENCRYPTION_KEY env value (before padding).
	encKeyRaw string
}

func Load() Config {
	encRaw := getenv("ENCRYPTION_KEY", defaultEncryptionKey)
	encPadded := false
	enc := encRaw
	if len(enc) < 32 {
		encPadded = true
		enc = (enc + encPadSuffix)[:32]
	}

	cfg := Config{
		Env:                   getenv("APP_ENV", "development"),
		HTTPAddr:              getenv("APP_HTTP_ADDR", ":8080"),
		PublicURL:             getenv("APP_PUBLIC_URL", "http://localhost:8080"),
		WebOrigin:             getenv("WEB_ORIGIN", "http://localhost:3010"),
		JWTSecret:             []byte(getenv("JWT_SECRET", defaultJWTSecret)),
		EncryptionKey:         []byte(enc[:32]),
		encKeyPadded:          encPadded,
		encKeyRaw:             encRaw,
		PostgresDSN:           getenv("POSTGRES_DSN", "postgres://thedobra:thedobra@localhost:5432/thedobra?sslmode=disable"),
		RedisAddr:             getenv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:         os.Getenv("REDIS_PASSWORD"),
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
		SMTPPort:              smtpPort(),
		SMTPUser:              os.Getenv("SMTP_USER"),
		SMTPPass:              os.Getenv("SMTP_PASS"),
		SMTPFrom:              getenv("SMTP_FROM", "TheDobra <noreply@thedobra.dev>"),
		SlackWebhook:          os.Getenv("SLACK_WEBHOOK_URL"),
		AlertWebhook:          os.Getenv("ALERT_WEBHOOK_URL"),
		AlertEmail:            os.Getenv("ALERT_EMAIL"),
		WhatsAppWebhook:       os.Getenv("WHATSAPP_WEBHOOK_URL"),
	}
	cfg.RequireEmailVerified = parseBoolEnv("REQUIRE_EMAIL_VERIFIED", cfg.IsProduction())
	return cfg
}

func smtpPort() int {
	port, err := strconv.Atoi(getenv("SMTP_PORT", "587"))
	if err != nil || port < 1 || port > 65535 {
		return 587
	}
	return port
}

// IsProduction reports whether APP_ENV is production or prod.
func (c Config) IsProduction() bool {
	env := strings.ToLower(strings.TrimSpace(c.Env))
	return env == "production" || env == "prod"
}

// Validate prevents the API from starting with credentials that are safe only
// for local development.
func (c Config) Validate() error {
	if !c.IsProduction() {
		return nil
	}

	jwt := string(c.JWTSecret)
	if jwt == defaultJWTSecret || len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be a strong production secret")
	}
	if strings.Contains(strings.ToLower(jwt), "dev") {
		return fmt.Errorf("JWT_SECRET must not contain 'dev' in production")
	}

	enc := string(c.EncryptionKey)
	if enc == defaultEncryptionKey {
		return fmt.Errorf("ENCRYPTION_KEY must be a strong production secret")
	}
	if c.encKeyPadded || len(c.encKeyRaw) < 32 {
		return fmt.Errorf("ENCRYPTION_KEY must be at least 32 bytes (short keys are not padded in production)")
	}
	if strings.HasSuffix(enc, encPadSuffix) || strings.Contains(enc, encPadSuffix) {
		return fmt.Errorf("ENCRYPTION_KEY looks padded from a short/dev key")
	}
	if strings.Contains(strings.ToLower(c.encKeyRaw), "dev") || strings.Contains(strings.ToLower(enc), "dev") {
		return fmt.Errorf("ENCRYPTION_KEY must not contain 'dev' in production")
	}

	if strings.TrimSpace(c.RedisPassword) == "" {
		return fmt.Errorf("REDIS_PASSWORD is required in production")
	}

	// Local Docker on the same VPS commonly uses loopback without TLS and the
	// compose defaults. Reject those only when the DSN points off-box.
	if !dsnIsLoopback(c.PostgresDSN) {
		dsn := strings.ToLower(c.PostgresDSN)
		if strings.Contains(dsn, "password=thedobra") || strings.Contains(dsn, ":thedobra@") {
			return fmt.Errorf("POSTGRES_DSN must not use the default thedobra password for a remote database")
		}
		if strings.Contains(dsn, "sslmode=disable") {
			return fmt.Errorf("POSTGRES_DSN must not use sslmode=disable for a remote database")
		}
	}

	if c.StripeSecret != "" && c.StripeWebhookSecret == "" {
		return fmt.Errorf("STRIPE_WEBHOOK_SECRET is required when Stripe is enabled")
	}
	return nil
}

func dsnIsLoopback(dsn string) bool {
	lower := strings.ToLower(dsn)
	for _, h := range []string{"@127.0.0.1", "@localhost", "@[::1]"} {
		if strings.Contains(lower, h) {
			return true
		}
	}
	return false
}

func parseBoolEnv(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
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
