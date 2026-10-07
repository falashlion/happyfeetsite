// Package promotion manages discount events: percentage or fixed-amount
// reductions, optionally gated behind a code, scoped to everything or to one
// category or product, and bounded by a date window and usage caps.
package promotion

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/happyfeet/api/pkg/database"
	"github.com/happyfeet/api/pkg/middleware"
	"github.com/happyfeet/api/pkg/response"
	"github.com/happyfeet/api/pkg/validator"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound = errors.New("promotion: not found")
	ErrNotValid = errors.New("promotion: not applicable")
)

type Promotion struct {
	ID             string     `json:"id"`
	Code           *string    `json:"code"`
	Name           string     `json:"name"`
	Description    *string    `json:"description"`
	DiscountType   string     `json:"discount_type"`
	DiscountValue  float64    `json:"discount_value"`
	MinOrderAmount *float64   `json:"min_order_amount"`
	MaxUses        *int       `json:"max_uses"`
	UsesCount      int        `json:"uses_count"`
	MaxUsesPerUser int        `json:"max_uses_per_user"`
	ApplicableTo   string     `json:"applicable_to"`
	ApplicableID   *string    `json:"applicable_id"`
	StartsAt       time.Time  `json:"starts_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      time.Time  `json:"created_at"`
	// Live is the computed answer to "would this apply right now", which is
	// what an admin actually wants to see. is_active alone does not say it:
	// a promotion can be active but not yet started, expired, or used up.
	Live bool `json:"live"`
}

// ── repository ────────────────────────────────────────────────────────────────

type Repository struct{ db *database.DB }

func NewRepository(db *database.DB) *Repository { return &Repository{db} }

const columns = `id,code,name,description,discount_type,discount_value,min_order_amount,
	max_uses,uses_count,max_uses_per_user,applicable_to,applicable_id,
	starts_at,expires_at,is_active,created_at`

func scan(row pgx.Row) (*Promotion, error) {
	p := &Promotion{}
	err := row.Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.DiscountType, &p.DiscountValue,
		&p.MinOrderAmount, &p.MaxUses, &p.UsesCount, &p.MaxUsesPerUser, &p.ApplicableTo,
		&p.ApplicableID, &p.StartsAt, &p.ExpiresAt, &p.IsActive, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	p.Live = p.live(time.Now())
	return p, nil
}

func (p *Promotion) live(now time.Time) bool {
	switch {
	case !p.IsActive:
		return false
	case now.Before(p.StartsAt):
		return false
	case p.ExpiresAt != nil && now.After(*p.ExpiresAt):
		return false
	case p.MaxUses != nil && p.UsesCount >= *p.MaxUses:
		return false
	}
	return true
}

type CreateParams struct {
	Code           *string
	Name           string
	Description    *string
	DiscountType   string
	DiscountValue  float64
	MinOrderAmount *float64
	MaxUses        *int
	MaxUsesPerUser int
	ApplicableTo   string
	ApplicableID   *string
	StartsAt       time.Time
	ExpiresAt      *time.Time
}

func (r *Repository) Create(ctx context.Context, p CreateParams) (*Promotion, error) {
	return scan(r.db.QueryRow(ctx, `
		INSERT INTO promotions(code,name,description,discount_type,discount_value,
			min_order_amount,max_uses,max_uses_per_user,applicable_to,applicable_id,
			starts_at,expires_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING `+columns,
		p.Code, p.Name, p.Description, p.DiscountType, p.DiscountValue,
		p.MinOrderAmount, p.MaxUses, p.MaxUsesPerUser, p.ApplicableTo, p.ApplicableID,
		p.StartsAt, p.ExpiresAt))
}

func (r *Repository) List(ctx context.Context) ([]*Promotion, error) {
	rows, err := r.db.Query(ctx, `SELECT `+columns+` FROM promotions ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Promotion
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetActive flips a promotion on or off. Ending a sale early is the common
// case, and it must not destroy the record — orders reference it.
func (r *Repository) SetActive(ctx context.Context, id string, active bool) error {
	res, err := r.db.Exec(ctx, `UPDATE promotions SET is_active=$2 WHERE id=$1`, id, active)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ByCode resolves a customer-entered code. Lookup is case-insensitive because
// nobody types a discount code the way it was written.
func (r *Repository) ByCode(ctx context.Context, code string) (*Promotion, error) {
	p, err := scan(r.db.QueryRow(ctx,
		`SELECT `+columns+` FROM promotions WHERE UPPER(code)=UPPER($1)`, code))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// ── handler ───────────────────────────────────────────────────────────────────

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo} }

type createReq struct {
	Code           *string  `json:"code"              validate:"omitempty,max=50"`
	Name           string   `json:"name"              validate:"required,max=200"`
	Description    *string  `json:"description"`
	DiscountType   string   `json:"discount_type"     validate:"required,oneof=percentage fixed_amount free_shipping buy_one_get_one"`
	DiscountValue  float64  `json:"discount_value"    validate:"gte=0"`
	MinOrderAmount *float64 `json:"min_order_amount"  validate:"omitempty,gte=0"`
	MaxUses        *int     `json:"max_uses"          validate:"omitempty,gte=1"`
	MaxUsesPerUser *int     `json:"max_uses_per_user" validate:"omitempty,gte=1"`
	ApplicableTo   string   `json:"applicable_to"     validate:"omitempty,oneof=all category product"`
	ApplicableID   *string  `json:"applicable_id"     validate:"omitempty,uuid4"`
	StartsAt       *string  `json:"starts_at"`
	ExpiresAt      *string  `json:"expires_at"`
}

// Create godoc
// @Summary      Create a discount event (admin)
// @Tags         promotions
// @Security     BearerAuth
// @Success      201  {object}  map[string]any
// @Router       /admin/promotions [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req createReq
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}

	// A percentage over 100 turns an order negative. Reject it here rather than
	// discovering it at checkout.
	if req.DiscountType == "percentage" && req.DiscountValue > 100 {
		response.BadRequest(w, "a percentage discount cannot exceed 100", rid)
		return
	}

	starts := time.Now()
	if req.StartsAt != nil && *req.StartsAt != "" {
		t, err := time.Parse(time.RFC3339, *req.StartsAt)
		if err != nil {
			response.BadRequest(w, "starts_at must be RFC3339", rid)
			return
		}
		starts = t
	}
	var expires *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			response.BadRequest(w, "expires_at must be RFC3339", rid)
			return
		}
		if !t.After(starts) {
			response.BadRequest(w, "expires_at must be after starts_at", rid)
			return
		}
		expires = &t
	}

	applicableTo := req.ApplicableTo
	if applicableTo == "" {
		applicableTo = "all"
	}
	if applicableTo != "all" && req.ApplicableID == nil {
		response.BadRequest(w, "applicable_id is required when applicable_to is not 'all'", rid)
		return
	}

	perUser := 1
	if req.MaxUsesPerUser != nil {
		perUser = *req.MaxUsesPerUser
	}

	// An empty string is not "no code" — it would collide on the UNIQUE index
	// with the next codeless promotion. NULL is what the schema means by that.
	code := req.Code
	if code != nil && strings.TrimSpace(*code) == "" {
		code = nil
	}

	p, err := h.repo.Create(r.Context(), CreateParams{
		Code: code, Name: req.Name, Description: req.Description,
		DiscountType: req.DiscountType, DiscountValue: req.DiscountValue,
		MinOrderAmount: req.MinOrderAmount, MaxUses: req.MaxUses, MaxUsesPerUser: perUser,
		ApplicableTo: applicableTo, ApplicableID: req.ApplicableID,
		StartsAt: starts, ExpiresAt: expires,
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			response.Conflict(w, "That discount code is already in use", rid)
			return
		}
		response.InternalError(w, rid)
		return
	}
	response.Created(w, p)
}

