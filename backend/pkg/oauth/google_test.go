package oauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testClientID = "1234567890-abc.apps.googleusercontent.com"

// fakeGoogle stands in for Google's key endpoint: it serves the JWKS for a
// locally generated RSA key so tests can mint ID tokens that verify for real,
// signature and all.
type fakeGoogle struct {
	key    *rsa.PrivateKey
	kid    string
	server *httptest.Server
	hits   atomic.Int32
	// cacheControl is sent verbatim; empty means no header.
	cacheControl string
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeGoogle{key: key, kid: "test-key-1", cacheControl: "public, max-age=3600"}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		if f.cacheControl != "" {
			w.Header().Set("Cache-Control", f.cacheControl)
		}
		w.Header().Set("Content-Type", "application/json")
		n := base64.RawURLEncoding.EncodeToString(key.N.Bytes())
		e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())
		fmt.Fprintf(w, `{"keys":[{"kid":%q,"kty":"RSA","alg":"RS256","use":"sig","n":%q,"e":%q}]}`, f.kid, n, e)
	}))
	t.Cleanup(f.server.Close)
	return f
}

// verifier returns a GoogleVerifier pointed at the fake key endpoint.
func (f *fakeGoogle) verifier(clientIDs ...string) *GoogleVerifier {
	if len(clientIDs) == 0 {
		clientIDs = []string{testClientID}
	}
	v := NewGoogleVerifier(clientIDs...)
	v.jwksURL = f.server.URL
	return v
}

// mint signs an ID token with the given claims merged over sensible defaults.
func (f *fakeGoogle) mint(t *testing.T, override map[string]any) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss":            "https://accounts.google.com",
		"aud":            testClientID,
		"sub":            "104729382910938271234",
		"exp":            time.Now().Add(time.Hour).Unix(),
		"iat":            time.Now().Unix(),
		"email":          "amina.ngassa@gmail.com",
		"email_verified": true,
		"given_name":     "Amina",
		"family_name":    "Ngassa",
		"name":           "Amina Ngassa",
	}
	for k, val := range override {
		if val == nil {
			delete(claims, k)
			continue
		}
		claims[k] = val
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = f.kid
	signed, err := tok.SignedString(f.key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

// ── the happy path ────────────────────────────────────────────────────────────

func TestVerifyAcceptsGenuineToken(t *testing.T) {
	f := newFakeGoogle(t)
	v := f.verifier()

	claims, err := v.Verify(context.Background(), f.mint(t, nil))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Subject != "104729382910938271234" {
		t.Errorf("Subject = %q", claims.Subject)
	}
	if claims.Email != "amina.ngassa@gmail.com" {
		t.Errorf("Email = %q", claims.Email)
	}
	if !claims.EmailVerified {
		t.Error("EmailVerified should be true")
	}
	if claims.GivenName != "Amina" || claims.FamilyName != "Ngassa" {
		t.Errorf("name = %q %q", claims.GivenName, claims.FamilyName)
	}
}

func TestVerifyAcceptsBareIssuerSpelling(t *testing.T) {
	f := newFakeGoogle(t)
	if _, err := f.verifier().Verify(context.Background(),
		f.mint(t, map[string]any{"iss": "accounts.google.com"})); err != nil {
		t.Fatalf("bare issuer should be accepted: %v", err)
	}
}

func TestVerifyNormalisesEmailCase(t *testing.T) {
	f := newFakeGoogle(t)
	claims, err := f.verifier().Verify(context.Background(),
		f.mint(t, map[string]any{"email": "  Amina.Ngassa@Gmail.COM "}))
	if err != nil {
		t.Fatal(err)
	}
	// Account linking matches on email; inconsistent casing would create a
	// duplicate account for the same person.
	if claims.Email != "amina.ngassa@gmail.com" {
		t.Errorf("Email = %q, want lowercased and trimmed", claims.Email)
	}
}

func TestVerifyAcceptsStringEmailVerified(t *testing.T) {
	f := newFakeGoogle(t)
	claims, err := f.verifier().Verify(context.Background(),
		f.mint(t, map[string]any{"email_verified": "true"}))
	if err != nil {
		t.Fatalf("string email_verified should be accepted: %v", err)
	}
	if !claims.EmailVerified {
		t.Error("EmailVerified should be true")
	}
}

func TestVerifyAcceptsAudienceArray(t *testing.T) {
	f := newFakeGoogle(t)
	if _, err := f.verifier().Verify(context.Background(),
		f.mint(t, map[string]any{"aud": []string{"other-app", testClientID}})); err != nil {
		t.Fatalf("array audience containing our ID should pass: %v", err)
	}
}

// ── the rejections that matter ────────────────────────────────────────────────

// The single most important check: a token Google really did sign, but for a
// different application, must not sign anyone in here.
func TestVerifyRejectsTokenMintedForAnotherApp(t *testing.T) {
	f := newFakeGoogle(t)
	_, err := f.verifier().Verify(context.Background(),
		f.mint(t, map[string]any{"aud": "999-someone-elses-app.apps.googleusercontent.com"}))
	if !errors.Is(err, ErrWrongAudience) {
		t.Fatalf("err = %v, want ErrWrongAudience", err)
	}
}

func TestVerifyRejectsForeignIssuer(t *testing.T) {
	f := newFakeGoogle(t)
	_, err := f.verifier().Verify(context.Background(),
		f.mint(t, map[string]any{"iss": "https://evil.example.com"}))
	if !errors.Is(err, ErrWrongIssuer) {
		t.Fatalf("err = %v, want ErrWrongIssuer", err)
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	f := newFakeGoogle(t)
	_, err := f.verifier().Verify(context.Background(),
		f.mint(t, map[string]any{"exp": time.Now().Add(-2 * time.Hour).Unix()}))
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func TestVerifyRejectsTokenWithNoExpiry(t *testing.T) {
	f := newFakeGoogle(t)
	_, err := f.verifier().Verify(context.Background(), f.mint(t, map[string]any{"exp": nil}))
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("a token that never expires must be rejected; got %v", err)
	}
}

// Signed with a different key than the one the JWKS publishes.
func TestVerifyRejectsForgedSignature(t *testing.T) {
	f := newFakeGoogle(t)
	attacker, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": "https://accounts.google.com", "aud": testClientID,
		"sub": "1", "exp": time.Now().Add(time.Hour).Unix(),
		"email": "victim@gmail.com", "email_verified": true,
	})
	tok.Header["kid"] = f.kid // claims our key id, signed with theirs
	signed, err := tok.SignedString(attacker)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.verifier().Verify(context.Background(), signed); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("forged signature must be rejected; got %v", err)
	}
}

