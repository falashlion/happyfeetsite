package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/happyfeet/api/pkg/cache"
	"github.com/happyfeet/api/pkg/logger"
	"github.com/happyfeet/api/pkg/response"
	"github.com/happyfeet/api/pkg/token"
)

type ctxKey string

const (
	keyUserID    ctxKey = "uid"
	keyUserRole  ctxKey = "role"
	keyJTI       ctxKey = "jti"
	keyRequestID ctxKey = "rid"
)

// ── RequestID ─────────────────────────────────────────────────────────────────

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = "req_" + uuid.NewString()
		}
		ctx := context.WithValue(r.Context(), keyRequestID, rid)
		w.Header().Set("X-Request-ID", rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(keyRequestID).(string); ok {
		return v
	}
	return ""
}

// ── Logger ────────────────────────────────────────────────────────────────────

type loggingRW struct {
	http.ResponseWriter
	status int
}

func (lw *loggingRW) WriteHeader(s int) { lw.status = s; lw.ResponseWriter.WriteHeader(s) }

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingRW{ResponseWriter: w, status: 200}
		next.ServeHTTP(lw, r)
		logger.FromContext(r.Context()).Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", lw.status).
			Dur("ms", time.Since(start)).
			Str("rid", GetRequestID(r.Context())).
			Msg("request")
	})
}

// ── Recovery ──────────────────────────────────────────────────────────────────

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				rid := GetRequestID(r.Context())
				logger.FromContext(r.Context()).Error().
					Interface("panic", rec).
					Bytes("stack", debug.Stack()).
					Msg("panic")
				response.InternalError(w, rid)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ── CORS ──────────────────────────────────────────────────────────────────────

func CORS(origins []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID", "X-Guest-Token", "Idempotency-Key"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}

// ── Security headers ──────────────────────────────────────────────────────────

func Secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		next.ServeHTTP(w, r)
	})
}

// ── Auth ──────────────────────────────────────────────────────────────────────

func Auth(maker *token.Maker, rdb *cache.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := GetRequestID(r.Context())
			tok := bearerToken(r)
			if tok == "" {
				response.Unauthorized(w, rid)
				return
			}
			claims, err := maker.VerifyAccessToken(tok)
			if err != nil {
				response.Unauthorized(w, rid)
				return
			}
			// Check blacklist
			ok, _ := rdb.Exists(r.Context(), cache.Key(cache.KeySessionBlacklist, claims.JTI))
			if ok {
				response.Unauthorized(w, rid)
				return
			}
			ctx := r.Context()
			ctx = context.WithValue(ctx, keyUserID, claims.UserID)
			ctx = context.WithValue(ctx, keyUserRole, claims.Role)
			ctx = context.WithValue(ctx, keyJTI, claims.JTI)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth validates JWT if present, silently skips if absent.
func OptionalAuth(maker *token.Maker, rdb *cache.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tok := bearerToken(r); tok != "" {
				if claims, err := maker.VerifyAccessToken(tok); err == nil {
					ok, _ := rdb.Exists(r.Context(), cache.Key(cache.KeySessionBlacklist, claims.JTI))
					if !ok {
						ctx := r.Context()
						ctx = context.WithValue(ctx, keyUserID, claims.UserID)
						ctx = context.WithValue(ctx, keyUserRole, claims.Role)
						ctx = context.WithValue(ctx, keyJTI, claims.JTI)
						r = r.WithContext(ctx)
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole ensures the user has one of the allowed roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	set := make(map[string]bool)
	for _, r := range roles {
		set[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := GetRequestID(r.Context())
			if !set[GetRole(r.Context())] {
				response.Forbidden(w, rid)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireIdempotencyKey enforces the Idempotency-Key header.
func RequireIdempotencyKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Idempotency-Key") == "" {
			rid := GetRequestID(r.Context())
			response.BadRequest(w, "Idempotency-Key header is required", rid)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── Accessors ─────────────────────────────────────────────────────────────────

func GetUserID(ctx context.Context) string {
	v, _ := ctx.Value(keyUserID).(string)
	return v
}
func GetRole(ctx context.Context) string {
	v, _ := ctx.Value(keyUserRole).(string)
	return v
}
func GetJTI(ctx context.Context) string {
	v, _ := ctx.Value(keyJTI).(string)
	return v
}
func IsAuthenticated(ctx context.Context) bool { return GetUserID(ctx) != "" }

// ── Health ────────────────────────────────────────────────────────────────────

func Healthz(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","version":%q}`, version)
	}
}

func Readyz(dbPing, redisPing func() error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbOK := dbPing() == nil
		redisOK := redisPing() == nil
		if !dbOK || !redisOK {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"db":%v,"redis":%v}`, dbOK, redisOK)
	}
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return h[7:]
	}
	return ""
}

// IdempotencyKey returns the Idempotency-Key header value.
func IdempotencyKey(r *http.Request) string { return r.Header.Get("Idempotency-Key") }
