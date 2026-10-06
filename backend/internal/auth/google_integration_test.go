package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/happyfeet/api/pkg/cache"
	"github.com/happyfeet/api/pkg/config"
	"github.com/happyfeet/api/pkg/database"
	"github.com/happyfeet/api/pkg/oauth"
	"github.com/happyfeet/api/pkg/token"
)

// End-to-end Google sign-in against the real Postgres and Redis from
// compose.local.yml: a token signed by a stand-in Google goes in, and a real row
// in `users` plus a real session comes out. Everything except Google itself is
// the production code path.
//
// Skipped automatically when the local stack is not running, so `make test`
// stays green on a bare checkout.
//
//	make infra-up && go test ./internal/auth/... -run Google -v

const itClientID = "1234567890-happyfeet.apps.googleusercontent.com"

func TestGoogleSignInCreatesAndReusesAccount(t *testing.T) {
	h, db := newIntegrationHandler(t)
	ctx := context.Background()

	// A distinct Google subject per run keeps repeat runs independent.
	sub := fmt.Sprintf("test-sub-%d", time.Now().UnixNano())
	email := fmt.Sprintf("amina.ngassa.%d@gmail.com", time.Now().UnixNano())
	t.Cleanup(func() { deleteUserByEmail(t, db, email) })

	// ── first sign-in: the account does not exist yet ──────────────────────
	first := postGoogleLogin(t, h, mintForIntegration(t, map[string]any{
		"sub": sub, "email": email,
		"given_name": "Amina", "family_name": "Ngassa", "name": "Amina Ngassa",
	}))

	if first.status != http.StatusCreated {
		t.Fatalf("first sign-in status = %d, want 201 Created\nbody: %s", first.status, first.raw)
	}
	if !first.body.IsNew {
		t.Error("is_new should be true on the sign-in that creates the account")
	}
	if first.body.User.Email != email {
		t.Errorf("user.email = %q, want %q", first.body.User.Email, email)
	}
	if first.body.User.FirstName != "Amina" || first.body.User.LastName != "Ngassa" {
		t.Errorf("name = %q %q, want Amina Ngassa", first.body.User.FirstName, first.body.User.LastName)
	}
	if first.body.Tokens.AccessToken == "" || first.body.Tokens.RefreshToken == "" {
		t.Fatal("a token pair must be issued")
	}
	userID := first.body.User.ID

	// The account really is in the database, with the Google identity linked.
	assertRowExists(t, db, `SELECT 1 FROM users WHERE id=$1 AND email=$2`, userID, email)
	assertRowExists(t, db,
		`SELECT 1 FROM user_social_auth WHERE user_id=$1 AND provider='google' AND provider_uid=$2`,
		userID, sub)

	// Google-created accounts have no password — that login route must stay shut.
	var hash *string
	if err := db.QueryRow(ctx, `SELECT password_hash FROM users WHERE id=$1`, userID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash != nil && *hash != "" {
		t.Error("a Google-created account must not carry a usable password hash")
	}

	// Google asserted the address, so it should not need re-verification.
	var verifiedAt *time.Time
	if err := db.QueryRow(ctx, `SELECT email_verified_at FROM users WHERE id=$1`, userID).Scan(&verifiedAt); err != nil {
		t.Fatal(err)
	}
	if verifiedAt == nil {
		t.Error("email_verified_at should be set — Google verified the address")
	}

	// ── second sign-in: same person, must reuse the account ───────────────
	second := postGoogleLogin(t, h, mintForIntegration(t, map[string]any{
		"sub": sub, "email": email, "given_name": "Amina", "family_name": "Ngassa",
	}))
	if second.status != http.StatusOK {
		t.Fatalf("returning sign-in status = %d, want 200 OK", second.status)
	}
	if second.body.IsNew {
		t.Error("is_new should be false for a returning user")
	}
	if second.body.User.ID != userID {
		t.Errorf("returning sign-in produced a different user id (%s vs %s) — duplicate account",
			second.body.User.ID, userID)
	}
	assertCount(t, db, 1, `SELECT COUNT(*) FROM users WHERE email=$1`, email)
}

// Signing in with Google using the address of an existing password account must
// attach to it, not create a second account for the same person.
func TestGoogleSignInLinksToExistingEmailAccount(t *testing.T) {
	h, db := newIntegrationHandler(t)

	email := fmt.Sprintf("existing.%d@gmail.com", time.Now().UnixNano())
	t.Cleanup(func() { deleteUserByEmail(t, db, email) })

	// Pre-existing local account.
	existingID, err := h.svc.repo.create(context.Background(),
		&email, nil, strPtr("$argon2id$fake"), "Existing", "Member", "customer")
	if err != nil {
		t.Fatal(err)
	}

	sub := fmt.Sprintf("test-sub-link-%d", time.Now().UnixNano())
	res := postGoogleLogin(t, h, mintForIntegration(t, map[string]any{
		"sub": sub, "email": email, "given_name": "Amina", "family_name": "Ngassa",
	}))

	if res.status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (linked, not created)\nbody: %s", res.status, res.raw)
	}
	if res.body.IsNew {
		t.Error("is_new should be false — this linked an existing account")
	}
	if res.body.User.ID != existingID {
		t.Errorf("linked to %s, want the existing account %s", res.body.User.ID, existingID)
	}
	assertCount(t, db, 1, `SELECT COUNT(*) FROM users WHERE email=$1`, email)
	assertRowExists(t, db,
		`SELECT 1 FROM user_social_auth WHERE user_id=$1 AND provider='google'`, existingID)
}

