// Package main is the HappyFeet API server entry point.
//
//	@title						HappyFeet E-Commerce API
//	@version					1.0.0
//	@description				High-performance e-commerce API for HappyFeet shoe store. Built with Go for 100k+ concurrent users.
//	@termsOfService				https://happyfeet.com/terms
//	@contact.name				HappyFeet Engineering
//	@contact.email				engineering@happyfeet.com
//	@license.name				Proprietary
//	@license.url				https://happyfeet.com/terms
//	@host						localhost:8080
//	@BasePath					/api/v1
//	@schemes					http https
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				JWT access token. Format: "Bearer <token>"
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/happyfeet/api/internal/admin"
	"github.com/happyfeet/api/internal/auth"
	"github.com/happyfeet/api/internal/cart"
	"github.com/happyfeet/api/internal/media"
	"github.com/happyfeet/api/internal/notification"
	"github.com/happyfeet/api/internal/order"
	"github.com/happyfeet/api/internal/payment"
	"github.com/happyfeet/api/internal/product"
	"github.com/happyfeet/api/internal/review"
	"github.com/happyfeet/api/internal/user"
	"github.com/happyfeet/api/internal/vendor"
	"github.com/happyfeet/api/internal/wishlist"
	"github.com/happyfeet/api/pkg/cache"
	"github.com/happyfeet/api/pkg/config"
	"github.com/happyfeet/api/pkg/database"
	"github.com/happyfeet/api/pkg/logger"
	"github.com/happyfeet/api/pkg/mailer"
	"github.com/happyfeet/api/pkg/middleware"
	"github.com/happyfeet/api/pkg/oauth"
	"github.com/happyfeet/api/pkg/token"
)

// healthcheck lets the container probe itself. The runtime image is
// distroless — no shell, no curl, no wget — so a Docker HEALTHCHECK has nothing
// to call unless the binary can check itself.
//
//	HEALTHCHECK CMD ["/happyfeet-api", "-healthcheck"]
func healthcheck() int {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/readyz")
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: /readyz returned %s\n", resp.Status)
		return 1
	}
	return 0
}

