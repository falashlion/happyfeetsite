// Package oauth verifies identity tokens issued by third-party sign-in
// providers.
//
// Google's "Sign in with Google" hands the browser a signed JWT (an ID token).
// The browser posts it to us and we must prove, without trusting the browser at
// all, that Google issued it *for this application*. That means checking the
// RS256 signature against Google's published keys and then checking the claims:
//
//	iss  one of Google's two issuer spellings
//	aud  our own OAuth client ID — the check that stops an attacker replaying a
//	     valid Google token minted for some *other* site against ours
//	exp  not expired
//
// Verification is offline after the first key fetch: no per-login round trip to
// Google, so a sign-in never waits on their availability.
package oauth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// GoogleJWKSURL publishes the RSA public keys Google signs ID tokens with.
	GoogleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"

	// Google spells its issuer both ways; both are legitimate.
	issuerBare  = "accounts.google.com"
	issuerHTTPS = "https://accounts.google.com"

	// Floor for the key cache, used when Google sends no usable Cache-Control.
	minKeyTTL = 5 * time.Minute
	// Ceiling, so a very long max-age still lets us pick up key rotation.
	maxKeyTTL = 6 * time.Hour
	// Shortest gap between forced refetches when we see an unknown key id.
	// Without it, tokens bearing a bogus kid become a request amplifier
	// pointed at Google.
	minRefetchGap = 30 * time.Second
)

var (
	ErrNotConfigured  = errors.New("oauth: google client ID not configured")
	ErrInvalidToken   = errors.New("oauth: invalid ID token")
	ErrWrongAudience  = errors.New("oauth: token was not issued for this application")
	ErrWrongIssuer    = errors.New("oauth: token was not issued by Google")
	ErrEmailUnverified = errors.New("oauth: Google has not verified this email address")
)

// GoogleClaims is the subset of the ID token payload the storefront uses.
type GoogleClaims struct {
	// Subject is Google's stable, immutable user id. It is the only field safe
	// to key an account on — email addresses can be changed or reassigned.
	Subject       string
	Email         string
	EmailVerified bool
	GivenName     string
	FamilyName    string
	Name          string
	Picture       string
	// HostedDomain is set for Google Workspace accounts ("example.com").
	HostedDomain string
}

// googleIDToken mirrors the wire format.
type googleIDToken struct {
	Issuer        string `json:"iss"`
	Subject       string `json:"sub"`
	Audience      any    `json:"aud"` // string, or []string in rare cases
	Expiry        int64  `json:"exp"`
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"` // Google has shipped both bool and "true"
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	HostedDomain  string `json:"hd"`
}

// GoogleVerifier validates Google ID tokens against a cached copy of Google's
// signing keys. It is safe for concurrent use.
type GoogleVerifier struct {
	clientIDs []string
	jwksURL   string
	http      *http.Client

	mu          sync.RWMutex
	keys        map[string]*rsa.PublicKey
	keysExpire  time.Time
	lastFetched time.Time
}

// NewGoogleVerifier builds a verifier for one or more accepted client IDs.
// Several are allowed because the web, iOS and Android clients of the same
// project each get their own ID and all are legitimate audiences here.
//
// An empty clientID yields a verifier that reports Enabled() == false and
// refuses every token, so an unconfigured deployment fails closed.
func NewGoogleVerifier(clientIDs ...string) *GoogleVerifier {
	cleaned := make([]string, 0, len(clientIDs))
	for _, id := range clientIDs {
		if id = strings.TrimSpace(id); id != "" {
			cleaned = append(cleaned, id)
		}
	}
	return &GoogleVerifier{
		clientIDs: cleaned,
		jwksURL:   GoogleJWKSURL,
		http:      &http.Client{Timeout: 10 * time.Second},
		keys:      map[string]*rsa.PublicKey{},
	}
}

// Enabled reports whether a client ID was configured.
func (v *GoogleVerifier) Enabled() bool { return len(v.clientIDs) > 0 }

// SetJWKSURLForTesting points key discovery at a stand-in for Google.
//
// Deliberately a method rather than a config value: pointing token verification
// at an attacker-controlled key set would let anyone mint valid sign-ins, so
// this must never be reachable from an environment variable or a config file.
// The only callers are tests.
func (v *GoogleVerifier) SetJWKSURLForTesting(url string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.jwksURL = url
	v.keys = map[string]*rsa.PublicKey{}
	v.keysExpire = time.Time{}
	v.lastFetched = time.Time{}
}

// ClientID returns the primary (first) accepted client ID, for surfacing to the
// storefront. Empty when unconfigured.
func (v *GoogleVerifier) ClientID() string {
	if len(v.clientIDs) == 0 {
		return ""
	}
	return v.clientIDs[0]
}

// Verify checks the signature and claims, returning the trusted subset.
func (v *GoogleVerifier) Verify(ctx context.Context, rawToken string) (*GoogleClaims, error) {
	if !v.Enabled() {
		return nil, ErrNotConfigured
	}

	var claims googleIDToken
	_, err := jwt.NewParser(
		// Pin the algorithm. Without this an attacker could present a token
		// signed with HS256 using the public key as the HMAC secret, or "none".
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
	).ParseWithClaims(rawToken, &claims, v.keyFor(ctx))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	if claims.Issuer != issuerBare && claims.Issuer != issuerHTTPS {
		return nil, fmt.Errorf("%w: iss=%q", ErrWrongIssuer, claims.Issuer)
	}
	if !v.audienceAccepted(claims.Audience) {
		return nil, ErrWrongAudience
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("%w: missing sub", ErrInvalidToken)
	}

	verified := truthy(claims.EmailVerified)
	// An unverified address must never be used to attach to an existing local
	// account — that is account takeover by signup. Reject rather than silently
	// dropping the email, so the caller cannot get this wrong.
	if claims.Email != "" && !verified {
		return nil, ErrEmailUnverified
	}

	return &GoogleClaims{
		Subject:       claims.Subject,
		Email:         strings.ToLower(strings.TrimSpace(claims.Email)),
		EmailVerified: verified,
		GivenName:     claims.GivenName,
		FamilyName:    claims.FamilyName,
		Name:          claims.Name,
		Picture:       claims.Picture,
		HostedDomain:  claims.HostedDomain,
	}, nil
}