// The session minted by Google sign-in must work on protected routes.
func TestGoogleSessionTokenIsUsable(t *testing.T) {
	h, db := newIntegrationHandler(t)

	email := fmt.Sprintf("session.%d@gmail.com", time.Now().UnixNano())
	t.Cleanup(func() { deleteUserByEmail(t, db, email) })

	res := postGoogleLogin(t, h, mintForIntegration(t, map[string]any{
		"sub": fmt.Sprintf("test-sub-sess-%d", time.Now().UnixNano()), "email": email,
	}))
	if res.status != http.StatusCreated {
		t.Fatalf("status = %d", res.status)
	}

	claims, err := h.svc.maker.VerifyAccessToken(res.body.Tokens.AccessToken)
	if err != nil {
		t.Fatalf("the issued access token does not verify: %v", err)
	}
	if claims.UserID != res.body.User.ID {
		t.Errorf("token subject = %q, want %q", claims.UserID, res.body.User.ID)
	}
}

// A token for another application must be refused by the HTTP layer too, not
// only by the verifier in isolation.
func TestGoogleLoginHandlerRejectsForeignAudience(t *testing.T) {
	h, _ := newIntegrationHandler(t)
	res := postGoogleLogin(t, h, mintForIntegration(t, map[string]any{
		"aud": "some-other-app.apps.googleusercontent.com",
	}))
	if res.status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401\nbody: %s", res.status, res.raw)
	}
}

func TestGoogleLoginHandlerRejectsMissingToken(t *testing.T) {
	h, _ := newIntegrationHandler(t)
	res := postGoogleLogin(t, h, "")
	if res.status != http.StatusUnprocessableEntity && res.status != http.StatusBadRequest {
		t.Fatalf("status = %d, want a 4xx validation error\nbody: %s", res.status, res.raw)
	}
}

// With no client ID configured the endpoint must report unavailable rather than
// authenticate anyone.
func TestGoogleLoginDisabledWithoutClientID(t *testing.T) {
	h, _ := newIntegrationHandler(t)
	h.google = oauth.NewGoogleVerifier() // no client IDs

	res := postGoogleLogin(t, h, "anything")
	if res.status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", res.status)
	}
}

// ── harness ───────────────────────────────────────────────────────────────────

