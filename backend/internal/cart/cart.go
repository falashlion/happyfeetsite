package cart

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/happyfeet/api/pkg/cache"
	"github.com/happyfeet/api/pkg/database"
	"github.com/happyfeet/api/pkg/middleware"
	"github.com/happyfeet/api/pkg/response"
	"github.com/happyfeet/api/pkg/validator"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound         = errors.New("cart: not found")
	ErrInsufficientStock = errors.New("cart: insufficient stock")
	ErrInvalidCoupon    = errors.New("cart: invalid coupon")
)

// ── models ────────────────────────────────────────────────────────────────────

type Item struct {
	ID              string
	ProductID       string
	SKUID           string
	ProductName     string
	PrimaryImageURL string
	SizeEU          float64
	Color           string
	UnitPrice       float64
	Quantity        int
	StockAvailable  int
}

type Cart struct {
	ID             string
	UserID         *string
	SessionToken   *string
	Items          []Item
	CouponCode     *string
	DiscountAmount float64
	Currency       string
	UpdatedAt      time.Time
}

func (c *Cart) Subtotal() float64 {
	var t float64
	for _, it := range c.Items {
		t += it.UnitPrice * float64(it.Quantity)
	}
	return t
}
func (c *Cart) Total() float64 { return c.Subtotal() - c.DiscountAmount }
func (c *Cart) Count() int {
	n := 0
	for _, it := range c.Items {
		n += it.Quantity
	}
	return n
}

// ── repository ────────────────────────────────────────────────────────────────

type Repository struct {
	db    *database.DB
	cache *cache.Client
}

func NewRepository(db *database.DB, c *cache.Client) *Repository { return &Repository{db, c} }

func (r *Repository) ForUser(ctx context.Context, userID string) (*Cart, error) {
	cart, err := r.byUser(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		var id string
		if err2 := r.db.QueryRow(ctx, `INSERT INTO carts(user_id) VALUES($1) RETURNING id`, userID).Scan(&id); err2 != nil {
			return nil, err2
		}
		return &Cart{ID: id, UserID: &userID, Items: []Item{}, Currency: "XAF"}, nil
	}
	return cart, err
}

