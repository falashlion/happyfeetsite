package wishlist

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

type Item struct {
	ProductID    string
	Name         string
	BasePrice    float64
	Currency     string
	PrimaryImage *string
	RatingAvg    float64
	InStock      bool
	AddedAt      time.Time
}

type Repository struct{ db *database.DB }

func NewRepository(db *database.DB) *Repository { return &Repository{db} }

func (r *Repository) List(ctx context.Context, userID string, limit int, cursor *string) ([]Item, string, int64, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.id,p.name,p.base_price,p.currency::text,
		       (SELECT url_thumbnail FROM product_images WHERE product_id=p.id AND is_primary LIMIT 1),
		       p.rating_avg,
		       EXISTS(SELECT 1 FROM product_skus s WHERE s.product_id=p.id AND s.stock_qty>0),
		       wi.created_at
		FROM wishlist_items wi
		JOIN products p ON p.id=wi.product_id
		WHERE wi.user_id=$1 AND p.deleted_at IS NULL
		ORDER BY wi.created_at DESC LIMIT $2`, userID, limit+1)
	if err != nil {
		return nil, "", 0, err
	}
	defer rows.Close()
	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ProductID, &it.Name, &it.BasePrice, &it.Currency, &it.PrimaryImage,
			&it.RatingAvg, &it.InStock, &it.AddedAt); err == nil {
			items = append(items, it)
		}
	}
	var cur string
	if len(items) > limit {
		items = items[:limit]
		cur = items[len(items)-1].ProductID
	}
	var total int64
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM wishlist_items WHERE user_id=$1`, userID).Scan(&total)
	return items, cur, total, nil
}

func (r *Repository) Add(ctx context.Context, userID, productID string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO wishlist_items(user_id,product_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, userID, productID)
	return err
}

func (r *Repository) Remove(ctx context.Context, userID, productID string) error {
	res, err := r.db.Exec(ctx, `DELETE FROM wishlist_items WHERE user_id=$1 AND product_id=$2`, userID, productID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return errors.New("not found")
	}
	return nil
}

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo} }

// GetWishlist godoc
// @Summary      Get wishlist
// @Tags         wishlist
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /wishlist [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	items, cur, total, err := h.repo.List(r.Context(), middleware.GetUserID(r.Context()), 20, nil)
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	var cursor *string
	if cur != "" {
		cursor = &cur
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"product_id": it.ProductID, "name": it.Name, "base_price": it.BasePrice,
			"currency": it.Currency, "primary_image": it.PrimaryImage,
			"rating_avg": it.RatingAvg, "in_stock": it.InStock, "added_at": it.AddedAt,
		})
	}
	response.Paginated(w, out, &response.Meta{Cursor: cursor, HasMore: cur != "", Total: total})
}

// AddToWishlist godoc
// @Summary      Add to wishlist
// @Tags         wishlist
// @Security     BearerAuth
// @Accept       json
// @Success      201
// @Router       /wishlist [post]
func (h *Handler) Add(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req struct {
		ProductID string `json:"product_id" validate:"required"`
	}
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	_ = h.repo.Add(r.Context(), middleware.GetUserID(r.Context()), req.ProductID)
	w.WriteHeader(http.StatusCreated)
}

// RemoveFromWishlist godoc
// @Summary      Remove from wishlist
// @Tags         wishlist
// @Security     BearerAuth
// @Param        productId  path  string  true  "Product UUID"
// @Success      204
// @Router       /wishlist/{productId} [delete]
func (h *Handler) Remove(w http.ResponseWriter, r *http.Request) {
	_ = h.repo.Remove(r.Context(), middleware.GetUserID(r.Context()), chi.URLParam(r, "productId"))
	response.NoContent(w)
}

// Satisfy pgx import
var _ = pgx.ErrNoRows
var _ = context.Background
