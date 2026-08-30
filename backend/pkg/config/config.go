package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App     AppConfig
	HTTP    HTTPConfig
	DB      DBConfig
	Redis   RedisConfig
	JWT     JWTConfig
	AWS     AWSConfig
	Payment PaymentConfig
	SMTP    SMTPConfig
	Store   StoreConfig
	OAuth   OAuthConfig
}

// OAuthConfig holds third-party sign-in settings.
type OAuthConfig struct {
	// GoogleClientIDs are the audiences accepted on a Google ID token. The
	// first is the one handed to the storefront. Several may be listed
	// (comma-separated) when web and mobile clients share one backend.
	GoogleClientIDs []string
}

// StoreConfig holds the merchant-facing identity used by transactional email
// and the WhatsApp contact affordances rendered into those emails.
type StoreConfig struct {
	Name string
	// OwnerEmail receives a "new order" alert for every order placed.
	OwnerEmail string
	// WhatsAppNumber is in international format without "+" or spaces, as
	// required by wa.me deep links (e.g. "237612345678").
	WhatsAppNumber string
	// WhatsAppDisplay is the human-readable rendering (e.g. "+237 6 12 34 56 78").
	WhatsAppDisplay string
	SupportEmail    string
	Address         string
	Hours           string
}

type AppConfig struct {
	Name        string
	Env         string
	Version     string
	Debug       bool
	FrontendURL string
	CDNURL      string
}

type HTTPConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type DBConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type RedisConfig struct {
	URL string
}

type JWTConfig struct {
	AccessSecret       string
	RefreshSecret      string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
}

type AWSConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	S3Bucket        string
	S3Endpoint      string // for MinIO local dev
	CloudFrontURL   string
	PresignExpiry   time.Duration
}

type PaymentConfig struct {
	MTNBaseURL          string
	MTNSubscriptionKey  string
	MTNAPIUserID        string
	MTNAPIKey           string
	MTNEnvironment      string
	MTNWebhookSecret    string
	OrangeBaseURL       string
	OrangeClientID      string
	OrangeClientSecret  string
	OrangeCountryCode   string
	OrangeMerchantKey   string
	OrangeWebhookSecret string
	OrangeReturnURL     string
	OrangeCancelURL     string
	StripeSecretKey     string
	StripeWebhookSecret string
}

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
}

