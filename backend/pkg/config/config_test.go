package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.App.Name != "happyfeet-api" {
		t.Fatalf("expected happyfeet-api, got %s", cfg.App.Name)
	}
	if cfg.HTTP.Port != 8080 {
		t.Fatalf("expected 8080, got %d", cfg.HTTP.Port)
	}
	if cfg.JWT.AccessTokenExpiry != 15*time.Minute {
		t.Fatalf("expected 15m, got %v", cfg.JWT.AccessTokenExpiry)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	os.Setenv("HTTP_PORT", "9090")
	os.Setenv("APP_ENV", "production")
	defer func() {
		os.Unsetenv("HTTP_PORT")
		os.Unsetenv("APP_ENV")
	}()
	cfg, _ := Load()
	if cfg.HTTP.Port != 9090 {
		t.Fatalf("expected 9090, got %d", cfg.HTTP.Port)
	}
	if !cfg.IsProd() {
		t.Fatal("expected production env")
	}
}

func TestAddr(t *testing.T) {
	cfg, _ := Load()
	addr := cfg.Addr()
	if addr == "" {
		t.Fatal("expected non-empty addr")
	}
}
