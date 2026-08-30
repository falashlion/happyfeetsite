package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/happyfeet/api/internal/notification"
	"github.com/happyfeet/api/pkg/database"
	"github.com/happyfeet/api/pkg/middleware"
	"github.com/happyfeet/api/pkg/response"
	"github.com/happyfeet/api/pkg/validator"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound      = errors.New("order: not found")
	ErrCannotCancel  = errors.New("order: cannot cancel at this stage")
	ErrEmptyCart     = errors.New("order: cart is empty")
	ErrAddrNotFound  = errors.New("order: address not found")
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
	Quantity        int
	UnitPrice       float64
	Subtotal        float64
}

type Order struct {
	ID                  string
	OrderNumber         string
	UserID              string
	Status              string
	Items               []Item
	DeliveryAddress     json.RawMessage
	DeliveryMethod      string
	DeliveryFee         float64
	EstimatedDelivery   *time.Time
	TrackingNumber      *string
	TrackingURL         *string
	Subtotal            float64
	DiscountAmount      float64
	TotalAmount         float64
	Currency            string
	PaymentMethod       string
	PlacedAt            time.Time
	UpdatedAt           time.Time
}

// ── repository ────────────────────────────────────────────────────────────────

type Repository struct{ db *database.DB }

func NewRepository(db *database.DB) *Repository { return &Repository{db} }

const orderCols = `o.id,o.order_number,o.user_id,o.status::text,
	o.delivery_address,o.delivery_method::text,o.delivery_fee,
	o.estimated_delivery_date,o.tracking_number,o.tracking_url,
	o.subtotal,o.discount_amount,o.total_amount,o.currency::text,
	o.payment_method::text,o.placed_at,o.updated_at`

func scanOrder(row pgx.Row, o *Order) error {
	return row.Scan(&o.ID, &o.OrderNumber, &o.UserID, &o.Status,
		&o.DeliveryAddress, &o.DeliveryMethod, &o.DeliveryFee,
		&o.EstimatedDelivery, &o.TrackingNumber, &o.TrackingURL,
		&o.Subtotal, &o.DiscountAmount, &o.TotalAmount, &o.Currency,
		&o.PaymentMethod, &o.PlacedAt, &o.UpdatedAt)
}

func (r *Repository) List(ctx context.Context, userID, status string, limit int, cursor *string) ([]*Order, string, int64, error) {
	args := []any{userID, limit + 1}
	cond := "o.user_id=$1"
	if status != "" {
		args = append(args, status)
		cond += fmt.Sprintf(" AND o.status=$%d", len(args))
	}
	if cursor != nil {
		args = append(args, *cursor)
		cond += fmt.Sprintf(" AND o.id<$%d", len(args))
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(
		`SELECT %s FROM orders o WHERE %s ORDER BY o.placed_at DESC LIMIT $2`, orderCols, cond), args...)
	if err != nil {
		return nil, "", 0, err
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		o := &Order{}
		if err := scanOrder(rows, o); err == nil {
			orders = append(orders, o)
		}
	}
	var cur string
	if len(orders) > limit {
		orders = orders[:limit]
		cur = orders[len(orders)-1].ID
	}
	var total int64
	_ = r.db.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM orders o WHERE %s", cond), args[:1]...).Scan(&total)
	return orders, cur, total, nil
}

func (r *Repository) GetByID(ctx context.Context, id, userID string) (*Order, error) {
	o := &Order{}
	err := scanOrder(r.db.QueryRow(ctx, fmt.Sprintf(
		`SELECT %s FROM orders o WHERE o.id=$1 AND o.user_id=$2`, orderCols), id, userID), o)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	o.Items, _ = r.items(ctx, o.ID)
	return o, nil
}

func (r *Repository) GetByNumber(ctx context.Context, number, userID string) (*Order, error) {
	o := &Order{}
	err := scanOrder(r.db.QueryRow(ctx, fmt.Sprintf(
		`SELECT %s FROM orders o WHERE o.order_number=$1 AND o.user_id=$2`, orderCols), number, userID), o)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	o.Items, _ = r.items(ctx, o.ID)
	return o, nil
}

