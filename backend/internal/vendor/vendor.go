package vendor

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/happyfeet/api/pkg/database"
	"github.com/happyfeet/api/pkg/middleware"
	"github.com/happyfeet/api/pkg/response"
	"github.com/happyfeet/api/pkg/validator"
	"github.com/jackc/pgx/v5"
)

type Vendor struct {
	ID             string
	OwnerID        string
	BusinessName   string
	BusinessType   string
	Description    *string
	Website        *string
	Status         string
	CommissionRate float64
	RatingAvg      float64
	RatingCount    int
	TotalSales     float64
	CreatedAt      time.Time
}

type Repository struct{ db *database.DB }

func NewRepository(db *database.DB) *Repository { return &Repository{db} }

func (r *Repository) GetByOwner(ctx context.Context, ownerID string) (*Vendor, error) {
	v := &Vendor{}
	err := r.db.QueryRow(ctx, `
		SELECT id,owner_id,business_name,business_type,description,website,status,
		       commission_rate,rating_avg,rating_count,total_sales,created_at
		FROM vendors WHERE owner_id=$1`, ownerID).Scan(
		&v.ID, &v.OwnerID, &v.BusinessName, &v.BusinessType, &v.Description, &v.Website,
		&v.Status, &v.CommissionRate, &v.RatingAvg, &v.RatingCount, &v.TotalSales, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("vendor: not found")
	}
	return v, err
}

func (r *Repository) Apply(ctx context.Context, ownerID, businessName, businessType, country string, description, website *string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO vendors(owner_id,business_name,business_type,description,website)
		VALUES($1,$2,$3,$4,$5) RETURNING id`,
		ownerID, businessName, businessType, description, website).Scan(&id)
	if err != nil && isDup(err) {
		return "", errors.New("vendor: application already exists")
	}
	return id, err
}

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo} }

// ApplyVendor godoc
// @Summary      Apply to become a vendor
// @Tags         vendors
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Success      201  {object}  map[string]any
// @Router       /vendors/apply [post]
func (h *Handler) Apply(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req struct {
		BusinessName string  `json:"business_name" validate:"required,max=255"`
		BusinessType string  `json:"business_type" validate:"required,oneof=individual company"`
		Country      string  `json:"country"       validate:"required,len=2"`
		Description  *string `json:"description"`
		Website      *string `json:"website"`
	}
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	id, err := h.repo.Apply(r.Context(), middleware.GetUserID(r.Context()),
		req.BusinessName, req.BusinessType, req.Country, req.Description, req.Website)
	if err != nil {
		response.Conflict(w, "Vendor application already exists", rid)
		return
	}
	response.Created(w, map[string]any{"vendor_id": id, "status": "pending", "message": "Application submitted for review."})
}

// GetVendorProfile godoc
// @Summary      Get vendor profile
// @Tags         vendors
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /vendors/me [get]
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	v, err := h.repo.GetByOwner(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		response.NotFound(w, "Vendor profile", rid)
		return
	}
	response.Ok(w, map[string]any{
		"id": v.ID, "business_name": v.BusinessName, "business_type": v.BusinessType,
		"description": v.Description, "website": v.Website, "status": v.Status,
		"commission_rate": v.CommissionRate, "rating_avg": v.RatingAvg,
		"rating_count": v.RatingCount, "total_sales": v.TotalSales, "created_at": v.CreatedAt,
	})
}

// ListVendorProducts godoc
// @Summary      List vendor's products
// @Tags         vendors
// @Security     BearerAuth
// @Success      200  {object}  map[string]any
// @Router       /vendors/me/products [get]
func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	response.Paginated(w, []any{}, &response.Meta{HasMore: false, Total: 0})
}

// ListVendorOrders godoc
// @Summary      List vendor's orders
// @Tags         vendors
// @Security     BearerAuth
// @Success      200  {object}  map[string]any
// @Router       /vendors/me/orders [get]
func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	response.Paginated(w, []any{}, &response.Meta{HasMore: false, Total: 0})
}

// GetEarnings godoc
// @Summary      Get vendor earnings
// @Tags         vendors
// @Security     BearerAuth
// @Success      200  {object}  map[string]any
// @Router       /vendors/me/earnings [get]
func (h *Handler) GetEarnings(w http.ResponseWriter, r *http.Request) {
	response.Ok(w, map[string]any{
		"total_earnings": 0, "pending_payout": 0, "last_payout": nil, "currency": "XAF",
	})
}

func isDup(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return len(s) > 0 && (containsStr(s, "duplicate") || containsStr(s, "unique"))
}

func containsStr(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

var _ = context.Background