// alg:none — the classic JWT bypass.
func TestVerifyRejectsUnsignedToken(t *testing.T) {
	f := newFakeGoogle(t)
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"iss": "https://accounts.google.com", "aud": testClientID,
		"sub": "1", "exp": time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = f.kid
	signed, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.verifier().Verify(context.Background(), signed); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("alg:none must be rejected; got %v", err)
	}
}

// An HS256 token whose "secret" is the RSA public key — the algorithm-confusion
// attack that pinning valid methods prevents.
func TestVerifyRejectsAlgorithmConfusion(t *testing.T) {
	f := newFakeGoogle(t)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": "https://accounts.google.com", "aud": testClientID,
		"sub": "1", "exp": time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = f.kid
	signed, err := tok.SignedString(f.key.N.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.verifier().Verify(context.Background(), signed); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("HS256 must be rejected; got %v", err)
	}
}

// Linking on an unverified address would let anyone who can mint a Google
// identity claiming victim@example.com take over that local account.
func TestVerifyRejectsUnverifiedEmail(t *testing.T) {
	f := newFakeGoogle(t)
	_, err := f.verifier().Verify(context.Background(),
		f.mint(t, map[string]any{"email_verified": false}))
	if !errors.Is(err, ErrEmailUnverified) {
		t.Fatalf("err = %v, want ErrEmailUnverified", err)
	}
}