func (v *GoogleVerifier) audienceAccepted(aud any) bool {
	var got []string
	switch a := aud.(type) {
	case string:
		got = []string{a}
	case []any:
		for _, item := range a {
			if s, ok := item.(string); ok {
				got = append(got, s)
			}
		}
	case []string:
		got = a
	}
	for _, g := range got {
		for _, want := range v.clientIDs {
			if g == want {
				return true
			}
		}
	}
	return false
}

// keyFor resolves the signing key named by the token header's "kid".
func (v *GoogleVerifier) keyFor(ctx context.Context) jwt.Keyfunc {
	return func(tok *jwt.Token) (any, error) {
		kid, _ := tok.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("token header has no kid")
		}

		if key := v.cachedKey(kid); key != nil {
			return key, nil
		}
		// Cache miss: either the cache is cold/stale, or Google rotated keys.
		if err := v.refresh(ctx); err != nil {
			return nil, err
		}
		if key := v.cachedKey(kid); key != nil {
			return key, nil
		}
		return nil, fmt.Errorf("no Google signing key with kid %q", kid)
	}
}

func (v *GoogleVerifier) cachedKey(kid string) *rsa.PublicKey {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if time.Now().After(v.keysExpire) {
		return nil
	}
	return v.keys[kid]
}

// refresh reloads the JWKS, rate-limited so an unknown kid cannot be used to
// hammer Google on our behalf.
func (v *GoogleVerifier) refresh(ctx context.Context) error {
	v.mu.Lock()
	if time.Since(v.lastFetched) < minRefetchGap && len(v.keys) > 0 {
		v.mu.Unlock()
		return nil
	}
	v.lastFetched = time.Now()
	url := v.jwksURL
	client := v.http
	v.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("oauth: fetch Google keys: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("oauth: Google keys returned %s", resp.Status)
	}

	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("oauth: decode Google keys: %w", err)
	}

	parsed := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || (k.Use != "" && k.Use != "sig") {
			continue
		}
		pub, err := rsaKeyFromJWK(k.N, k.E)
		if err != nil {
			continue // skip the unusable key rather than failing the whole set
		}
		parsed[k.Kid] = pub
	}
	if len(parsed) == 0 {
		return errors.New("oauth: Google key set contained no usable RSA keys")
	}

	v.mu.Lock()
	v.keys = parsed
	v.keysExpire = time.Now().Add(cacheTTL(resp.Header.Get("Cache-Control")))
	v.mu.Unlock()
	return nil
}

// rsaKeyFromJWK rebuilds a public key from the base64url modulus and exponent.
func rsaKeyFromJWK(nB64, eB64 string) (*rsa.PublicKey, error) {
	n, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(nB64, "="))
	if err != nil {
		return nil, err
	}
	e, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(eB64, "="))
	if err != nil {
		return nil, err
	}
	if len(n) == 0 || len(e) == 0 {
		return nil, errors.New("empty modulus or exponent")
	}
	exp := new(big.Int).SetBytes(e)
	if !exp.IsInt64() || exp.Int64() > 1<<31-1 {
		return nil, errors.New("exponent out of range")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(exp.Int64())}, nil
}

// cacheTTL reads max-age out of a Cache-Control header, clamped to a sane band.
func cacheTTL(header string) time.Duration {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "max-age=") {
			continue
		}
		secs, err := strconv.Atoi(strings.TrimPrefix(part, "max-age="))
		if err != nil {
			break
		}
		d := time.Duration(secs) * time.Second
		if d < minKeyTTL {
			return minKeyTTL
		}
		if d > maxKeyTTL {
			return maxKeyTTL
		}
		return d
	}
	return minKeyTTL
}

// truthy accepts both the boolean and the stringified forms Google has used for
// email_verified over the years.
func truthy(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true"
	}
	return false
}

// ── jwt.Claims ────────────────────────────────────────────────────────────────
// Implemented by hand: the library's registered-claims helper would re-check
// iss/aud with its own semantics, and we want those checks explicit above.

func (c *googleIDToken) GetExpirationTime() (*jwt.NumericDate, error) {
	if c.Expiry == 0 {
		return nil, nil
	}
	return jwt.NewNumericDate(time.Unix(c.Expiry, 0)), nil
}
func (c *googleIDToken) GetIssuedAt() (*jwt.NumericDate, error)  { return nil, nil }
func (c *googleIDToken) GetNotBefore() (*jwt.NumericDate, error) { return nil, nil }
func (c *googleIDToken) GetIssuer() (string, error)              { return c.Issuer, nil }
func (c *googleIDToken) GetSubject() (string, error)             { return c.Subject, nil }
func (c *googleIDToken) GetAudience() (jwt.ClaimStrings, error)  { return nil, nil }