func (r *Repository) items(ctx context.Context, orderID string) ([]Item, error) {
	rows, err := r.db.Query(ctx, `
		SELECT oi.id,oi.product_id,oi.sku_id,oi.product_name,
		       COALESCE((SELECT url_thumbnail FROM product_images WHERE product_id=oi.product_id AND is_primary LIMIT 1),''),
		       oi.size_eu,oi.color,oi.quantity,oi.unit_price,oi.subtotal
		FROM order_items oi WHERE oi.order_id=$1`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.ProductID, &it.SKUID, &it.ProductName,
			&it.PrimaryImageURL, &it.SizeEU, &it.Color, &it.Quantity, &it.UnitPrice, &it.Subtotal); err == nil {
			items = append(items, it)
		}
	}
	return items, nil
}

type CreateParams struct {
	UserID         string
	CartID         string
	AddressID      string
	DeliveryMethod string
	PaymentMethod  string
}

func (r *Repository) CreateFromCart(ctx context.Context, p CreateParams) (*Order, error) {
	// Fetch address as JSON
	var addrJSON json.RawMessage
	err := r.db.QueryRow(ctx,
		`SELECT row_to_json(a) FROM user_addresses a WHERE a.id=$1 AND a.user_id=$2`, p.AddressID, p.UserID).Scan(&addrJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAddrNotFound
	}
	if err != nil {
		return nil, err
	}

	// Fetch cart items
	rows, err := r.db.Query(ctx, `
		SELECT ci.sku_id,ci.quantity,ci.unit_price,p.id,p.name,s.size_eu,s.color
		FROM cart_items ci
		JOIN product_skus s ON s.id=ci.sku_id
		JOIN products p ON p.id=s.product_id
		WHERE ci.cart_id=$1`, p.CartID)
	if err != nil {
		return nil, err
	}
	type ci struct{ skuID, productID, name, color string; qty int; price, sizeEU float64 }
	var items []ci
	var sub float64
	for rows.Next() {
		var it ci
		if err := rows.Scan(&it.skuID, &it.qty, &it.price, &it.productID, &it.name, &it.sizeEU, &it.color); err == nil {
			sub += it.price * float64(it.qty)
			items = append(items, it)
		}
	}
	rows.Close()
	if len(items) == 0 {
		return nil, ErrEmptyCart
	}

	deliveryFee := deliveryFeeFor(p.DeliveryMethod, sub)
	total := sub + deliveryFee

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orderID string
	err = tx.QueryRow(ctx, `
		INSERT INTO orders(user_id,delivery_address,delivery_method,delivery_fee,payment_method,subtotal,total_amount,currency)
		VALUES($1,$2,$3::delivery_method,$4,$5::payment_provider,$6,$7,'XAF') RETURNING id`,
		p.UserID, addrJSON, p.DeliveryMethod, deliveryFee, p.PaymentMethod, sub, total).Scan(&orderID)
	if err != nil {
		return nil, err
	}

	for _, it := range items {
		_, err = tx.Exec(ctx, `
			INSERT INTO order_items(order_id,product_id,sku_id,product_name,sku_code,size_eu,color,quantity,unit_price,subtotal)
			VALUES($1,$2,$3,$4,'N/A',$5,$6,$7,$8,$9)`,
			orderID, it.productID, it.skuID, it.name, it.sizeEU, it.color, it.qty, it.price, it.price*float64(it.qty))
		if err != nil {
			return nil, err
		}
	}

	_, _ = tx.Exec(ctx, `DELETE FROM cart_items WHERE cart_id=$1`, p.CartID)

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, orderID, p.UserID)
}

// deliveryFeeFor mirrors the frontend's pricing table:
//   STANDARD   2,000 (free over 50,000)
//   EXPRESS    4,000
//   SAME_DAY   8,000
func deliveryFeeFor(method string, subtotal float64) float64 {
	switch method {
	case "EXPRESS":
		return 4000
	case "SAME_DAY":
		return 8000
	default: // STANDARD
		if subtotal >= 50000 {
			return 0
		}
		return 2000
	}
}