type googleLoginResult struct {
	status int
	raw    string
	body   struct {
		IsNew    bool   `json:"is_new"`
		Provider string `json:"provider"`
		Tokens   struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"tokens"`
		User struct {
			ID        string `json:"id"`
			Email     string `json:"email"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"user"`
	}
}

func postGoogleLogin(t *testing.T, h *Handler, idToken string) googleLoginResult {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"id_token": idToken})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/social/google", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.GoogleLogin(rec, req)

	out := googleLoginResult{status: rec.Code, raw: rec.Body.String()}
	_ = json.Unmarshal(rec.Body.Bytes(), &out.body)
	return out
}

// fakeGoogleKeys is the stand-in signing authority, shared by the whole package
// so every test mints tokens with the same key.
var (
	fakeKey    *rsa.PrivateKey
	fakeKeySrv *httptest.Server
)

const fakeKID = "integration-key"

func startFakeGoogle(t *testing.T) string {
	t.Helper()
	if fakeKeySrv != nil {
		return fakeKeySrv.URL
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	fakeKey = key
	fakeKeySrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := base64.RawURLEncoding.EncodeToString(key.N.Bytes())
		e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"keys":[{"kid":%q,"kty":"RSA","alg":"RS256","use":"sig","n":%q,"e":%q}]}`, fakeKID, n, e)
	}))
	return fakeKeySrv.URL
}

func mintForIntegration(t *testing.T, override map[string]any) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss": "https://accounts.google.com",
		"aud": itClientID,
		"sub": "default-sub",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"email":          "default@gmail.com",
		"email_verified": true,
	}
	for k, v := range override {
		claims[k] = v
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = fakeKID
	signed, err := tok.SignedString(fakeKey)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

// newIntegrationHandler wires the real service over the real database.
func newIntegrationHandler(t *testing.T) (*Handler, *database.DB) {
	t.Helper()
	jwksURL := startFakeGoogle(t)

	cfg := &config.Config{}
	cfg.App.Env = "test"
	cfg.JWT = config.JWTConfig{
		AccessSecret:       "test-access-secret-at-least-32-characters",
		RefreshSecret:      "test-refresh-secret-at-least-32-characters",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 24 * time.Hour,
	}
	cfg.DB.URL = envOr("TEST_DATABASE_URL",
		"postgres://happyfeet:happyfeet_dev_secret@localhost:5433/happyfeet?sslmode=disable")
	cfg.DB.MaxConns, cfg.DB.MinConns = 4, 1
	cfg.Redis.URL = envOr("TEST_REDIS_URL", "redis://localhost:6380/1")

	ctx := context.Background()
	db, err := database.Connect(ctx, &cfg.DB)
	if err != nil {
		t.Skipf("local Postgres unavailable (%v) — run `make infra-up` to include this test", err)
	}
	t.Cleanup(db.Close)

	rdb, err := cache.New(&cfg.Redis)
	if err != nil {
		t.Skipf("local Redis unavailable (%v) — run `make infra-up` to include this test", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })

	verifier := oauth.NewGoogleVerifier(itClientID)
	verifier.SetJWKSURLForTesting(jwksURL)

	svc := NewService(NewRepository(db), rdb, token.NewMaker(&cfg.JWT), cfg)
	return NewHandler(svc, verifier, nil), db
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func deleteUserByEmail(t *testing.T, db *database.DB, email string) {
	t.Helper()
	// user_social_auth cascades on user delete.
	_, _ = db.Exec(context.Background(), `DELETE FROM users WHERE email=$1`, email)
}

func assertRowExists(t *testing.T, db *database.DB, query string, args ...any) {
	t.Helper()
	var one int
	if err := db.QueryRow(context.Background(), query, args...).Scan(&one); err != nil {
		t.Errorf("expected a row for %q %v: %v", query, args, err)
	}
}

func assertCount(t *testing.T, db *database.DB, want int, query string, args ...any) {
	t.Helper()
	var got int
	if err := db.QueryRow(context.Background(), query, args...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("count = %d, want %d (%s %v)", got, want, query, args)
	}
}
