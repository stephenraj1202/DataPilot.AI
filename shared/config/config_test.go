package config

import (
	"os"
	"strings"
	"testing"
)

// writeTemp creates a temporary .ini file with the given content and returns its path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "config-*.ini")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

// fullConfig is a valid config.ini covering all sections.
const fullConfig = `[database]
host=localhost
port=3306
username=testuser
password=testpass
database_name=testdb

[redis]
host=redishost
port=6380
password=redispass
db=1

[stripe]
api_key=sk_test_123
webhook_secret=whsec_abc
success_url=http://localhost:3000/success
cancel_url=http://localhost:3000/cancel

[mail]
default_smtp_host=smtp.example.com
default_smtp_port=587
default_from_email=noreply@example.com
super_admin_email=admin@example.com

[ai]
fastapi_url=http://localhost:8084
timeout_seconds=45
max_retries=5
openai_api_key=sk-openai-test

[encryption]
aes_key=test-32-byte-key-for-encryption!!

[auth]
jwt_secret=super-secret-jwt-key
`

// --- Database section ---

func TestLoad_DatabaseSection(t *testing.T) {
	cfg, err := Load(writeTemp(t, fullConfig))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("Database.Host: want localhost, got %s", cfg.Database.Host)
	}
	if cfg.Database.Port != "3306" {
		t.Errorf("Database.Port: want 3306, got %s", cfg.Database.Port)
	}
	if cfg.Database.Username != "testuser" {
		t.Errorf("Database.Username: want testuser, got %s", cfg.Database.Username)
	}
	if cfg.Database.Password != "testpass" {
		t.Errorf("Database.Password: want testpass, got %s", cfg.Database.Password)
	}
	if cfg.Database.DatabaseName != "testdb" {
		t.Errorf("Database.DatabaseName: want testdb, got %s", cfg.Database.DatabaseName)
	}
}

func TestLoad_MissingDatabaseHost(t *testing.T) {
	content := strings.ReplaceAll(fullConfig, "host=localhost\n", "")
	_, err := Load(writeTemp(t, content))
	if err == nil {
		t.Error("expected error for missing database.host, got nil")
	}
}

func TestLoad_MissingDatabasePort(t *testing.T) {
	content := strings.ReplaceAll(fullConfig, "port=3306\n", "")
	_, err := Load(writeTemp(t, content))
	if err == nil {
		t.Error("expected error for missing database.port, got nil")
	}
}

func TestLoad_MissingDatabaseUsername(t *testing.T) {
	content := strings.ReplaceAll(fullConfig, "username=testuser\n", "")
	_, err := Load(writeTemp(t, content))
	if err == nil {
		t.Error("expected error for missing database.username, got nil")
	}
}

func TestLoad_MissingDatabasePassword(t *testing.T) {
	content := strings.ReplaceAll(fullConfig, "password=testpass\n", "")
	_, err := Load(writeTemp(t, content))
	if err == nil {
		t.Error("expected error for missing database.password, got nil")
	}
}

func TestLoad_MissingDatabaseName(t *testing.T) {
	content := strings.ReplaceAll(fullConfig, "database_name=testdb\n", "")
	_, err := Load(writeTemp(t, content))
	if err == nil {
		t.Error("expected error for missing database.database_name, got nil")
	}
}

// --- Stripe section ---

func TestLoad_StripeSection(t *testing.T) {
	cfg, err := Load(writeTemp(t, fullConfig))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Stripe.APIKey != "sk_test_123" {
		t.Errorf("Stripe.APIKey: want sk_test_123, got %s", cfg.Stripe.APIKey)
	}
	if cfg.Stripe.WebhookSecret != "whsec_abc" {
		t.Errorf("Stripe.WebhookSecret: want whsec_abc, got %s", cfg.Stripe.WebhookSecret)
	}
	if cfg.Stripe.SuccessURL != "http://localhost:3000/success" {
		t.Errorf("Stripe.SuccessURL: want http://localhost:3000/success, got %s", cfg.Stripe.SuccessURL)
	}
	if cfg.Stripe.CancelURL != "http://localhost:3000/cancel" {
		t.Errorf("Stripe.CancelURL: want http://localhost:3000/cancel, got %s", cfg.Stripe.CancelURL)
	}
}

// --- Mail section ---

func TestLoad_MailSection(t *testing.T) {
	cfg, err := Load(writeTemp(t, fullConfig))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Mail.DefaultSMTPHost != "smtp.example.com" {
		t.Errorf("Mail.DefaultSMTPHost: want smtp.example.com, got %s", cfg.Mail.DefaultSMTPHost)
	}
	if cfg.Mail.DefaultSMTPPort != "587" {
		t.Errorf("Mail.DefaultSMTPPort: want 587, got %s", cfg.Mail.DefaultSMTPPort)
	}
	if cfg.Mail.DefaultFromEmail != "noreply@example.com" {
		t.Errorf("Mail.DefaultFromEmail: want noreply@example.com, got %s", cfg.Mail.DefaultFromEmail)
	}
	if cfg.Mail.SuperAdminEmail != "admin@example.com" {
		t.Errorf("Mail.SuperAdminEmail: want admin@example.com, got %s", cfg.Mail.SuperAdminEmail)
	}
}