func (r *Repository) Cancel(ctx context.Context, id, userID string) (*Order, error) {
	var status string
	err := r.db.QueryRow(ctx, `SELECT status FROM orders WHERE id=$1 AND user_id=$2`, id, userID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if status != "PLACED" && status != "PAID" {
		return nil, ErrCannotCancel
	}
	_, err = r.db.Exec(ctx, `UPDATE orders SET status='CANCELLED',updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id, userID)
}

// ── customer snapshot for notifications ───────────────────────────────────────

// Customer is the contact detail needed to notify both sides about an order.
type Customer struct {
	Name  string
	Email string
	Phone string
}

func (r *Repository) Customer(ctx context.Context, userID string) (Customer, error) {
	var c Customer
	var email, phone *string
	err := r.db.QueryRow(ctx,
		`SELECT TRIM(CONCAT(first_name,' ',last_name)),email,phone FROM users WHERE id=$1`,
		userID).Scan(&c.Name, &email, &phone)
	if email != nil {
		c.Email = *email
	}
	if phone != nil {
		c.Phone = *phone
	}
	return c, err
}

// ── handler ───────────────────────────────────────────────────────────────────

// Notifier is the slice of the notification service the order flow needs. It is
// declared here (rather than imported as a concrete type) so orders can be
// tested without a mail relay, and so a nil notifier is a valid no-op.
type Notifier interface {
	SendOrderPlaced(notification.OrderEmail)
}

type Handler struct {
	repo   *Repository
	notify Notifier
}

func NewHandler(repo *Repository, notify Notifier) *Handler { return &Handler{repo, notify} }

// ListOrders godoc
// @Summary      List user's orders
// @Tags         orders
// @Security     BearerAuth
// @Produce      json
// @Param        status  query  string  false  "Filter by status"
// @Success      200  {object}  map[string]any
// @Router       /orders [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	orders, cur, total, err := h.repo.List(r.Context(), middleware.GetUserID(r.Context()), r.URL.Query().Get("status"), 20, nil)
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	var cursor *string
	if cur != "" {
		cursor = &cur
	}
	response.Paginated(w, fmtOrders(orders), &response.Meta{Cursor: cursor, HasMore: cur != "", Total: total})
}

// GetOrder godoc
// @Summary      Get order detail
// @Tags         orders
// @Security     BearerAuth
// @Produce      json
// @Param        id  path  string  true  "Order UUID"
// @Success      200  {object}  map[string]any
// @Failure      404  {object}  map[string]any
// @Router       /orders/{id} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	o, err := h.repo.GetByID(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()))
	if errors.Is(err, ErrNotFound) {
		response.NotFound(w, "Order", rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, fmtOrder(o))
}

// GetOrderByNumber godoc
// @Summary      Get order detail by human-readable order number
// @Tags         orders
// @Security     BearerAuth
// @Produce      json
// @Param        number  path  string  true  "Order number e.g. HF-2026-00012847"
// @Success      200  {object}  map[string]any
// @Failure      404  {object}  map[string]any
// @Router       /orders/by-number/{number} [get]
func (h *Handler) GetByNumber(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	o, err := h.repo.GetByNumber(r.Context(), chi.URLParam(r, "number"), middleware.GetUserID(r.Context()))
	if errors.Is(err, ErrNotFound) {
		response.NotFound(w, "Order", rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, fmtOrder(o))
}

type createOrderReq struct {
	DeliveryAddressID string `json:"delivery_address_id" validate:"required"`
	DeliveryMethod    string `json:"delivery_method"     validate:"required,oneof=STANDARD EXPRESS SAME_DAY"`
	PaymentMethod     string `json:"payment_method"      validate:"required,oneof=mtn_momo orange_money stripe cash_on_delivery"`
}

// CreateOrder godoc
// @Summary      Create order from cart
// @Tags         orders
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string          true  "Idempotency key (UUID)"
// @Param        body            body    createOrderReq  true  "Order request"
// @Success      201  {object}  map[string]any
// @Router       /orders [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	userID := middleware.GetUserID(r.Context())
	var req createOrderReq
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}

	var cartID string
	err := h.repo.db.QueryRow(r.Context(), `SELECT id FROM carts WHERE user_id=$1`, userID).Scan(&cartID)
	if errors.Is(err, pgx.ErrNoRows) {
		response.BadRequest(w, "Cart is empty", rid)
		return
	}

	o, err := h.repo.CreateFromCart(r.Context(), CreateParams{
		UserID: userID, CartID: cartID,
		AddressID: req.DeliveryAddressID, DeliveryMethod: req.DeliveryMethod, PaymentMethod: req.PaymentMethod,
	})
	switch {
	case errors.Is(err, ErrAddrNotFound):
		response.NotFound(w, "Address", rid)
	case errors.Is(err, ErrEmptyCart):
		response.BadRequest(w, "Cart is empty", rid)
	case err != nil:
		response.InternalError(w, rid)
	default:
		h.notifyOrderPlaced(r.Context(), o, userID)
		response.Created(w, map[string]any{"order": fmtOrder(o), "payment": nil})
	}
}

// notifyOrderPlaced builds the email view model and hands it to the notifier.
// Every failure here is logged-and-swallowed by the notifier: a mail problem
// must never turn a successfully placed order into an error for the customer.
func (h *Handler) notifyOrderPlaced(ctx context.Context, o *Order, userID string) {
	if h.notify == nil {
		return
	}
	customer, _ := h.repo.Customer(ctx, userID)
	addr := parseAddress(o.DeliveryAddress)

	// The address snapshot is the authority on who receives the parcel; the
	// account record only fills the gaps.
	name := addr.RecipientName
	if name == "" {
		name = customer.Name
	}
	phone := addr.RecipientPhone
	if phone == "" {
		phone = customer.Phone
	}

	lines := make([]notification.OrderLine, 0, len(o.Items))
	count := 0
	for _, it := range o.Items {
		count += it.Quantity
		lines = append(lines, notification.OrderLine{
			Name:     it.ProductName,
			Variant:  fmt.Sprintf("EU %s · %s", trimSize(it.SizeEU), it.Color),
			Quantity: it.Quantity,
			Amount:   notification.FormatMoney(it.Subtotal, o.Currency),
		})
	}

	paymentLabel, paymentNote := notification.PaymentLabel(o.PaymentMethod)
	discount := ""
	if o.DiscountAmount > 0 {
		discount = notification.FormatMoney(o.DiscountAmount, o.Currency)
	}
	deliveryFee := "Complimentary"
	if o.DeliveryFee > 0 {
		deliveryFee = notification.FormatMoney(o.DeliveryFee, o.Currency)
	}

	h.notify.SendOrderPlaced(notification.OrderEmail{
		OrderNumber:   o.OrderNumber,
		PlacedAt:      notification.FormatDate(o.PlacedAt),
		Status:        o.Status,
		PaymentLabel:  paymentLabel,
		PaymentNote:   paymentNote,
		DeliveryLabel: notification.DeliveryLabel(o.DeliveryMethod),

		CustomerName:  name,
		CustomerEmail: customer.Email,
		CustomerPhone: phone,
		AddressLines:  addr.Lines(),

		Lines:       lines,
		ItemCount:   count,
		Subtotal:    notification.FormatMoney(o.Subtotal, o.Currency),
		Discount:    discount,
		DeliveryFee: deliveryFee,
		Total:       notification.FormatMoney(o.TotalAmount, o.Currency),
	})
}

// address mirrors the JSONB snapshot stored on the order.
type address struct {
	Label          string `json:"label"`
	RecipientName  string `json:"recipient_name"`
	RecipientPhone string `json:"recipient_phone"`
	Street         string `json:"street"`
	City           string `json:"city"`
	State          string `json:"state"`
	PostalCode     string `json:"postal_code"`
	Country        string `json:"country"`
}

// Lines renders the address as display rows, skipping empty components.
func (a address) Lines() []string {
	var out []string
	if a.Street != "" {
		out = append(out, a.Street)
	}
	cityLine := strings.TrimSpace(strings.Join(nonEmpty(a.City, a.State, a.PostalCode), ", "))
	if cityLine != "" {
		out = append(out, cityLine)
	}
	if a.Country != "" {
		out = append(out, countryName(a.Country))
	}
	if len(out) == 0 {
		out = append(out, "Address on file")
	}
	return out
}

func parseAddress(raw json.RawMessage) address {
	var a address
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &a)
	}
	return a
}

func nonEmpty(vals ...string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

// countryName expands the ISO codes the storefront actually offers; anything
// else passes through as the raw code.
func countryName(code string) string {
	switch strings.ToUpper(code) {
	case "CM":
		return "Cameroon"
	case "NG":
		return "Nigeria"
	case "GA":
		return "Gabon"
	case "TD":
		return "Chad"
	case "CF":
		return "Central African Republic"
	case "GQ":
		return "Equatorial Guinea"
	default:
		return strings.ToUpper(code)
	}
}

// trimSize renders EU sizes without a trailing ".0" while keeping half sizes.
func trimSize(size float64) string {
	s := strconv.FormatFloat(size, 'f', -1, 64)
	return s
}

// CancelOrder godoc
// @Summary      Cancel order
// @Tags         orders
// @Security     BearerAuth
// @Param        id  path  string  true  "Order UUID"
// @Success      200  {object}  map[string]any
// @Failure      409  {object}  map[string]any
// @Router       /orders/{id}/cancel [post]
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	o, err := h.repo.Cancel(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()))
	switch {
	case errors.Is(err, ErrNotFound):
		response.NotFound(w, "Order", rid)
	case errors.Is(err, ErrCannotCancel):
		response.Err(w, http.StatusConflict, "CANNOT_CANCEL", "Order cannot be cancelled at this stage", rid)
	case err != nil:
		response.InternalError(w, rid)
	default:
		response.Ok(w, fmtOrder(o))
	}
}

// ReturnOrder godoc
// @Summary      Submit return request
// @Tags         orders
// @Security     BearerAuth
// @Param        id  path  string  true  "Order UUID"
// @Success      201  {object}  map[string]any
// @Router       /orders/{id}/return [post]
func (h *Handler) Return(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req struct {
		Reason      string `json:"reason" validate:"required,oneof=wrong_size damaged not_as_described changed_mind other"`
		Description string `json:"description"`
	}
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{
		"return_id":    "ret_" + chi.URLParam(r, "id")[:8],
		"status":       "PENDING_REVIEW",
		"instructions": "Our team will review your request within 24 hours.",
	})
}

// ── formatters ────────────────────────────────────────────────────────────────

func fmtOrders(orders []*Order) []map[string]any {
	out := make([]map[string]any, 0, len(orders))
	for _, o := range orders {
		out = append(out, fmtOrder(o))
	}
	return out
}

func fmtOrder(o *Order) map[string]any {
	items := make([]map[string]any, 0, len(o.Items))
	for _, it := range o.Items {
		items = append(items, map[string]any{
			"id": it.ID, "product_id": it.ProductID, "sku_id": it.SKUID,
			"product_name": it.ProductName, "primary_image_url": it.PrimaryImageURL,
			"size_eu": it.SizeEU, "color": it.Color,
			"quantity": it.Quantity, "unit_price": it.UnitPrice, "subtotal": it.Subtotal,
		})
	}
	return map[string]any{
		"id": o.ID, "order_number": o.OrderNumber, "status": o.Status, "items": items,
		"delivery_address": o.DeliveryAddress, "delivery_method": o.DeliveryMethod,
		"delivery_fee": o.DeliveryFee, "estimated_delivery_date": o.EstimatedDelivery,
		"tracking_number": o.TrackingNumber, "tracking_url": o.TrackingURL,
		"subtotal": o.Subtotal, "discount_amount": o.DiscountAmount, "total_amount": o.TotalAmount,
		"currency": o.Currency, "payment_method": o.PaymentMethod,
		"placed_at": o.PlacedAt, "updated_at": o.UpdatedAt,
	}
}
