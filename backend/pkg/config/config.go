package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App        AppConfig
	HTTP       HTTPConfig
	DB         DBConfig
	Redis      RedisConfig
	JWT        JWTConfig
	AWS        AWSConfig
	Cloudinary CloudinaryConfig
	Payment    PaymentConfig
	SMTP       SMTPConfig
	Mail       MailConfig
	Store      StoreConfig
	OAuth      OAuthConfig
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

// CloudinaryConfig drives signed, direct-from-browser image uploads. The API
// secret only ever signs request parameters server-side — it is never sent to
// the client. Leave CloudName blank and uploads report 503 rather than
// pretending to work.
type CloudinaryConfig struct {
	CloudName string
	APIKey    string
	APISecret string
	Folder    string
}

func (c CloudinaryConfig) Enabled() bool {
	return c.CloudName != "" && c.APIKey != "" && c.APISecret != ""
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

// SMTPConfig is one relay. Several may be configured at once — see MailConfig.
type SMTPConfig struct {
	// Name labels the relay in logs and metrics ("resend", "brevo"). Derived
	// from the host when not set explicitly.
	Name     string
	Host     string
	Port     int
	Username string
	Password string
	// From and FromName are per-relay because each provider will only accept a
	// sender on a domain IT has verified — Resend and Brevo rarely agree.
	From     string
	FromName string
	// DailyLimit and MonthlyLimit are the free-tier allowances (0 = unmetered).
	// They are advisory: the mailer uses them to spread volume and to stop
	// pushing at a relay that is spent. Exceeding one is not fatal — the relay
	// rejects the message and delivery falls through to the next provider.
	DailyLimit   int
	MonthlyLimit int
}

// MailConfig holds every configured relay, in preference order.
//
// Two free tiers side by side (Resend 3,000/month, Brevo 300/day) send far
// more than either alone, and a provider outage stops being an outage for the
// store — the next relay in the list takes the message.
type MailConfig struct {
	// Strategy is "rotate" (round-robin, spreads volume across every relay and
	// is what raises the ceiling) or "failover" (always prefer the first, use
	// the rest only when it is failing or spent).
	Strategy string
	// Providers is the ordered list, primary first. Empty means email is off.
	Providers []SMTPConfig
}

func Load() (*Config, error) {
	LoadDotenv(".")

	// The primary relay keeps the unprefixed names, so every existing
	// deployment and the local Mailpit default keep working untouched.
	primarySMTP := SMTPConfig{
		Name:         getEnv("SMTP_NAME", ""),
		Host:         getEnv("SMTP_HOST", "localhost"),
		Port:         getInt("SMTP_PORT", 1025),
		Username:     getEnv("SMTP_USERNAME", ""),
		Password:     getEnv("SMTP_PASSWORD", ""),
		From:         getEnv("SMTP_FROM", "noreply@happyfeet.com"),
		FromName:     getEnv("SMTP_FROM_NAME", "HappyFeet"),
		DailyLimit:   getInt("SMTP_DAILY_LIMIT", 0),
		MonthlyLimit: getInt("SMTP_MONTHLY_LIMIT", 0),
	}

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
		Cloudinary: CloudinaryConfig{
			CloudName: getEnv("CLOUDINARY_CLOUD_NAME", ""),
			APIKey:    getEnv("CLOUDINARY_API_KEY", ""),
			APISecret: getEnv("CLOUDINARY_API_SECRET", ""),
			Folder:    getEnv("CLOUDINARY_FOLDER", "happyfeet"),
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
		SMTP: primarySMTP,
		Mail: MailConfig{
			Strategy:  strings.ToLower(getEnv("MAIL_STRATEGY", "rotate")),
			Providers: smtpProviders(primarySMTP),
		},
		Store: StoreConfig{
			Name:            getEnv("STORE_NAME", "Happy Feet"),
			OwnerEmail:      getEnv("STORE_OWNER_EMAIL", ""),
			WhatsAppNumber:  normalizeWhatsApp(getEnv("STORE_WHATSAPP_NUMBER", "237654904707")),
			WhatsAppDisplay: getEnv("STORE_WHATSAPP_DISPLAY", "+237 6 54 90 47 07"),
			SupportEmail:    getEnv("STORE_SUPPORT_EMAIL", "contact@happyfeetcm.com"),
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

// smtpProviders collects every configured relay in preference order: the
// primary from the unprefixed SMTP_* names, then SMTP2_* and SMTP3_*.
//
// A slot with no host is skipped, so adding a second provider is one block of
// environment variables and no code change.
func smtpProviders(primary SMTPConfig) []SMTPConfig {
	providers := make([]SMTPConfig, 0, 3)
	if primary.Host != "" {
		primary.Name = smtpName(primary.Name, primary.Host)
		providers = append(providers, primary)
	}

	for _, prefix := range []string{"SMTP2", "SMTP3"} {
		host := os.Getenv(prefix + "_HOST")
		if strings.TrimSpace(host) == "" {
			continue
		}
		p := SMTPConfig{
			Name:     smtpName(os.Getenv(prefix+"_NAME"), host),
			Host:     host,
			Port:     getInt(prefix+"_PORT", 587),
			Username: getEnv(prefix+"_USERNAME", ""),
			Password: getEnv(prefix+"_PASSWORD", ""),
			// A secondary relay almost always shares the sender identity with
			// the primary, so fall back to it rather than demanding a repeat.
			From:         getEnv(prefix+"_FROM", primary.From),
			FromName:     getEnv(prefix+"_FROM_NAME", primary.FromName),
			DailyLimit:   getInt(prefix+"_DAILY_LIMIT", 0),
			MonthlyLimit: getInt(prefix+"_MONTHLY_LIMIT", 0),
		}
		providers = append(providers, p)
	}
	return providers
}

// smtpName keeps an explicit label, otherwise takes the registrable-looking
// label out of the host: smtp.resend.com → resend, smtp-relay.brevo.com → brevo.
func smtpName(explicit, host string) string {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		return explicit
	}
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return host
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
