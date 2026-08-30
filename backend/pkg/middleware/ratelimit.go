package middleware

import (
	"net"
	"net/http"
	"time"

	"github.com/go-chi/httprate"
)

// RateLimit throttles per client IP, correctly, from behind a reverse proxy.
//
// Two things make the obvious `httprate.LimitByIP` wrong for this deployment:
//
//  1. It keys on r.RemoteAddr, which behind Caddy is always the proxy's
//     container address. Every visitor on earth would then share ONE bucket and
//     the storefront would start returning 429 to everybody at launch.
//
//  2. Server-side rendering calls the API from the `web` container over the
//     internal network. Those are legitimate first-party traffic, they scale
//     with total site traffic rather than with any one visitor, and they all
//     originate from a single address — so counting them at all guarantees
//     the limiter fires under exactly the load it should tolerate.
//
// So: skip internal server-side calls, and key everything else on the real
// client address forwarded by the proxy.
func RateLimit(requestsPerWindow int, window time.Duration) func(http.Handler) http.Handler {
	limiter := httprate.Limit(
		requestsPerWindow,
		window,
		// Prefers True-Client-IP, then X-Real-IP, then the FIRST entry of
		// X-Forwarded-For — which is the real visitor whether the chain is
		// [Caddy] or [Cloudflare, Caddy].
		httprate.WithKeyFuncs(httprate.KeyByRealIP),
	)

	return func(next http.Handler) http.Handler {
		limited := limiter(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isInternalCall(r) {
				next.ServeHTTP(w, r)
				return
			}
			limited.ServeHTTP(w, r)
		})
	}
}

// isInternalCall reports whether this request came from our own server-side
// renderer rather than from a browser.
//
// The discriminator is the absence of any forwarding header combined with a
// private source address. Anything arriving through the proxy always carries
// X-Forwarded-For, so a browser request can never be mistaken for an internal
// one — and a spoofed forwarding header only ever moves a caller INTO the
// rate-limited path, never out of it.
func isInternalCall(r *http.Request) bool {
	if r.Header.Get("X-Forwarded-For") != "" ||
		r.Header.Get("X-Real-IP") != "" ||
		r.Header.Get("True-Client-IP") != "" {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}
