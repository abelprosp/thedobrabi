package config

import (
	"os"
	"strings"
	"testing"
)

func TestIsProduction(t *testing.T) {
	for _, env := range []string{"production", "prod", "Production", "PROD"} {
		if !(Config{Env: env}.IsProduction()) {
			t.Fatalf("expected %q to be production", env)
		}
	}
	if (Config{Env: "development"}).IsProduction() {
		t.Fatal("development must not be production")
	}
}

func TestValidateRejectsDevSecretsInProduction(t *testing.T) {
	cfg := Config{
		Env:           "production",
		JWTSecret:     []byte(defaultJWTSecret),
		EncryptionKey: []byte(defaultEncryptionKey),
		RedisPassword: "strong-redis",
		PostgresDSN:   "postgres://u:strong@db/thedobra?sslmode=require",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected JWT default to fail")
	}

	cfg.JWTSecret = []byte("this-is-a-strong-jwt-secret-32b!!")
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected default ENCRYPTION_KEY to fail")
	}
}

func TestValidateRejectsShortPaddedEncryptionKey(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "this-is-a-strong-jwt-secret-32b!!")
	t.Setenv("ENCRYPTION_KEY", "short")
	t.Setenv("REDIS_PASSWORD", "redis-secret")
	t.Setenv("POSTGRES_DSN", "postgres://u:strong@db/x?sslmode=require")
	cfg := Load()
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected padded short ENCRYPTION_KEY to fail")
	}
	if !cfg.encKeyPadded {
		t.Fatal("expected encKeyPadded")
	}
}

func TestValidateRejectsEmptyRedisPassword(t *testing.T) {
	cfg := Config{
		Env:           "prod",
		JWTSecret:     []byte("this-is-a-strong-jwt-secret-32b!!"),
		EncryptionKey: []byte("this-is-a-strong-enc-key-32bytes!"),
		encKeyRaw:     "this-is-a-strong-enc-key-32bytes!",
		RedisPassword: "",
		PostgresDSN:   "postgres://u:strong@db/x?sslmode=require",
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "REDIS_PASSWORD") {
		t.Fatalf("expected REDIS_PASSWORD error, got %v", err)
	}
}

func TestValidateRejectsDevishAndWeakPostgres(t *testing.T) {
	base := Config{
		Env:           "production",
		JWTSecret:     []byte("this-is-a-strong-jwt-secret-32b!!"),
		EncryptionKey: []byte("this-is-a-strong-enc-key-32bytes!"),
		encKeyRaw:     "this-is-a-strong-enc-key-32bytes!",
		RedisPassword: "redis-secret",
		PostgresDSN:   "postgres://thedobra:thedobra@db.example.com/x?sslmode=require",
	}
	if err := base.Validate(); err == nil {
		t.Fatal("expected default postgres password on remote host to fail")
	}
	base.PostgresDSN = "postgres://u:strong@db.example.com/x?sslmode=disable"
	if err := base.Validate(); err == nil {
		t.Fatal("expected sslmode=disable on remote host to fail")
	}
	// Loopback Docker on the VPS is allowed (common production-on-one-box layout).
	base.PostgresDSN = "postgres://thedobra:thedobra@127.0.0.1:15432/thedobra?sslmode=disable"
	if err := base.Validate(); err != nil {
		t.Fatalf("loopback compose DSN should be allowed: %v", err)
	}
	base.PostgresDSN = "postgres://u:strong@db.example.com/x?sslmode=require"
	base.JWTSecret = []byte("production-dev-jwt-secret-change!!")
	if err := base.Validate(); err == nil {
		t.Fatal("expected JWT containing 'dev' to fail")
	}
}

func TestRequireEmailVerifiedDefaults(t *testing.T) {
	os.Unsetenv("REQUIRE_EMAIL_VERIFIED")
	t.Setenv("APP_ENV", "production")
	cfg := Load()
	if !cfg.RequireEmailVerified {
		t.Fatal("production should default RequireEmailVerified=true")
	}
	t.Setenv("APP_ENV", "development")
	cfg = Load()
	if cfg.RequireEmailVerified {
		t.Fatal("development should default RequireEmailVerified=false")
	}
	t.Setenv("REQUIRE_EMAIL_VERIFIED", "true")
	cfg = Load()
	if !cfg.RequireEmailVerified {
		t.Fatal("explicit true should win")
	}
}
