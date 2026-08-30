package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// hit sends one request with the given headers and source address, returning
// the status code.
func hit(h http.Handler, remoteAddr string, headers map[string]string) int {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
}

// The bug this middleware exists to prevent: behind a proxy every visitor
// shares the proxy's address, so a naive limiter throttles the whole site.
func TestDistinctVisitorsGetDistinctBuckets(t *testing.T) {
	h := RateLimit(3, time.Minute)(okHandler())

	// Ten visitors, each making the full allowance, all arriving through the
	// same proxy container address.
	for v := 0; v < 10; v++ {
		visitor := fmt.Sprintf("41.202.207.%d", v+1)
		for i := 0; i < 3; i++ {
			code := hit(h, "172.18.0.5:44444", map[string]string{"X-Forwarded-For": visitor})
			if code != http.StatusOK {
				t.Fatalf("visitor %s request %d got %d — buckets are being shared", visitor, i+1, code)
			}
		}
	}
}

func TestVisitorIsThrottledPastTheLimit(t *testing.T) {
	h := RateLimit(3, time.Minute)(okHandler())
	hdr := map[string]string{"X-Forwarded-For": "41.202.207.99"}

	for i := 0; i < 3; i++ {
		if code := hit(h, "172.18.0.5:44444", hdr); code != http.StatusOK {
			t.Fatalf("request %d within the allowance got %d", i+1, code)
		}
	}
	if code := hit(h, "172.18.0.5:44444", hdr); code != http.StatusTooManyRequests {
		t.Errorf("fourth request got %d, want 429", code)
	}
}

// Throttling one visitor must not affect anyone else.
func TestThrottlingIsIsolatedPerVisitor(t *testing.T) {
	h := RateLimit(2, time.Minute)(okHandler())
	noisy := map[string]string{"X-Forwarded-For": "41.202.207.10"}
	quiet := map[string]string{"X-Forwarded-For": "41.202.207.11"}

	for i := 0; i < 5; i++ {
		hit(h, "172.18.0.5:1", noisy)
	}
	if code := hit(h, "172.18.0.5:1", noisy); code != http.StatusTooManyRequests {
		t.Fatalf("noisy visitor should be limited, got %d", code)
	}
	if code := hit(h, "172.18.0.5:1", quiet); code != http.StatusOK {
		t.Errorf("unrelated visitor got %d — collateral throttling", code)
	}
}

// Behind Cloudflare the chain is [client, cloudflare]; the client is first.
func TestRealClientTakenFromForwardedChain(t *testing.T) {
	h := RateLimit(2, time.Minute)(okHandler())

	for i := 0; i < 2; i++ {
		hit(h, "172.18.0.5:1", map[string]string{"X-Forwarded-For": "41.202.207.20, 172.71.0.9"})
	}
	// Same real client, different Cloudflare edge — must be the same bucket.
	if code := hit(h, "172.18.0.5:1", map[string]string{"X-Forwarded-For": "41.202.207.20, 108.162.0.4"}); code != http.StatusTooManyRequests {
		t.Errorf("got %d — the chain's first entry is not being used as the key", code)
	}
	// A genuinely different client behind the same edge is a different bucket.
	if code := hit(h, "172.18.0.5:1", map[string]string{"X-Forwarded-For": "41.202.207.21, 172.71.0.9"}); code != http.StatusOK {
		t.Errorf("got %d — keying on the proxy rather than the client", code)
	}
}

// SSR calls from the web container carry no forwarding headers and must never
// be throttled: they scale with total site traffic, not per-visitor traffic.
func TestInternalServerSideCallsAreNotLimited(t *testing.T) {
	h := RateLimit(3, time.Minute)(okHandler())

	for i := 0; i < 50; i++ {
		if code := hit(h, "172.18.0.7:52000", nil); code != http.StatusOK {
			t.Fatalf("internal SSR request %d got %d — rendering would fail under load", i+1, code)
		}
	}
}

// A caller cannot dodge the limiter by claiming to be internal: the bypass
// requires BOTH a private source address and no forwarding headers, and any
// header they add only puts them back into the limited path.
func TestBypassCannotBeForged(t *testing.T) {
	h := RateLimit(2, time.Minute)(okHandler())

	// Public source address, no headers → not internal, gets limited.
	for i := 0; i < 2; i++ {
		hit(h, "41.202.207.30:1", nil)
	}
	if code := hit(h, "41.202.207.30:1", nil); code != http.StatusTooManyRequests {
		t.Errorf("public client with no headers got %d, want 429", code)
	}
}

func TestIsInternalCall(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		headers map[string]string
		want    bool
	}{
		{"docker bridge, no headers", "172.18.0.7:5000", nil, true},
		{"loopback, no headers", "127.0.0.1:5000", nil, true},
		{"private 10/8", "10.0.0.4:5000", nil, true},
		{"private 192.168/16", "192.168.1.9:5000", nil, true},
		{"public address", "41.202.207.5:5000", nil, false},
		{"private but forwarded — a browser via the proxy", "172.18.0.5:5000",
			map[string]string{"X-Forwarded-For": "41.202.207.5"}, false},
		{"private but X-Real-IP present", "172.18.0.5:5000",
			map[string]string{"X-Real-IP": "41.202.207.5"}, false},
		{"private but True-Client-IP present", "172.18.0.5:5000",
			map[string]string{"True-Client-IP": "41.202.207.5"}, false},
		{"unparseable address", "garbage", nil, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.addr
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			if got := isInternalCall(req); got != tc.want {
				t.Errorf("isInternalCall = %v, want %v", got, tc.want)
			}
		})
	}
}
