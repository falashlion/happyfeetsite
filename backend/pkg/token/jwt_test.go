package token

import (
	"testing"
	"time"

	"github.com/happyfeet/api/pkg/config"
)

func testCfg() *config.JWTConfig {
	return &config.JWTConfig{
		AccessSecret:       "test-access-secret-32-chars-long!!",
		RefreshSecret:      "test-refresh-secret-32-chars-long!",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 30 * 24 * time.Hour,
	}
}

func TestCreateAndVerifyAccessToken(t *testing.T) {
	m := NewMaker(testCfg())
	tok, claims, err := m.CreateAccessToken("user-123", "customer")
	if err != nil {
		t.Fatalf("CreateAccessToken: %v", err)
	}
	if tok == "" {
		t.Fatal("expected non-empty token")
	}
	if claims.UserID != "user-123" {
		t.Fatalf("expected user-123, got %s", claims.UserID)
	}

	verified, err := m.VerifyAccessToken(tok)
	if err != nil {
		t.Fatalf("VerifyAccessToken: %v", err)
	}
	if verified.UserID != "user-123" {
		t.Fatalf("expected user-123, got %s", verified.UserID)
	}
	if verified.Role != "customer" {
		t.Fatalf("expected customer, got %s", verified.Role)
	}
	if verified.Type != TypeAccess {
		t.Fatalf("expected access token type")
	}
}

func TestWrongTypeRejected(t *testing.T) {
	m := NewMaker(testCfg())
	// Create refresh token then try to verify as access
	tok, _, err := m.CreateRefreshToken("user-456", "vendor")
	if err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	if _, err := m.VerifyAccessToken(tok); err == nil {
		t.Fatal("expected error for wrong token type")
	}
}

func TestInvalidTokenRejected(t *testing.T) {
	m := NewMaker(testCfg())
	if _, err := m.VerifyAccessToken("not.a.jwt"); err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestExpiredToken(t *testing.T) {
	cfg := testCfg()
	cfg.AccessTokenExpiry = -1 * time.Second // already expired
	m := NewMaker(cfg)
	tok, _, _ := m.CreateAccessToken("u", "customer")
	if _, err := m.VerifyAccessToken(tok); err != ErrExpired {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
}

func TestRefreshToken(t *testing.T) {
	m := NewMaker(testCfg())
	tok, claims, err := m.CreateRefreshToken("user-789", "admin")
	if err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	if claims.Type != TypeRefresh {
		t.Fatal("expected refresh type")
	}
	verified, err := m.VerifyRefreshToken(tok)
	if err != nil {
		t.Fatalf("VerifyRefreshToken: %v", err)
	}
	if verified.UserID != "user-789" {
		t.Fatalf("wrong user ID")
	}
}
