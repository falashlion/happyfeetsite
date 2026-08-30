package review

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/happyfeet/api/pkg/database"
	"github.com/happyfeet/api/pkg/middleware"
	"github.com/happyfeet/api/pkg/response"
	"github.com/happyfeet/api/pkg/validator"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNotVerified  = errors.New("review: not verified buyer")
	ErrDuplicate    = errors.New("review: already reviewed")
)

type Review struct {
	ID               string
	ProductID        string
	UserID           string
	Rating           int
	Title            *string
	Body             *string
	VerifiedPurchase bool
	HelpfulCount     int
	CreatedAt        time.Time
}

type Repository struct{ db *database.DB }

func NewRepository(db *database.DB) *Repository { return &Repository{db} }

func (r *Repository) Create(ctx context.Context, userID, orderID string, rating int, title, body *string) (*Review, error) {
	// Verify purchase
	var productID, orderItemID string
	err := r.db.QueryRow(ctx, `
		SELECT oi.product_id,oi.id FROM order_items oi
		JOIN orders o ON o.id=oi.order_id
		WHERE o.id=$1 AND o.user_id=$2 AND o.status IN ('DELIVERED','RETURNED')
		LIMIT 1`, orderID, userID).Scan(&productID, &orderItemID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotVerified
	}
	if err != nil {
		return nil, err
	}

	var exists bool
	_ = r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM reviews WHERE order_item_id=$1)`, orderItemID).Scan(&exists)
	if exists {
		return nil, ErrDuplicate
	}

	var rev Review
	err = r.db.QueryRow(ctx, `
		INSERT INTO reviews(product_id,user_id,order_id,order_item_id,rating,title,body,verified_purchase)
		VALUES($1,$2,$3,$4,$5,$6,$7,true)
		RETURNING id,product_id,user_id,rating,helpful_count,created_at`,
		productID, userID, orderID, orderItemID, rating, title, body).
		Scan(&rev.ID, &rev.ProductID, &rev.UserID, &rev.Rating, &rev.HelpfulCount, &rev.CreatedAt)
	if err != nil {
		return nil, err
	}
	rev.Title, rev.Body, rev.VerifiedPurchase = title, body, true
	return &rev, nil
}

func (r *Repository) MarkHelpful(ctx context.Context, reviewID, userID string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO review_helpful_votes(user_id,review_id) VALUES($1,$2) ON CONFLICT DO NOTHING`,
		userID, reviewID)
	if err == nil {
		_, _ = r.db.Exec(ctx, `UPDATE reviews SET helpful_count=helpful_count+1 WHERE id=$1`, reviewID)
	}
	return err
}

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo} }

// CreateReview godoc
// @Summary      Submit product review (verified buyers only)
// @Tags         reviews
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Success      201  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Failure      409  {object}  map[string]any
// @Router       /reviews [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req struct {
		OrderID string  `json:"order_id" validate:"required"`
		Rating  int     `json:"rating"   validate:"required,gte=1,lte=5"`
		Title   *string `json:"title"    validate:"omitempty,max=150"`
		Body    *string `json:"body"     validate:"omitempty,max=1000"`
	}
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	rev, err := h.repo.Create(r.Context(), middleware.GetUserID(r.Context()), req.OrderID, req.Rating, req.Title, req.Body)
	switch {
	case errors.Is(err, ErrNotVerified):
		response.Err(w, http.StatusForbidden, "NOT_VERIFIED_BUYER", "You must have received this product to review it", rid)
	case errors.Is(err, ErrDuplicate):
		response.Err(w, http.StatusConflict, "ALREADY_REVIEWED", "You have already reviewed this order", rid)
	case err != nil:
		response.InternalError(w, rid)
	default:
		response.Created(w, fmtReview(rev))
	}
}

// MarkHelpful godoc
// @Summary      Mark review as helpful
// @Tags         reviews
// @Security     BearerAuth
// @Param        reviewId  path  string  true  "Review UUID"
// @Success      200  {object}  map[string]any
// @Router       /reviews/{reviewId}/helpful [post]
func (h *Handler) MarkHelpful(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	_ = h.repo.MarkHelpful(r.Context(), chi.URLParam(r, "reviewId"), middleware.GetUserID(r.Context()))
	response.Ok(w, map[string]bool{"success": true})
	_ = rid
}

func fmtReview(rev *Review) map[string]any {
	return map[string]any{
		"id": rev.ID, "product_id": rev.ProductID, "user_id": rev.UserID,
		"rating": rev.Rating, "title": rev.Title, "body": rev.Body,
		"verified_purchase": rev.VerifiedPurchase, "helpful_count": rev.HelpfulCount,
		"created_at": rev.CreatedAt,
	}
}