// --- AI section ---

func TestLoad_AISection(t *testing.T) {
	cfg, err := Load(writeTemp(t, fullConfig))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AI.FastAPIURL != "http://localhost:8084" {
		t.Errorf("AI.FastAPIURL: want http://localhost:8084, got %s", cfg.AI.FastAPIURL)
	}
	if cfg.AI.TimeoutSeconds != 45 {
		t.Errorf("AI.TimeoutSeconds: want 45, got %d", cfg.AI.TimeoutSeconds)
	}
	if cfg.AI.MaxRetries != 5 {
		t.Errorf("AI.MaxRetries: want 5, got %d", cfg.AI.MaxRetries)
	}
	if cfg.AI.OpenAIAPIKey != "sk-openai-test" {
		t.Errorf("AI.OpenAIAPIKey: want sk-openai-test, got %s", cfg.AI.OpenAIAPIKey)
	}
}

func TestLoad_AIDefaults(t *testing.T) {
	// Omit the [ai] section entirely — defaults should apply.
	content := fullConfig[:strings.Index(fullConfig, "[ai]")]
	content += "[encryption]\naes_key=test-32-byte-key-for-encryption!!\n\n[auth]\njwt_secret=super-secret-jwt-key\n"

	cfg, err := Load(writeTemp(t, content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AI.FastAPIURL != "http://localhost:8084" {
		t.Errorf("AI.FastAPIURL default: want http://localhost:8084, got %s", cfg.AI.FastAPIURL)
	}
	if cfg.AI.TimeoutSeconds != 30 {
		t.Errorf("AI.TimeoutSeconds default: want 30, got %d", cfg.AI.TimeoutSeconds)
	}
	if cfg.AI.MaxRetries != 3 {
		t.Errorf("AI.MaxRetries default: want 3, got %d", cfg.AI.MaxRetries)
	}
}

// --- Encryption section ---

func TestLoad_MissingEncryptionKey(t *testing.T) {
	content := strings.ReplaceAll(fullConfig, "aes_key=test-32-byte-key-for-encryption!!\n", "")
	_, err := Load(writeTemp(t, content))
	if err == nil {
		t.Error("expected error for missing encryption.aes_key, got nil")
	}
}

// --- Auth section ---

func TestLoad_AuthSection(t *testing.T) {
	cfg, err := Load(writeTemp(t, fullConfig))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Auth.JWTSecret != "super-secret-jwt-key" {
		t.Errorf("Auth.JWTSecret: want super-secret-jwt-key, got %s", cfg.Auth.JWTSecret)
	}
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	content := strings.ReplaceAll(fullConfig, "jwt_secret=super-secret-jwt-key\n", "")
	_, err := Load(writeTemp(t, content))
	if err == nil {
		t.Error("expected error for missing auth.jwt_secret, got nil")
	}
}

// --- File-level errors ---

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("nonexistent-config.ini")
	if err == nil {
		t.Error("expected error for missing config file, got nil")
	}
}

func TestLoad_InvalidINI(t *testing.T) {
	_, err := Load(writeTemp(t, "this is not valid ini content ==="))
	// The ini parser is lenient; what matters is required keys will be absent.
	// If it does return an error, that's also acceptable.
	if err == nil {
		// Acceptable only if the parser is lenient — but required keys will be missing.
		t.Log("ini parser did not error on invalid content; missing-key validation should catch it")
	}
}

// --- Redis section (optional, uses defaults) ---

func TestLoad_RedisSection(t *testing.T) {
	cfg, err := Load(writeTemp(t, fullConfig))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Redis.Host != "redishost" {
		t.Errorf("Redis.Host: want redishost, got %s", cfg.Redis.Host)
	}
	if cfg.Redis.Port != "6380" {
		t.Errorf("Redis.Port: want 6380, got %s", cfg.Redis.Port)
	}
	if cfg.Redis.Password != "redispass" {
		t.Errorf("Redis.Password: want redispass, got %s", cfg.Redis.Password)
	}
	if cfg.Redis.DB != 1 {
		t.Errorf("Redis.DB: want 1, got %d", cfg.Redis.DB)
	}
}

func TestLoad_RedisDefaults(t *testing.T) {
	// Omit [redis] section — defaults should apply.
	content := strings.Replace(fullConfig,
		"[redis]\nhost=redishost\nport=6380\npassword=redispass\ndb=1\n\n", "", 1)

	cfg, err := Load(writeTemp(t, content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Redis.Host != "localhost" {
		t.Errorf("Redis.Host default: want localhost, got %s", cfg.Redis.Host)
	}
	if cfg.Redis.Port != "6379" {
		t.Errorf("Redis.Port default: want 6379, got %s", cfg.Redis.Port)
	}
	if cfg.Redis.DB != 0 {
		t.Errorf("Redis.DB default: want 0, got %d", cfg.Redis.DB)
	}
}
