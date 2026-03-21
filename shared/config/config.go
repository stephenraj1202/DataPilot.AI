package config

import (
	"fmt"
	"os"

	"gopkg.in/ini.v1"
)

type DatabaseConfig struct {
	Host         string
	Port         string
	Username     string
	Password     string
	DatabaseName string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type StripeConfig struct {
	APIKey        string
	WebhookSecret string
	SuccessURL    string
	CancelURL     string
}

type StripePlansConfig struct {
	BasePriceID       string
	ProPriceID        string
	EnterprisePriceID string
}

type MailConfig struct {
	DefaultSMTPHost  string
	DefaultSMTPPort  string
	DefaultFromEmail string
	SuperAdminEmail  string
	SMTPUsername     string
	SMTPPassword     string
}

type AIConfig struct {
	FastAPIURL     string
	TimeoutSeconds int
	MaxRetries     int
	OpenAIAPIKey   string
}

type EncryptionConfig struct {
	AESKey string
}

type AuthConfig struct {
	JWTSecret string
}

type Config struct {
	Database    DatabaseConfig
	Redis       RedisConfig
	Stripe      StripeConfig
	StripePlans StripePlansConfig
	Mail        MailConfig
	AI          AIConfig
	Encryption  EncryptionConfig
	Auth        AuthConfig
}

func Load(configPath string) (*Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	cfg, err := ini.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	config := &Config{}

	// Load database config
	dbSection := cfg.Section("database")
	if !dbSection.HasKey("host") || !dbSection.HasKey("port") || !dbSection.HasKey("username") ||
		!dbSection.HasKey("password") || !dbSection.HasKey("database_name") {
		return nil, fmt.Errorf("missing required database configuration values")
	}
	config.Database = DatabaseConfig{
		Host:         dbSection.Key("host").String(),
		Port:         dbSection.Key("port").String(),
		Username:     dbSection.Key("username").String(),
		Password:     dbSection.Key("password").String(),
		DatabaseName: dbSection.Key("database_name").String(),
	}

	// Load Redis config
	redisSection := cfg.Section("redis")
	config.Redis = RedisConfig{
		Host:     redisSection.Key("host").MustString("localhost"),
		Port:     redisSection.Key("port").MustString("6379"),
		Password: redisSection.Key("password").String(),
		DB:       redisSection.Key("db").MustInt(0),
	}

	// Load Stripe config
	stripeSection := cfg.Section("stripe")
	config.Stripe = StripeConfig{
		APIKey:        stripeSection.Key("api_key").String(),
		WebhookSecret: stripeSection.Key("webhook_secret").String(),
		SuccessURL:    stripeSection.Key("success_url").String(),
		CancelURL:     stripeSection.Key("cancel_url").String(),
	}

	// Load Stripe plan price IDs (optional)
	spSection := cfg.Section("stripe_plans")
	config.StripePlans = StripePlansConfig{
		BasePriceID:       spSection.Key("base_price_id").String(),
		ProPriceID:        spSection.Key("pro_price_id").String(),
		EnterprisePriceID: spSection.Key("enterprise_price_id").String(),
	}

	// Load Mail config
	mailSection := cfg.Section("mail")
	config.Mail = MailConfig{
		DefaultSMTPHost:  mailSection.Key("default_smtp_host").String(),
		DefaultSMTPPort:  mailSection.Key("default_smtp_port").String(),
		DefaultFromEmail: mailSection.Key("default_from_email").String(),
		SuperAdminEmail:  mailSection.Key("super_admin_email").String(),
		SMTPUsername:     mailSection.Key("smtp_username").String(),
		SMTPPassword:     mailSection.Key("smtp_password").String(),
	}

	// Load AI config
	aiSection := cfg.Section("ai")
	config.AI = AIConfig{
		FastAPIURL:     aiSection.Key("fastapi_url").MustString("http://localhost:8084"),
		TimeoutSeconds: aiSection.Key("timeout_seconds").MustInt(30),
		MaxRetries:     aiSection.Key("max_retries").MustInt(3),
		OpenAIAPIKey:   aiSection.Key("openai_api_key").String(),
	}

	// Load Encryption config
	encSection := cfg.Section("encryption")
	if !encSection.HasKey("aes_key") {
		return nil, fmt.Errorf("missing required encryption.aes_key configuration value")
	}
	config.Encryption = EncryptionConfig{
		AESKey: encSection.Key("aes_key").String(),
	}

	// Load Auth config
	authSection := cfg.Section("auth")
	if !authSection.HasKey("jwt_secret") {
		return nil, fmt.Errorf("missing required auth.jwt_secret configuration value")
	}
	config.Auth = AuthConfig{
		JWTSecret: authSection.Key("jwt_secret").String(),
	}

	return config, nil
}