func Load() (*Config, error) {
	LoadDotenv(".")
	return &Config{
		App: AppConfig{
			Name:        getEnv("APP_NAME", "happyfeet-api"),
			Env:         getEnv("APP_ENV", "development"),
			Version:     getEnv("APP_VERSION", "1.0.0"),
			Debug:       getBool("APP_DEBUG", true),
			FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
			CDNURL:      getEnv("CDN_URL", "http://localhost:9000/happyfeet-media-dev"),
		},
		HTTP: HTTPConfig{
			Host:         getEnv("HTTP_HOST", "0.0.0.0"),
			Port:         getInt("HTTP_PORT", 8080),
			ReadTimeout:  getDur("HTTP_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getDur("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:  getDur("HTTP_IDLE_TIMEOUT", 120*time.Second),
		},
		DB: DBConfig{
			URL:             getEnv("DATABASE_URL", "postgres://happyfeet:happyfeet_dev_secret@localhost:5432/happyfeet?sslmode=disable"),
			MaxConns:        int32(getInt("DB_MAX_CONNS", 25)),
			MinConns:        int32(getInt("DB_MIN_CONNS", 5)),
			ConnMaxLifetime: getDur("DB_CONN_MAX_LIFETIME", 15*time.Minute),
			ConnMaxIdleTime: getDur("DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", "redis://localhost:6379/0"),
		},
		JWT: JWTConfig{
			AccessSecret:       getEnv("JWT_ACCESS_SECRET", "dev-access-secret-CHANGE-in-prod-32c"),
			RefreshSecret:      getEnv("JWT_REFRESH_SECRET", "dev-refresh-secret-CHANGE-in-prod-32c"),
			AccessTokenExpiry:  getDur("JWT_ACCESS_EXPIRY", 15*time.Minute),
			RefreshTokenExpiry: getDur("JWT_REFRESH_EXPIRY", 30*24*time.Hour),
		},
		AWS: AWSConfig{
			Region:          getEnv("AWS_REGION", "us-east-1"),
			AccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", "minioadmin"),
			SecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", "minioadmin123"),
			S3Bucket:        getEnv("S3_BUCKET", "happyfeet-media-dev"),
			S3Endpoint:      getEnv("S3_ENDPOINT", "http://localhost:9000"),
			CloudFrontURL:   getEnv("CLOUDFRONT_URL", "http://localhost:9000/happyfeet-media-dev"),
			PresignExpiry:   getDur("S3_PRESIGN_EXPIRY", 15*time.Minute),
		},
		Payment: PaymentConfig{
			MTNBaseURL:          getEnv("MTN_BASE_URL", "https://sandbox.momodeveloper.mtn.com"),
			MTNSubscriptionKey:  getEnv("MTN_SUBSCRIPTION_KEY", ""),
			MTNAPIUserID:        getEnv("MTN_API_USER_ID", ""),
			MTNAPIKey:           getEnv("MTN_API_KEY", ""),
			MTNEnvironment:      getEnv("MTN_ENVIRONMENT", "sandbox"),
			MTNWebhookSecret:    getEnv("MTN_WEBHOOK_SECRET", ""),
			OrangeBaseURL:       getEnv("ORANGE_BASE_URL", "https://api.orange.com"),
			OrangeClientID:      getEnv("ORANGE_CLIENT_ID", ""),
			OrangeClientSecret:  getEnv("ORANGE_CLIENT_SECRET", ""),
			OrangeCountryCode:   getEnv("ORANGE_COUNTRY_CODE", "CM"),
			OrangeMerchantKey:   getEnv("ORANGE_MERCHANT_KEY", ""),
			OrangeWebhookSecret: getEnv("ORANGE_WEBHOOK_SECRET", ""),
			OrangeReturnURL:     getEnv("ORANGE_RETURN_URL", "http://localhost:3000/checkout/complete"),
			OrangeCancelURL:     getEnv("ORANGE_CANCEL_URL", "http://localhost:3000/checkout/cancelled"),
			StripeSecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
			StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		},
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", "localhost"),
			Port:     getInt("SMTP_PORT", 1025),
			Username: getEnv("SMTP_USERNAME", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", "noreply@happyfeet.com"),
			FromName: getEnv("SMTP_FROM_NAME", "HappyFeet"),
		},
		Store: StoreConfig{
			Name:            getEnv("STORE_NAME", "Happy Feet"),
			OwnerEmail:      getEnv("STORE_OWNER_EMAIL", ""),
			WhatsAppNumber:  normalizeWhatsApp(getEnv("STORE_WHATSAPP_NUMBER", "237612345678")),
			WhatsAppDisplay: getEnv("STORE_WHATSAPP_DISPLAY", "+237 6 12 34 56 78"),
			SupportEmail:    getEnv("STORE_SUPPORT_EMAIL", "care@happyfeet.com"),
			Address:         getEnv("STORE_ADDRESS", "Bonapriso, Douala, Cameroon"),
			Hours:           getEnv("STORE_HOURS", "Mon–Sat, 08:00–20:00 WAT"),
		},
		OAuth: OAuthConfig{
			GoogleClientIDs: splitList(getEnv("GOOGLE_CLIENT_ID", "")),
		},
	}, nil
}

// splitList parses a comma-separated env value, dropping blanks.
func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// normalizeWhatsApp strips everything a wa.me link cannot carry: the leading
// "+", spaces, dashes and parentheses.
func normalizeWhatsApp(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (c *Config) Addr() string { return fmt.Sprintf("%s:%d", c.HTTP.Host, c.HTTP.Port) }
func (c *Config) IsProd() bool { return c.App.Env == "production" }

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func getInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return d
}
func getBool(k string, d bool) bool {
	if v := os.Getenv(k); v != "" {
		return v == "true" || v == "1"
	}
	return d
}
func getDur(k string, d time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if dur, err := time.ParseDuration(v); err == nil {
			return dur
		}
	}
	return d
}