func (r *Repository) byUser(ctx context.Context, userID string) (*Cart, error) {
	c := &Cart{}
	err := r.db.QueryRow(ctx, `SELECT id,user_id,session_token,updated_at FROM carts WHERE user_id=$1`, userID).
		Scan(&c.ID, &c.UserID, &c.SessionToken, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.Items, _ = r.items(ctx, c.ID)
	c.Currency = "XAF"
	return c, nil
}

func (r *Repository) byID(ctx context.Context, id string) (*Cart, error) {
	c := &Cart{}
	err := r.db.QueryRow(ctx, `SELECT id,user_id,session_token,updated_at FROM carts WHERE id=$1`, id).
		Scan(&c.ID, &c.UserID, &c.SessionToken, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.Items, _ = r.items(ctx, c.ID)
	c.Currency = "XAF"
	return c, nil
}

func (r *Repository) items(ctx context.Context, cartID string) ([]Item, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ci.id,p.id,ci.sku_id,p.name,
		       COALESCE((SELECT url_thumbnail FROM product_images WHERE product_id=p.id AND is_primary LIMIT 1),''),
		       s.size_eu,s.color,ci.unit_price,ci.quantity,s.stock_qty
		FROM cart_items ci
		JOIN product_skus s ON s.id=ci.sku_id
		JOIN products p ON p.id=s.product_id
		WHERE ci.cart_id=$1 ORDER BY ci.created_at`, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.ProductID, &it.SKUID, &it.ProductName, &it.PrimaryImageURL,
			&it.SizeEU, &it.Color, &it.UnitPrice, &it.Quantity, &it.StockAvailable); err == nil {
			items = append(items, it)
		}
	}
	return items, nil
}

func (r *Repository) CartIDForUser(ctx context.Context, userID string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `SELECT id FROM carts WHERE user_id=$1`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

func (r *Repository) AddItem(ctx context.Context, cartID, skuID string, qty int) (*Cart, error) {
	var stock int
	var price float64
	err := r.db.QueryRow(ctx, `
		SELECT s.stock_qty,p.base_price+s.additional_price
		FROM product_skus s JOIN products p ON p.id=s.product_id
		WHERE s.id=$1 AND s.is_active=true AND p.status='active'`, skuID).Scan(&stock, &price)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if stock < qty {
		return nil, ErrInsufficientStock
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO cart_items(cart_id,sku_id,quantity,unit_price) VALUES($1,$2,$3,$4)
		ON CONFLICT(cart_id,sku_id) DO UPDATE SET quantity=LEAST(cart_items.quantity+$3,10),updated_at=NOW()`,
		cartID, skuID, qty, price)
	if err != nil {
		return nil, err
	}
	r.bust(ctx, cartID)
	return r.byID(ctx, cartID)
}

func (r *Repository) UpdateItem(ctx context.Context, cartID, itemID string, qty int) (*Cart, error) {
	res, err := r.db.Exec(ctx, `UPDATE cart_items SET quantity=$3,updated_at=NOW() WHERE id=$1 AND cart_id=$2`, itemID, cartID, qty)
	if err != nil {
		return nil, err
	}
	if res.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	r.bust(ctx, cartID)
	return r.byID(ctx, cartID)
}

func (r *Repository) RemoveItem(ctx context.Context, cartID, itemID string) (*Cart, error) {
	_, err := r.db.Exec(ctx, `DELETE FROM cart_items WHERE id=$1 AND cart_id=$2`, itemID, cartID)
	if err != nil {
		return nil, err
	}
	r.bust(ctx, cartID)
	return r.byID(ctx, cartID)
}

func (r *Repository) Clear(ctx context.Context, cartID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM cart_items WHERE cart_id=$1`, cartID)
	r.bust(ctx, cartID)
	return err
}

func (r *Repository) ApplyCoupon(ctx context.Context, cartID, code string) (*Cart, error) {
	var promoID string
	err := r.db.QueryRow(ctx, `
		SELECT id FROM promotions
		WHERE code=$1 AND is_active=true AND starts_at<=NOW() AND (expires_at IS NULL OR expires_at>NOW())`,
		code).Scan(&promoID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidCoupon
	}
	if err != nil {
		return nil, err
	}
	_, err = r.db.Exec(ctx, `UPDATE carts SET coupon_id=$2,updated_at=NOW() WHERE id=$1`, cartID, promoID)
	if err != nil {
		return nil, err
	}
	return r.byID(ctx, cartID)
}

func (r *Repository) RemoveCoupon(ctx context.Context, cartID string) (*Cart, error) {
	_, err := r.db.Exec(ctx, `UPDATE carts SET coupon_id=NULL,updated_at=NOW() WHERE id=$1`, cartID)
	if err != nil {
		return nil, err
	}
	return r.byID(ctx, cartID)
}

func (r *Repository) bust(ctx context.Context, cartID string) {
	_ = r.cache.Del(ctx, fmt.Sprintf("cart:%s", cartID))
}

// ── handler ───────────────────────────────────────────────────────────────────

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo} }

// GetCart godoc
// @Summary      Get cart
// @Tags         cart
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /cart [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	cart, err := h.repo.ForUser(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, fmtCart(cart))
}

// ClearCart godoc
// @Summary      Clear cart
// @Tags         cart
// @Security     BearerAuth
// @Success      204
// @Router       /cart [delete]
func (h *Handler) Clear(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	cartID, err := h.repo.CartIDForUser(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		response.NoContent(w)
		return
	}
	if err := h.repo.Clear(r.Context(), cartID); err != nil {
		response.InternalError(w, rid)
		return
	}
	response.NoContent(w)
}

type addItemReq struct {
	SKUID    string `json:"sku_id"   validate:"required"`
	Quantity int    `json:"quantity" validate:"required,gte=1,lte=10"`
}

// AddItem godoc
// @Summary      Add item to cart
// @Tags         cart
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  addItemReq  true  "Item to add"
// @Success      200  {object}  map[string]any
// @Failure      409  {object}  map[string]any
// @Router       /cart/items [post]
func (h *Handler) AddItem(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req addItemReq
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	cart, err := h.repo.ForUser(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	updated, err := h.repo.AddItem(r.Context(), cart.ID, req.SKUID, req.Quantity)
	switch {
	case errors.Is(err, ErrInsufficientStock):
		response.Err(w, http.StatusConflict, "INSUFFICIENT_STOCK", "Requested quantity exceeds available stock", rid)
	case errors.Is(err, ErrNotFound):
		response.NotFound(w, "SKU", rid)
	case err != nil:
		response.InternalError(w, rid)
	default:
		response.Ok(w, fmtCart(updated))
	}
}

type updateItemReq struct {
	Quantity int `json:"quantity" validate:"required,gte=1,lte=10"`
}

// UpdateItem godoc
// @Summary      Update cart item quantity
// @Tags         cart
// @Security     BearerAuth
// @Accept       json
// @Param        itemId  path  string  true  "Cart item UUID"
// @Success      200  {object}  map[string]any
// @Router       /cart/items/{itemId} [put]
func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req updateItemReq
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	cartID, err := h.repo.CartIDForUser(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		response.NotFound(w, "Cart", rid)
		return
	}
	updated, err := h.repo.UpdateItem(r.Context(), cartID, chi.URLParam(r, "itemId"), req.Quantity)
	if errors.Is(err, ErrNotFound) {
		response.NotFound(w, "Cart item", rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, fmtCart(updated))
}

// RemoveItem godoc
// @Summary      Remove cart item
// @Tags         cart
// @Security     BearerAuth
// @Param        itemId  path  string  true  "Cart item UUID"
// @Success      200  {object}  map[string]any
// @Router       /cart/items/{itemId} [delete]
func (h *Handler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	cartID, err := h.repo.CartIDForUser(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		response.NotFound(w, "Cart", rid)
		return
	}
	updated, err := h.repo.RemoveItem(r.Context(), cartID, chi.URLParam(r, "itemId"))
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, fmtCart(updated))
}

// ApplyCoupon godoc
// @Summary      Apply coupon
// @Tags         cart
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]any
// @Failure      422  {object}  map[string]any
// @Router       /cart/coupon [post]
func (h *Handler) ApplyCoupon(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req struct {
		CouponCode string `json:"coupon_code" validate:"required"`
	}
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	cartID, err := h.repo.CartIDForUser(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		response.NotFound(w, "Cart", rid)
		return
	}
	cart, err := h.repo.ApplyCoupon(r.Context(), cartID, req.CouponCode)
	if errors.Is(err, ErrInvalidCoupon) {
		response.Unprocessable(w, "Coupon is invalid, expired, or not applicable", rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, fmtCart(cart))
}

// RemoveCoupon godoc
// @Summary      Remove coupon
// @Tags         cart
// @Security     BearerAuth
// @Success      200  {object}  map[string]any
// @Router       /cart/coupon [delete]
func (h *Handler) RemoveCoupon(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	cartID, err := h.repo.CartIDForUser(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		response.NotFound(w, "Cart", rid)
		return
	}
	cart, err := h.repo.RemoveCoupon(r.Context(), cartID)
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, fmtCart(cart))
}

// ── formatter ─────────────────────────────────────────────────────────────────

func fmtCart(c *Cart) map[string]any {
	items := make([]map[string]any, 0, len(c.Items))
	for _, it := range c.Items {
		items = append(items, map[string]any{
			"id": it.ID, "product_id": it.ProductID, "sku_id": it.SKUID,
			"product_name": it.ProductName, "primary_image_url": it.PrimaryImageURL,
			"size_eu": it.SizeEU, "color": it.Color, "unit_price": it.UnitPrice,
			"quantity": it.Quantity, "subtotal": roundTo2(it.UnitPrice * float64(it.Quantity)),
			"stock_available": it.StockAvailable,
		})
	}
	return map[string]any{
		"id": c.ID, "items": items, "items_count": c.Count(),
		"subtotal": roundTo2(c.Subtotal()), "currency": c.Currency,
		"coupon_code": c.CouponCode, "discount_amount": c.DiscountAmount,
		"total": roundTo2(c.Total()), "updated_at": c.UpdatedAt,
	}
}

func roundTo2(f float64) float64 {
	v, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", f), 64)
	return v
}
