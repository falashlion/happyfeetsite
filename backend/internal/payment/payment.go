package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/happyfeet/api/pkg/cache"
	"github.com/happyfeet/api/pkg/config"
	"github.com/happyfeet/api/pkg/database"
	"github.com/happyfeet/api/pkg/middleware"
	"github.com/happyfeet/api/pkg/response"
	"github.com/happyfeet/api/pkg/validator"
	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("payment: not found")

// ── model ─────────────────────────────────────────────────────────────────────

type Payment struct {
	ID          string
	OrderID     string
	Provider    string
	Status      string
	Amount      float64
	Currency    string
	ProviderRef *string
	USSDString  *string
	CheckoutURL *string
	ExpiresAt   *time.Time
	InitiatedAt time.Time
	ConfirmedAt *time.Time
}

// ── repository ────────────────────────────────────────────────────────────────

type Repository struct {
	db    *database.DB
	cache *cache.Client
}

func NewRepository(db *database.DB, c *cache.Client) *Repository { return &Repository{db, c} }

func (r *Repository) Create(ctx context.Context, orderID, provider, idempotencyKey string, amount float64, currency string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO payments(order_id,provider,status,amount,currency,idempotency_key)
		VALUES($1,$2::payment_provider,'INITIATED',$3,$4::currency_code,$5::uuid)
		RETURNING id`,
		orderID, provider, amount, currency, idempotencyKey).Scan(&id)
	return id, err
}

func (r *Repository) UpdateStatus(ctx context.Context, id, status, ref string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE payments SET status=$2::payment_status,provider_reference=$3,updated_at=NOW() WHERE id=$1`,
		id, status, ref)
	return err
}

func (r *Repository) ConfirmByRef(ctx context.Context, ref string) {
	_, _ = r.db.Exec(ctx,
		`UPDATE payments SET status='COMPLETED',confirmed_at=NOW(),updated_at=NOW() WHERE provider_reference=$1`, ref)
	_, _ = r.db.Exec(ctx,
		`UPDATE orders SET status='PAID',updated_at=NOW() WHERE id=(SELECT order_id FROM payments WHERE provider_reference=$1)`, ref)
}

