package config

import (
	"strings"
	"testing"
)

func TestLocalDeliveryRefusedInProduction(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("CONTACT_DELIVERY", "local")
	t.Setenv("APP_ENV", "production")
	if _, err := Load(); err == nil {
		t.Fatal("local delivery must be refused in production")
	}
	t.Setenv("APP_ENV", "development")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONTACT_DELIVERY", "")
	if cfg, err := Load(); err != nil || cfg.ContactDelivery != "disabled" {
		t.Fatalf("default must be disabled: %q %v", cfg.ContactDelivery, err)
	}
}

func TestSMTPConfig(t *testing.T) {
	const secret = "app-password-value"
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("APP_ENV", "production")
	t.Setenv("CONTACT_DELIVERY", "smtp")
	t.Setenv("SMTP_HOST", "smtp.gmail.com")
	t.Setenv("SMTP_USERNAME", "box@gmail.com")
	if _, err := Load(); err == nil {
		t.Fatal("a missing password must stop the start")
	}
	t.Setenv("SMTP_PASSWORD", secret)
	cfg, err := Load()
	if err != nil || cfg.SMTP.Port != "587" || cfg.SMTP.TLS != "starttls" || cfg.SMTP.From != "box@gmail.com" || cfg.EmailDailyLimit != 400 {
		t.Fatalf("defaults: %v", err)
	}
	for k, v := range map[string]string{"SMTP_TLS": "none", "SMTP_PORT": "x", "SMTP_FROM": "Name <a@b.kz>", "SMTP_HOST": "a:25"} {
		t.Run(k, func(t *testing.T) {
			t.Setenv(k, v)
			if _, err := Load(); err == nil || strings.Contains(err.Error(), secret) {
				t.Fatalf("want an error without the secret, got %v", err)
			}
		})
	}
}