func TestVerifyRejectsMissingSubject(t *testing.T) {
	f := newFakeGoogle(t)
	_, err := f.verifier().Verify(context.Background(), f.mint(t, map[string]any{"sub": nil}))
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func TestVerifyRejectsUnknownKeyID(t *testing.T) {
	f := newFakeGoogle(t)
	v := f.verifier()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": "https://accounts.google.com", "aud": testClientID,
		"sub": "1", "exp": time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = "a-key-google-never-published"
	signed, _ := tok.SignedString(f.key)
	if _, err := v.Verify(context.Background(), signed); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func TestVerifyRejectsGarbage(t *testing.T) {
	f := newFakeGoogle(t)
	for _, bad := range []string{"", "not-a-jwt", "a.b.c", strings.Repeat("x", 500)} {
		if _, err := f.verifier().Verify(context.Background(), bad); err == nil {
			t.Errorf("Verify(%q) returned no error", truncate(bad))
		}
	}
}

// ── configuration ─────────────────────────────────────────────────────────────

// An unconfigured verifier must fail closed, never open.
func TestUnconfiguredVerifierRefusesEverything(t *testing.T) {
	f := newFakeGoogle(t)
	v := NewGoogleVerifier("")
	v.jwksURL = f.server.URL

	if v.Enabled() {
		t.Fatal("Enabled() should be false with no client ID")
	}
	if _, err := v.Verify(context.Background(), f.mint(t, nil)); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}

func TestMultipleClientIDsAreAccepted(t *testing.T) {
	f := newFakeGoogle(t)
	v := f.verifier("android-client.apps.googleusercontent.com", testClientID)
	if _, err := v.Verify(context.Background(), f.mint(t, nil)); err != nil {
		t.Fatalf("second configured audience should be accepted: %v", err)
	}
	if got := v.ClientID(); got != "android-client.apps.googleusercontent.com" {
		t.Errorf("ClientID() = %q, want the first entry", got)
	}
}

// ── key caching ───────────────────────────────────────────────────────────────

func TestKeysAreCachedAcrossVerifications(t *testing.T) {
	f := newFakeGoogle(t)
	v := f.verifier()
	for i := 0; i < 5; i++ {
		if _, err := v.Verify(context.Background(), f.mint(t, nil)); err != nil {
			t.Fatal(err)
		}
	}
	if hits := f.hits.Load(); hits != 1 {
		t.Errorf("fetched the key set %d times, want 1 — sign-in should not "+
			"call Google on every login", hits)
	}
}

// A stream of tokens bearing bogus key ids must not turn into a stream of
// requests to Google.
func TestUnknownKeyIDRefetchIsRateLimited(t *testing.T) {
	f := newFakeGoogle(t)
	v := f.verifier()
	if _, err := v.Verify(context.Background(), f.mint(t, nil)); err != nil {
		t.Fatal(err)
	}

	bogus := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": "https://accounts.google.com", "aud": testClientID,
		"sub": "1", "exp": time.Now().Add(time.Hour).Unix(),
	})
	bogus.Header["kid"] = "unknown"
	signed, _ := bogus.SignedString(f.key)
	for i := 0; i < 20; i++ {
		_, _ = v.Verify(context.Background(), signed)
	}
	if hits := f.hits.Load(); hits > 2 {
		t.Errorf("key endpoint hit %d times for 20 bogus tokens; rate limit not working", hits)
	}
}

func TestCacheTTLClamping(t *testing.T) {
	for _, tc := range []struct {
		header string
		want   time.Duration
	}{
		{"public, max-age=3600", time.Hour},
		{"max-age=60", minKeyTTL},          // below the floor
		{"max-age=999999", maxKeyTTL},      // above the ceiling
		{"", minKeyTTL},                    // absent
		{"no-store", minKeyTTL},            // present but unusable
		{"max-age=notanumber", minKeyTTL},  // malformed
	} {
		if got := cacheTTL(tc.header); got != tc.want {
			t.Errorf("cacheTTL(%q) = %v, want %v", tc.header, got, tc.want)
		}
	}
}

func TestConcurrentVerifyIsRaceFree(t *testing.T) {
	f := newFakeGoogle(t)
	v := f.verifier()
	tok := f.mint(t, nil)

	done := make(chan error, 16)
	for i := 0; i < 16; i++ {
		go func() {
			_, err := v.Verify(context.Background(), tok)
			done <- err
		}()
	}
	for i := 0; i < 16; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent Verify: %v", err)
		}
	}
}

func truncate(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}