// List godoc
// @Summary      List discount events (admin)
// @Tags         promotions
// @Security     BearerAuth
// @Success      200  {object}  map[string]any
// @Router       /admin/promotions [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	list, err := h.repo.List(r.Context())
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, map[string]any{"data": list})
}

// SetActive godoc
// @Summary      Activate or end a discount event (admin)
// @Tags         promotions
// @Security     BearerAuth
// @Success      204
// @Router       /admin/promotions/{id}/active [patch]
func (h *Handler) SetActive(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		response.NotFound(w, "Promotion", rid)
		return
	}

	var req struct {
		IsActive *bool `json:"is_active" validate:"required"`
	}
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}

	err := h.repo.SetActive(r.Context(), id, *req.IsActive)
	if errors.Is(err, ErrNotFound) {
		response.NotFound(w, "Promotion", rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.NoContent(w)
}

// Validate godoc
// @Summary      Check a discount code
// @Tags         promotions
// @Success      200  {object}  map[string]any
// @Router       /promotions/validate [get]
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		response.BadRequest(w, "code is required", rid)
		return
	}

	p, err := h.repo.ByCode(r.Context(), code)
	if errors.Is(err, ErrNotFound) || (err == nil && !p.Live) {
		// One answer for "no such code" and "expired code": a customer gains
		// nothing from the difference, and probing would otherwise reveal
		// which codes exist.
		response.Ok(w, map[string]any{"valid": false})
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}

	response.Ok(w, map[string]any{
		"valid":            true,
		"name":             p.Name,
		"discount_type":    p.DiscountType,
		"discount_value":   p.DiscountValue,
		"min_order_amount": p.MinOrderAmount,
	})
}