func main() {
	// Must be handled before any config, database or Redis work: the probe runs
	// as a separate short-lived process many times a minute.
	probe := flag.Bool("healthcheck", false, "probe the running server and exit 0 (ready) or 1")
	flag.Parse()
	if *probe {
		os.Exit(healthcheck())
	}

	// ── Config ────────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	// ── Logger ────────────────────────────────────────────────────────────────
	log := logger.Setup(cfg.App.Env, cfg.App.Version, cfg.App.Debug)
	log.Info().Str("env", cfg.App.Env).Str("version", cfg.App.Version).Msg("Starting HappyFeet API")

	ctx := context.Background()

	// ── Database ──────────────────────────────────────────────────────────────
	db, err := database.Connect(ctx, &cfg.DB)
	if err != nil {
		log.Fatal().Err(err).Msg("database connect failed")
	}
	defer db.Close()
	log.Info().Msg("Database connected")

	// ── Redis ─────────────────────────────────────────────────────────────────
	rdb, err := cache.New(&cfg.Redis)
	if err != nil {
		log.Fatal().Err(err).Msg("redis connect failed")
	}
	defer rdb.Close()
	log.Info().Msg("Redis connected")

	// ── Token maker ───────────────────────────────────────────────────────────
	maker := token.NewMaker(&cfg.JWT)

	// ── Repositories ──────────────────────────────────────────────────────────
	authRepo := auth.NewRepository(db)
	userRepo := user.NewRepository(db)
	productRepo := product.NewRepository(db)
	cartRepo := cart.NewRepository(db, rdb)
	orderRepo := order.NewRepository(db)
	paymentRepo := payment.NewRepository(db, rdb)
	reviewRepo := review.NewRepository(db)
	wishlistRepo := wishlist.NewRepository(db)
	notifRepo := notification.NewRepository(db)
	vendorRepo := vendor.NewRepository(db)
	adminRepo := admin.NewRepository(db)

	// ── Services ──────────────────────────────────────────────────────────────
	authSvc := auth.NewService(authRepo, rdb, maker, cfg)

	// Sign in with Google. Unset GOOGLE_CLIENT_ID leaves the verifier disabled,
	// and /auth/social/google answers 503 instead of trusting anything.
	googleVerifier := oauth.NewGoogleVerifier(cfg.OAuth.GoogleClientIDs...)
	if googleVerifier.Enabled() {
		log.Info().Str("client_id", cfg.OAuth.GoogleClientIDs[0]).Msg("Google sign-in enabled")
	} else {
		log.Warn().Msg("GOOGLE_CLIENT_ID is unset — Google sign-in disabled")
	}

	// Transactional email. A missing relay is a warning, not a fatal error —
	// the storefront stays fully functional without it.
	mail := mailer.New(cfg.SMTP, log)
	emailSvc, err := notification.NewEmailService(mail, cfg, log)
	if err != nil {
		log.Fatal().Err(err).Msg("email templates failed to parse")
	}
	switch {
	case !emailSvc.Enabled():
		log.Warn().Msg("SMTP_HOST is unset — order emails will not be sent")
	case cfg.Store.OwnerEmail == "":
		log.Warn().Msg("STORE_OWNER_EMAIL is unset — merchant order alerts will not be sent")
	default:
		log.Info().
			Str("relay", fmt.Sprintf("%s:%d", cfg.SMTP.Host, cfg.SMTP.Port)).
			Str("owner", cfg.Store.OwnerEmail).
			Msg("Order email enabled")
	}

	// ── Handlers ──────────────────────────────────────────────────────────────
	authH := auth.NewHandler(authSvc, googleVerifier)
	userH := user.NewHandler(userRepo)
	productH := product.NewHandler(productRepo)
	cartH := cart.NewHandler(cartRepo)
	orderH := order.NewHandler(orderRepo, emailSvc)
	paymentH := payment.NewHandler(paymentRepo, cfg)
	reviewH := review.NewHandler(reviewRepo)
	wishlistH := wishlist.NewHandler(wishlistRepo)
	mediaH := media.NewHandler(cfg, rdb)
	notifH := notification.NewHandler(notifRepo)
	vendorH := vendor.NewHandler(vendorRepo)
	adminH := admin.NewHandler(adminRepo)

	// ── Middleware shortcuts ───────────────────────────────────────────────────
	authMW := middleware.Auth(maker, rdb)
	optionalAuthMW := middleware.OptionalAuth(maker, rdb)
	_ = optionalAuthMW

	// ── Router ────────────────────────────────────────────────────────────────
	r := chi.NewRouter()

	// Global middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.Recovery)
	r.Use(middleware.Logger)
	r.Use(middleware.Secure)
	r.Use(middleware.CORS([]string{cfg.App.FrontendURL, "http://localhost:3000"}))
	r.Use(chimw.Compress(5))

	// Rate limiting, keyed on the real client address rather than the proxy —
	// see pkg/middleware/ratelimit.go for why LimitByIP is wrong here.
	r.Use(middleware.RateLimit(120, time.Minute))

	// ── Health ────────────────────────────────────────────────────────────────
	r.Get("/healthz", middleware.Healthz(cfg.App.Version))
	r.Get("/readyz", middleware.Readyz(
		func() error { return db.Ping(context.Background()) },
		func() error { return rdb.Ping(context.Background()) },
	))

	// ── Swagger UI ────────────────────────────────────────────────────────────
	r.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusMovedPermanently)
	})
	r.Handle("/swagger/*", http.StripPrefix("/swagger", http.FileServer(http.Dir("./docs/swagger-ui"))))
	r.Get("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./docs/openapi.json")
	})
	r.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./docs/openapi.yaml")
	})

	// ── API v1 ────────────────────────────────────────────────────────────────
	r.Route("/api/v1", func(r chi.Router) {

		// ── Auth (public) ─────────────────────────────────────────────────────
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authH.Register)
			r.Post("/login", authH.Login)
			r.Post("/refresh", authH.Refresh)
			r.Post("/logout", authH.Logout)
			r.Post("/otp/send", authH.SendOTP)
			r.Post("/otp/verify", authH.VerifyOTP)
			r.Post("/password/reset", authH.RequestPasswordReset)
			r.Put("/password/reset", authH.ConfirmPasswordReset)
			r.Get("/providers", authH.Providers)
			r.Post("/social/google", authH.GoogleLogin)
			r.Post("/social/apple", authH.AppleLogin)
		})

		// ── Users (protected) ────────────────────────────────────────────────
		r.Route("/users", func(r chi.Router) {
			r.Use(authMW)
			r.Get("/me", userH.GetMe)
			r.Patch("/me", userH.UpdateMe)
			r.Get("/me/addresses", userH.ListAddresses)
			r.Post("/me/addresses", userH.CreateAddress)
			r.Put("/me/addresses/{addressId}", userH.UpdateAddress)
			r.Delete("/me/addresses/{addressId}", userH.DeleteAddress)
		})

		// ── Products (mostly public) ─────────────────────────────────────────
		r.Route("/products", func(r chi.Router) {
			r.Get("/", productH.List)
			r.Get("/search", productH.Search)
			r.Get("/featured", productH.Featured)
			r.Get("/by-slug/{slug}", productH.GetBySlug)
			r.Get("/{id}", productH.Get)
			r.Get("/{id}/reviews", productH.ListReviews)

			// Protected product mutations
			r.With(authMW).Post("/", productH.Create)
			r.With(authMW).Delete("/{id}", productH.Archive)
		})

		// ── Categories & Brands (public) ─────────────────────────────────────
		r.Get("/categories", productH.ListCategories)
		r.Get("/brands", productH.ListBrands)

		// ── Cart (protected) ─────────────────────────────────────────────────
		r.Route("/cart", func(r chi.Router) {
			r.Use(authMW)
			r.Get("/", cartH.Get)
			r.Delete("/", cartH.Clear)
			r.Post("/items", cartH.AddItem)
			r.Put("/items/{itemId}", cartH.UpdateItem)
			r.Delete("/items/{itemId}", cartH.RemoveItem)
			r.Post("/coupon", cartH.ApplyCoupon)
			r.Delete("/coupon", cartH.RemoveCoupon)
		})

		// ── Orders (protected) ───────────────────────────────────────────────
		r.Route("/orders", func(r chi.Router) {
			r.Use(authMW)
			r.Get("/", orderH.List)
			r.With(middleware.RequireIdempotencyKey).Post("/", orderH.Create)
			r.Get("/by-number/{number}", orderH.GetByNumber)
			r.Get("/{id}", orderH.Get)
			r.Post("/{id}/cancel", orderH.Cancel)
			r.Post("/{id}/return", orderH.Return)
		})

		// ── Payments ─────────────────────────────────────────────────────────
		r.Route("/payments", func(r chi.Router) {
			// Webhooks are public (verified by HMAC signature internally)
			r.Post("/webhook/mtn", paymentH.MTNWebhook)
			r.Post("/webhook/orange", paymentH.OrangeWebhook)
			r.Post("/webhook/stripe", paymentH.StripeWebhook)

			// Protected payment endpoints
			r.With(authMW).With(middleware.RequireIdempotencyKey).Post("/initiate", paymentH.Initiate)
			r.With(authMW).Get("/{id}/status", paymentH.GetStatus)
			r.With(authMW).With(middleware.RequireIdempotencyKey).Post("/{id}/refund", paymentH.Refund)
		})

		// ── Reviews (protected) ──────────────────────────────────────────────
		r.Route("/reviews", func(r chi.Router) {
			r.Use(authMW)
			r.Post("/", reviewH.Create)
			r.Post("/{reviewId}/helpful", reviewH.MarkHelpful)
		})

		// ── Wishlist (protected) ─────────────────────────────────────────────
		r.Route("/wishlist", func(r chi.Router) {
			r.Use(authMW)
			r.Get("/", wishlistH.Get)
			r.Post("/", wishlistH.Add)
			r.Delete("/{productId}", wishlistH.Remove)
		})

		// ── Media (protected) ────────────────────────────────────────────────
		r.Route("/media", func(r chi.Router) {
			r.Use(authMW)
			r.Post("/upload", mediaH.RequestUpload)
			r.Get("/upload/{uploadId}/status", mediaH.GetUploadStatus)
		})

		// ── Notifications (protected) ────────────────────────────────────────
		r.Route("/notifications", func(r chi.Router) {
			r.Use(authMW)
			r.Get("/", notifH.List)
			r.Post("/read-all", notifH.MarkAllRead)
			r.Get("/preferences", notifH.GetPreferences)
			r.Put("/preferences", notifH.UpdatePreferences)
			r.Put("/push-token", authH.UpdatePushToken)
		})

		// ── Vendors ──────────────────────────────────────────────────────────
		r.Route("/vendors", func(r chi.Router) {
			r.Use(authMW)
			r.Post("/apply", vendorH.Apply)
			r.Get("/me", vendorH.GetProfile)
			r.Get("/me/products", vendorH.ListProducts)
			r.Get("/me/orders", vendorH.ListOrders)
			r.Get("/me/earnings", vendorH.GetEarnings)
		})

		// ── Admin (admin/super_admin only) ────────────────────────────────────
		r.Route("/admin", func(r chi.Router) {
			r.Use(authMW)
			r.Use(middleware.RequireRole("admin", "super_admin"))
			r.Get("/users", adminH.ListUsers)
			r.Post("/users/{userId}/suspend", adminH.SuspendUser)
			r.Post("/vendors/{vendorId}/approve", adminH.ApproveVendor)
			r.Get("/analytics/dashboard", adminH.GetDashboard)
		})
	})

	// ── Server ────────────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      r,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	// Graceful shutdown
	serverErr := make(chan error, 1)
	go func() {
		log.Info().Str("addr", cfg.Addr()).Msg("Server listening")
		serverErr <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-serverErr:
		log.Fatal().Err(err).Msg("Server error")
	case sig := <-quit:
		log.Info().Str("signal", sig.String()).Msg("Shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Graceful shutdown failed")
		}
		log.Info().Msg("Server stopped")
	}
}