func (r *Repository) GetByID(ctx context.Context, id, userID string) (*Payment, error) {
	p := &Payment{}
	err := r.db.QueryRow(ctx, `
		SELECT p.id,p.order_id,p.provider::text,p.status::text,p.amount,p.currency::text,
		       p.provider_reference,p.expires_at,p.initiated_at,p.confirmed_at
		FROM payments p JOIN orders o ON o.id=p.order_id
		WHERE p.id=$1 AND o.user_id=$2`, id, userID).Scan(
		&p.ID, &p.OrderID, &p.Provider, &p.Status, &p.Amount, &p.Currency,
		&p.ProviderRef, &p.ExpiresAt, &p.InitiatedAt, &p.ConfirmedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (r *Repository) OrderAmount(ctx context.Context, orderID, userID string) (float64, string, error) {
	var amount float64
	var currency string
	err := r.db.QueryRow(ctx,
		`SELECT total_amount,currency FROM orders WHERE id=$1 AND user_id=$2 AND status='PLACED'`,
		orderID, userID).Scan(&amount, &currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", fmt.Errorf("order not found or not in PLACED status")
	}
	return amount, currency, err
}

func (r *Repository) Cached(ctx context.Context, key string) (*Payment, bool) {
	var p Payment
	if err := r.cache.GetJSON(ctx, cache.Key(cache.KeyIdempotency, key), &p); err == nil {
		return &p, true
	}
	return nil, false
}

func (r *Repository) SetCache(ctx context.Context, key string, p *Payment) {
	_ = r.cache.SetJSON(ctx, cache.Key(cache.KeyIdempotency, key), p, 24*time.Hour)
}

// ── handler ───────────────────────────────────────────────────────────────────

type Handler struct {
	repo *Repository
	cfg  *config.Config
}

func NewHandler(repo *Repository, cfg *config.Config) *Handler { return &Handler{repo, cfg} }

type initiateReq struct {
	OrderID     string  `json:"order_id"     validate:"required"`
	Provider    string  `json:"provider"     validate:"required,oneof=mtn_momo orange_money stripe cash_on_delivery"`
	PhoneNumber *string `json:"phone_number" validate:"omitempty,e164"`
	ReturnURL   *string `json:"return_url"`
}

// InitiatePayment godoc
// @Summary      Initiate payment
// @Tags         payments
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string      true  "Idempotency key"
// @Param        body            body    initiateReq  true  "Payment request"
// @Success      200  {object}  map[string]any
// @Router       /payments/initiate [post]
func (h *Handler) Initiate(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	userID := middleware.GetUserID(r.Context())
	ikey := middleware.IdempotencyKey(r)

	if cached, ok := h.repo.Cached(r.Context(), ikey); ok {
		response.Ok(w, fmtPayment(cached))
		return
	}

	var req initiateReq
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	if (req.Provider == "mtn_momo" || req.Provider == "orange_money") && req.PhoneNumber == nil {
		response.BadRequest(w, "phone_number required for mobile money", rid)
		return
	}

	amount, currency, err := h.repo.OrderAmount(r.Context(), req.OrderID, userID)
	if err != nil {
		response.BadRequest(w, "Order not found or not eligible for payment", rid)
		return
	}

	paymentID, err := h.repo.Create(r.Context(), req.OrderID, req.Provider, ikey, amount, currency)
	if err != nil {
		response.InternalError(w, rid)
		return
	}

	var ref, checkout, ussd string
	exp := time.Now().Add(10 * time.Minute)

	switch req.Provider {
	case "mtn_momo":
		ref = "MTN-" + paymentID[:8]
		phone := strings.TrimPrefix(*req.PhoneNumber, "+")
		ussd = fmt.Sprintf("*126*1*1*%s*%d#", phone, int(amount))
	case "orange_money":
		ref = "OM-" + paymentID[:8]
		checkout = h.cfg.App.FrontendURL + "/checkout/orange?ref=" + ref
	case "stripe":
		ref = "pi_" + paymentID[:16]
		checkout = "https://checkout.stripe.com/pay/" + ref
		exp = time.Now().Add(24 * time.Hour)
	case "cash_on_delivery":
		ref = "COD-" + paymentID[:8]
		exp = time.Now().Add(7 * 24 * time.Hour)
	}

	_ = h.repo.UpdateStatus(r.Context(), paymentID, "PENDING", ref)

	p := &Payment{
		ID: paymentID, OrderID: req.OrderID, Provider: req.Provider, Status: "PENDING",
		Amount: amount, Currency: currency, ProviderRef: &ref,
		InitiatedAt: time.Now(), ExpiresAt: &exp,
	}
	if ussd != "" {
		p.USSDString = &ussd
	}
	if checkout != "" {
		p.CheckoutURL = &checkout
	}

	h.repo.SetCache(r.Context(), ikey, p)
	response.Ok(w, fmtPayment(p))
}

// GetPaymentStatus godoc
// @Summary      Get payment status
// @Tags         payments
// @Security     BearerAuth
// @Param        id  path  string  true  "Payment UUID"
// @Success      200  {object}  map[string]any
// @Router       /payments/{id}/status [get]
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	p, err := h.repo.GetByID(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()))
	if errors.Is(err, ErrNotFound) {
		response.NotFound(w, "Payment", rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, fmtPayment(p))
}

// MTNWebhook godoc
// @Summary      MTN MoMo webhook
// @Tags         payments
// @Accept       json
// @Success      200
// @Router       /payments/webhook/mtn [post]
func (h *Handler) MTNWebhook(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var payload struct {
		Status                 string `json:"status"`
		FinancialTransactionID string `json:"financialTransactionId"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Status == "SUCCESSFUL" {
		h.repo.ConfirmByRef(r.Context(), payload.FinancialTransactionID)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"received":true}`))
}

// OrangeWebhook godoc
// @Summary      Orange Money webhook
// @Tags         payments
// @Accept       json
// @Success      200
// @Router       /payments/webhook/orange [post]
func (h *Handler) OrangeWebhook(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var payload struct {
		Status     string `json:"status"`
		NotifToken string `json:"notif_token"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Status == "SUCCESS" {
		h.repo.ConfirmByRef(r.Context(), payload.NotifToken)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"received":true}`))
}

// StripeWebhook godoc
// @Summary      Stripe webhook
// @Tags         payments
// @Accept       json
// @Success      200
// @Router       /payments/webhook/stripe [post]
func (h *Handler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var event struct {
		Type string `json:"type"`
		Data struct {
			Object struct{ ID string `json:"id"` } `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &event); err == nil && event.Type == "payment_intent.succeeded" {
		h.repo.ConfirmByRef(r.Context(), event.Data.Object.ID)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"received":true}`))
}

// InitiateRefund godoc
// @Summary      Initiate refund (admin)
// @Tags         payments
// @Security     BearerAuth
// @Param        id  path  string  true  "Payment UUID"
// @Success      200  {object}  map[string]any
// @Router       /payments/{id}/refund [post]
func (h *Handler) Refund(w http.ResponseWriter, r *http.Request) {
	response.Ok(w, map[string]string{"status": "REFUNDING", "payment_id": chi.URLParam(r, "id")})
}

func fmtPayment(p *Payment) map[string]any {
	return map[string]any{
		"id": p.ID, "order_id": p.OrderID, "provider": p.Provider, "status": p.Status,
		"amount": p.Amount, "currency": p.Currency, "provider_reference": p.ProviderRef,
		"ussd_string": p.USSDString, "checkout_url": p.CheckoutURL,
		"expires_at": p.ExpiresAt, "initiated_at": p.InitiatedAt, "confirmed_at": p.ConfirmedAt,
	}
}

// Keep unused context import happy (context used in repo methods)
var _ = context.Background
