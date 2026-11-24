// Package config loads application settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/qurylys/qurylys/backend/internal/delivery"
)

const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

type Config struct {
	AppEnv          string
	HTTPAddr        string
	ShutdownTimeout time.Duration

	DatabaseURL       string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	DBConnMaxIdleTime time.Duration

	// CORSAllowedOrigins is an explicit allowlist. Empty means CORS is disabled,
	// which is the expected mode behind the shared Nginx (same origin).
	CORSAllowedOrigins []string

	// ContactDelivery: "disabled" (default), "smtp", or "local" (test outbox file, never in production).
	ContactDelivery string
	ContactLocalDir string
	SMTP            delivery.SMTPConfig
	// EmailDailyLimit caps code requests per day across all visitors (0 = no cap),
	// so abuse cannot get the mailbox suspended.
	EmailDailyLimit int
}

func (c Config) IsProduction() bool { return c.AppEnv == EnvProduction }

// Load reads configuration from the environment. DATABASE_URL is required;
// everything else has a safe default for local development.
func Load() (Config, error) {
	var errs []error

	cfg := Config{
		AppEnv:      getString("APP_ENV", EnvDevelopment),
		HTTPAddr:    getString("HTTP_ADDR", ":8081"),
		DatabaseURL: strings.TrimSpace(os.Getenv("DATABASE_URL")),
	}

	switch cfg.AppEnv {
	case EnvDevelopment, EnvTest, EnvProduction:
	default:
		errs = append(errs, fmt.Errorf("APP_ENV must be one of development, test, production; got %q", cfg.AppEnv))
	}
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}

	cfg.ShutdownTimeout = getDuration("SHUTDOWN_TIMEOUT", 15*time.Second, &errs)
	cfg.DBMaxOpenConns = getInt("DB_MAX_OPEN_CONNS", 20, &errs)
	cfg.DBMaxIdleConns = getInt("DB_MAX_IDLE_CONNS", 10, &errs)
	cfg.DBConnMaxLifetime = getDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute, &errs)
	cfg.DBConnMaxIdleTime = getDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute, &errs)
	cfg.CORSAllowedOrigins = getList("CORS_ALLOWED_ORIGINS")

	cfg.ContactDelivery = getString("CONTACT_DELIVERY", "disabled")
	cfg.ContactLocalDir = getString("CONTACT_LOCAL_DIR", filepath.Join(os.TempDir(), "qurylys-outbox"))
	switch cfg.ContactDelivery {
	case delivery.ModeDisabled:
	case delivery.ModeSMTP:
		cfg.SMTP = delivery.SMTPConfig{
			Host:     getString("SMTP_HOST", ""),
			Port:     getString("SMTP_PORT", "587"),
			TLS:      getString("SMTP_TLS", delivery.TLSStartTLS),
			Username: getString("SMTP_USERNAME", ""),
			Password: getString("SMTP_PASSWORD", ""),
			FromName: getString("SMTP_FROM_NAME", "QURYLYS B2B"),
			ReplyTo:  getString("SMTP_REPLY_TO", ""),
		}
		cfg.SMTP.From = getString("SMTP_FROM", cfg.SMTP.Username)
		cfg.EmailDailyLimit = getInt("EMAIL_DAILY_LIMIT", 400, &errs)
		if err := cfg.SMTP.Validate(); err != nil {
			errs = append(errs, err)
		}
	case delivery.ModeLocal:
		if cfg.IsProduction() {
			errs = append(errs, errors.New("CONTACT_DELIVERY=local is not allowed when APP_ENV=production"))
		}
	default:
		errs = append(errs, fmt.Errorf("CONTACT_DELIVERY must be disabled, smtp or local, got %q", cfg.ContactDelivery))
	}

	for _, o := range cfg.CORSAllowedOrigins {
		if o == "*" {
			errs = append(errs, errors.New("CORS_ALLOWED_ORIGINS must list explicit origins, '*' is not allowed"))
		}
	}

	return cfg, errors.Join(errs...)
}

func getString(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int, errs *[]error) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		*errs = append(*errs, fmt.Errorf("%s must be a non-negative integer, got %q", key, v))
		return def
	}
	return n
}

func getDuration(key string, def time.Duration, errs *[]error) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		*errs = append(*errs, fmt.Errorf("%s must be a duration like 15s, got %q", key, v))
		return def
	}
	return d
}

func getList(key string) []string {
	var out []string
	for _, part := range strings.Split(os.Getenv(key), ",") {
		if p := strings.TrimRight(strings.TrimSpace(part), "/"); p != "" {
			out = append(out, p)
		}
	}
	return out
}
