package order

// Admin-side order operations: the fulfilment queue the staff console reads,
// and the guarded status transitions it writes.
//
// Two rules shape everything here. A transition is only legal if the state
// machine below says so — the console is a convenience, never the authority.
// And every transition is one transaction: the row, its history entry and any
// stock movement commit together or not at all, with the customer email sent
// only once that commit has succeeded.

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/happyfeet/api/pkg/middleware"
	"github.com/happyfeet/api/pkg/response"
	"github.com/happyfeet/api/pkg/validator"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

var (
	ErrBadTransition = errors.New("order: status transition not allowed")
	ErrBadCursor     = errors.New("order: malformed cursor")
)

// ── the fulfilment state machine ──────────────────────────────────────────────

// transitions lists, for each current status, the statuses the console may move
// an order to. Anything absent is rejected — including transitions that are
// legal elsewhere in the system, such as the return flow, which has its own
// handling and must not be driveable from a dropdown.
//
// Terminal states (DELIVERED, RETURNED, REFUNDED) have no entry: once a parcel
// is in the customer's hands, correcting the record is a deliberate act that
// should leave a different kind of trail than a status dropdown.
var transitions = map[string][]string{
	"PLACED":     {"PROCESSING", "SHIPPED", "DELIVERED", "CANCELLED"},
	"PAID":       {"PROCESSING", "SHIPPED", "DELIVERED", "CANCELLED"},
	"PROCESSING": {"SHIPPED", "DELIVERED", "CANCELLED"},
	// Cash on delivery makes refusal at the door a routine outcome, so a
	// dispatched parcel can still be cancelled — which is what returns the
	// stock to the shelf.
	"SHIPPED": {"DELIVERED", "CANCELLED"},
}

// NextStatuses reports the legal moves from a status, for the console to render
// as options. The client gets the same list the server enforces, so the UI
// cannot offer a button that is guaranteed to 409.
func NextStatuses(from string) []string {
	next := transitions[from]
	out := make([]string, len(next))
	copy(out, next)
	return out
}

func canTransition(from, to string) bool {
	for _, s := range transitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// ── models ────────────────────────────────────────────────────────────────────

// AdminOrder is an order plus the contact details needed to act on it. A
// fulfilment queue without a phone number is not actionable.
type AdminOrder struct {
	Order
	Customer    Customer
	ShippedAt   *time.Time
	DeliveredAt *time.Time
}

// StatusEvent is one row of the audit trail behind an order.
type StatusEvent struct {
	From      *string
	To        string
	ChangedBy *string
	ActorName string
	Reason    *string
	CreatedAt time.Time
}

// AdminFilter narrows the fulfilment queue.
type AdminFilter struct {
	// Statuses is an OR set; empty means every status.
	Statuses []string
	// Search matches an order number, customer name, email or phone.
	Search string
	Limit  int
	Cursor string
}

// validStatuses guards the filter against arbitrary input reaching the enum
// cast. Postgres would reject an unknown label anyway, but as a 500 rather than
// the 400 the caller deserves.
var validStatuses = map[string]bool{
	"PLACED": true, "PAID": true, "PROCESSING": true, "SHIPPED": true,
	"DELIVERED": true, "CANCELLED": true, "RETURN_REQUESTED": true,
	"RETURNED": true, "REFUNDED": true,
}

// ── cursor ────────────────────────────────────────────────────────────────────
//
// Keyset pagination on (placed_at, id). Ordering by placed_at while paginating
// on id alone would skip and repeat rows, because order ids are random UUIDs
// with no relationship to time.

func encodeCursor(placedAt time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(placedAt.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func decodeCursor(s string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, "", ErrBadCursor
	}
	at, id, ok := strings.Cut(string(raw), "|")
	if !ok {
		return time.Time{}, "", ErrBadCursor
	}
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return time.Time{}, "", ErrBadCursor
	}
	return t, id, nil
}

// ── repository ────────────────────────────────────────────────────────────────

const adminOrderCols = orderCols + `,o.shipped_at,o.delivered_at,
	TRIM(CONCAT(u.first_name,' ',u.last_name)),u.email,u.phone`

func scanAdminOrder(row pgx.Row) (*AdminOrder, error) {
	a := &AdminOrder{}
	o := &a.Order
	var email, phone *string
	err := row.Scan(&o.ID, &o.OrderNumber, &o.UserID, &o.Status,
		&o.DeliveryAddress, &o.DeliveryMethod, &o.DeliveryFee,
		&o.EstimatedDelivery, &o.TrackingNumber, &o.TrackingURL,
		&o.Subtotal, &o.DiscountAmount, &o.TotalAmount, &o.Currency,
		&o.PaymentMethod, &o.PlacedAt, &o.UpdatedAt,
		&a.ShippedAt, &a.DeliveredAt,
		&a.Customer.Name, &email, &phone)
	if err != nil {
		return nil, err
	}
	if email != nil {
		a.Customer.Email = *email
	}
	if phone != nil {
		a.Customer.Phone = *phone
	}
	return a, nil
}

// AdminList returns the fulfilment queue, newest first.
func (r *Repository) AdminList(ctx context.Context, f AdminFilter) ([]*AdminOrder, string, int64, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 25
	}

	var args []any
	conds := []string{"TRUE"}

	if len(f.Statuses) > 0 {
		args = append(args, f.Statuses)
		conds = append(conds, fmt.Sprintf("o.status::text = ANY($%d)", len(args)))
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		args = append(args, "%"+s+"%")
		conds = append(conds, fmt.Sprintf(
			`(o.order_number ILIKE $%[1]d
			  OR TRIM(CONCAT(u.first_name,' ',u.last_name)) ILIKE $%[1]d
			  OR u.email ILIKE $%[1]d
			  OR u.phone ILIKE $%[1]d)`, len(args)))
	}

	// COUNT runs against the filters but not the cursor: the total is the size
	// of the whole filtered queue, not of the page the caller is standing on.
	where := strings.Join(conds, " AND ")
	var total int64
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders o JOIN users u ON u.id=o.user_id WHERE `+where,
		args...).Scan(&total); err != nil {
		return nil, "", 0, err
	}

	if f.Cursor != "" {
		at, id, err := decodeCursor(f.Cursor)
		if err != nil {
			return nil, "", 0, err
		}
		args = append(args, at, id)
		conds = append(conds, fmt.Sprintf("(o.placed_at,o.id) < ($%d,$%d)", len(args)-1, len(args)))
	}
	args = append(args, f.Limit+1)

	rows, err := r.db.Query(ctx, fmt.Sprintf(
		`SELECT %s FROM orders o JOIN users u ON u.id=o.user_id
		 WHERE %s ORDER BY o.placed_at DESC, o.id DESC LIMIT $%d`,
		adminOrderCols, strings.Join(conds, " AND "), len(args)), args...)
	if err != nil {
		return nil, "", 0, err
	}
	defer rows.Close()

	var orders []*AdminOrder
	for rows.Next() {
		a, err := scanAdminOrder(rows)
		if err != nil {
			return nil, "", 0, err
		}
		orders = append(orders, a)
	}
	if err := rows.Err(); err != nil {
		return nil, "", 0, err
	}

	// One extra row was requested purely to learn whether another page exists.
	var cursor string
	if len(orders) > f.Limit {
		orders = orders[:f.Limit]
		last := orders[len(orders)-1]
		cursor = encodeCursor(last.PlacedAt, last.ID)
	}

	if err := r.attachItems(ctx, orders); err != nil {
		return nil, "", 0, err
	}
	return orders, cursor, total, nil
}

// attachItems loads the lines for a whole page in one round trip. Per-order
// queries would make a 25-row queue cost 26 statements.
func (r *Repository) attachItems(ctx context.Context, orders []*AdminOrder) error {
	if len(orders) == 0 {
		return nil
	}
	byID := make(map[string]*AdminOrder, len(orders))
	ids := make([]string, 0, len(orders))
	for _, o := range orders {
		byID[o.ID] = o
		ids = append(ids, o.ID)
	}

	rows, err := r.db.Query(ctx, `
		SELECT oi.order_id,oi.id,oi.product_id,oi.sku_id,oi.product_name,
		       COALESCE((SELECT url_thumbnail FROM product_images WHERE product_id=oi.product_id AND is_primary LIMIT 1),''),
		       oi.size_eu,oi.color,oi.quantity,oi.unit_price,oi.subtotal
		FROM order_items oi WHERE oi.order_id = ANY($1)
		ORDER BY oi.created_at`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var orderID string
		var it Item
		if err := rows.Scan(&orderID, &it.ID, &it.ProductID, &it.SKUID, &it.ProductName,
			&it.PrimaryImageURL, &it.SizeEU, &it.Color, &it.Quantity, &it.UnitPrice, &it.Subtotal); err != nil {
			return err
		}
		if o := byID[orderID]; o != nil {
			o.Items = append(o.Items, it)
		}
	}
	return rows.Err()
}

// AdminGetByID loads one order for the console, with no user_id restriction —
// staff act on other people's orders by definition. The route is role-gated.
func (r *Repository) AdminGetByID(ctx context.Context, id string) (*AdminOrder, error) {
	a, err := scanAdminOrder(r.db.QueryRow(ctx, fmt.Sprintf(
		`SELECT %s FROM orders o JOIN users u ON u.id=o.user_id WHERE o.id=$1`,
		adminOrderCols), id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.Items, _ = r.items(ctx, a.ID)
	return a, nil
}

// History returns the audit trail for an order, newest first.
func (r *Repository) History(ctx context.Context, orderID string) ([]StatusEvent, error) {
	rows, err := r.db.Query(ctx, `
		SELECT h.from_status::text,h.to_status::text,h.changed_by,
		       COALESCE(TRIM(CONCAT(u.first_name,' ',u.last_name)),''),
		       h.reason,h.created_at
		FROM order_status_history h
		LEFT JOIN users u ON u.id=h.changed_by
		WHERE h.order_id=$1 ORDER BY h.created_at DESC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []StatusEvent
	for rows.Next() {
		var e StatusEvent
		if err := rows.Scan(&e.From, &e.To, &e.ChangedBy, &e.ActorName, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// StatusChange is one transition requested by a member of staff.
type StatusChange struct {
	OrderID string
	To      string
	ActorID string
	Reason  string
}

// UpdateStatus moves an order along the fulfilment path.
//
// It reports whether anything actually changed: re-sending the same status is
// treated as a no-op rather than an error, so a double-clicked button or a
// retried request cannot write a second history row or send a second email.
func (r *Repository) UpdateStatus(ctx context.Context, p StatusChange) (changed bool, err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// FOR UPDATE: two members of staff pressing "Delivered" at the same moment
	// must serialise here, so exactly one of them writes the transition.
	var from string
	err = tx.QueryRow(ctx, `SELECT status::text FROM orders WHERE id=$1 FOR UPDATE`, p.OrderID).Scan(&from)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	if from == p.To {
		return false, nil
	}
	if !canTransition(from, p.To) {
		return false, ErrBadTransition
	}

	// The two timestamp columns are set from the database clock, and only on
	// the first arrival at that status, so a later correction cannot rewrite
	// when the parcel actually moved.
	if _, err = tx.Exec(ctx, `
		UPDATE orders SET
			status       = $2::order_status,
			updated_at   = NOW(),
			shipped_at   = CASE WHEN $2::text = 'SHIPPED'   THEN COALESCE(shipped_at, NOW())   ELSE shipped_at   END,
			delivered_at = CASE WHEN $2::text = 'DELIVERED' THEN COALESCE(delivered_at, NOW()) ELSE delivered_at END
		WHERE id = $1`, p.OrderID, p.To); err != nil {
		return false, err
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO order_status_history(order_id,from_status,to_status,changed_by,reason)
		VALUES($1,$2::order_status,$3::order_status,$4,NULLIF($5,''))`,
		p.OrderID, from, p.To, p.ActorID, strings.TrimSpace(p.Reason)); err != nil {
		return false, err
	}

	// Cancelling returns the pairs to the shelf. Checkout took the stock when
	// the order was created, so without this a refused delivery would leave the
	// catalogue permanently short.
	if p.To == "CANCELLED" {
		if _, err = tx.Exec(ctx, `
			UPDATE product_skus s
			SET stock_qty = s.stock_qty + oi.quantity, updated_at = NOW()
			FROM order_items oi
			WHERE oi.order_id = $1 AND s.id = oi.sku_id`, p.OrderID); err != nil {
			return false, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// ── handler ───────────────────────────────────────────────────────────────────

// AdminList godoc
// @Summary      List orders for the staff console (admin)
// @Tags         admin
// @Security     BearerAuth
// @Produce      json
// @Param        status  query  string  false  "Comma-separated statuses, or 'all'"
// @Param        q       query  string  false  "Order number, customer name, email or phone"
// @Param        cursor  query  string  false  "Opaque keyset cursor from the previous page"
// @Param        limit   query  int     false  "Page size, 1-100 (default 25)"
// @Success      200  {object}  map[string]any
// @Router       /admin/orders [get]
func (h *Handler) AdminList(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	q := r.URL.Query()

	f := AdminFilter{
		Search: strings.TrimSpace(q.Get("q")),
		Cursor: q.Get("cursor"),
		Limit:  25,
	}
	if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 {
		f.Limit = n // the repository clamps the upper bound
	}
	if s := q.Get("status"); s != "" && s != "all" {
		for _, part := range strings.Split(s, ",") {
			part = strings.ToUpper(strings.TrimSpace(part))
			if !validStatuses[part] {
				response.BadRequest(w, "Unknown order status: "+part, rid)
				return
			}
			f.Statuses = append(f.Statuses, part)
		}
	}

	orders, cursor, total, err := h.repo.AdminList(r.Context(), f)
	if errors.Is(err, ErrBadCursor) {
		response.BadRequest(w, "Malformed cursor", rid)
		return
	}
	if err != nil {
		log.Error().Err(err).Str("rid", rid).Msg("admin list orders")
		response.InternalError(w, rid)
		return
	}

	out := make([]map[string]any, 0, len(orders))
	for _, o := range orders {
		out = append(out, fmtAdminOrder(o, nil))
	}
	var cur *string
	if cursor != "" {
		cur = &cursor
	}
	response.Paginated(w, out, &response.Meta{Cursor: cur, HasMore: cursor != "", Total: total})
}

// AdminGet godoc
// @Summary      Get one order with its status history (admin)
// @Tags         admin
// @Security     BearerAuth
// @Produce      json
// @Param        id  path  string  true  "Order UUID"
// @Success      200  {object}  map[string]any
// @Failure      404  {object}  map[string]any
// @Router       /admin/orders/{id} [get]
func (h *Handler) AdminGet(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	o, err := h.repo.AdminGetByID(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, ErrNotFound) {
		response.NotFound(w, "Order", rid)
		return
	}
	if err != nil {
		log.Error().Err(err).Str("rid", rid).Msg("admin get order")
		response.InternalError(w, rid)
		return
	}
	events, _ := h.repo.History(r.Context(), o.ID)
	response.Ok(w, fmtAdminOrder(o, events))
}

type updateStatusReq struct {
	Status string `json:"status" validate:"required,oneof=PROCESSING SHIPPED DELIVERED CANCELLED"`
	Reason string `json:"reason" validate:"omitempty,max=500"`
}

// AdminUpdateStatus godoc
// @Summary      Move an order along the fulfilment path (admin)
// @Tags         admin
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  string           true  "Order UUID"
// @Param        body  body  updateStatusReq  true  "Target status"
// @Success      200  {object}  map[string]any
// @Failure      409  {object}  map[string]any
// @Router       /admin/orders/{id}/status [patch]
func (h *Handler) AdminUpdateStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rid := middleware.GetRequestID(ctx)
	id := chi.URLParam(r, "id")

	var req updateStatusReq
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}

	changed, err := h.repo.UpdateStatus(ctx, StatusChange{
		OrderID: id,
		To:      req.Status,
		ActorID: middleware.GetUserID(ctx),
		Reason:  req.Reason,
	})
	switch {
	case errors.Is(err, ErrNotFound):
		response.NotFound(w, "Order", rid)
		return
	case errors.Is(err, ErrBadTransition):
		response.Err(w, http.StatusConflict, "INVALID_TRANSITION",
			"That status change is not allowed from the order's current state", rid)
		return
	case err != nil:
		log.Error().Err(err).Str("rid", rid).Str("order", id).Msg("admin update order status")
		response.InternalError(w, rid)
		return
	}

	o, err := h.repo.AdminGetByID(ctx, id)
	if err != nil {
		log.Error().Err(err).Str("rid", rid).Msg("reload order after status change")
		response.InternalError(w, rid)
		return
	}
	events, _ := h.repo.History(ctx, id)

	// The thank-you goes out only on a real transition into DELIVERED, and only
	// after the transaction above committed — a rolled-back update must never
	// leave a customer holding an email about it.
	var forward string
	if changed && req.Status == "DELIVERED" {
		forward = h.notifyOrderDelivered(o)
	}

	body := fmtAdminOrder(o, events)
	body["changed"] = changed
	// Present whenever we can reach the customer on WhatsApp, so staff can
	// forward the same thank-you by hand — the usual fallback here, where many
	// customers have a phone but no email.
	body["whatsapp_forward_url"] = forward
	response.Ok(w, body)
}

// notifyOrderDelivered sends the delivery thank-you and returns the WhatsApp
// deep link carrying the same message. Mail failures are logged and swallowed
// inside the notifier: a relay problem must not undo a delivered parcel.
func (h *Handler) notifyOrderDelivered(o *AdminOrder) string {
	if h.notify == nil {
		return ""
	}
	view := h.orderEmail(&o.Order, o.Customer)
	h.notify.SendOrderDelivered(view)
	return h.notify.ForwardLink(view)
}

// ── formatter ─────────────────────────────────────────────────────────────────

func fmtAdminOrder(o *AdminOrder, events []StatusEvent) map[string]any {
	m := fmtOrder(&o.Order)
	m["customer"] = map[string]any{
		"name":  o.Customer.Name,
		"email": o.Customer.Email,
		"phone": o.Customer.Phone,
	}
	m["shipped_at"] = o.ShippedAt
	m["delivered_at"] = o.DeliveredAt
	m["next_statuses"] = NextStatuses(o.Status)

	if events != nil {
		hist := make([]map[string]any, 0, len(events))
		for _, e := range events {
			hist = append(hist, map[string]any{
				"from_status": e.From,
				"to_status":   e.To,
				"changed_by":  e.ChangedBy,
				"actor_name":  e.ActorName,
				"reason":      e.Reason,
				"created_at":  e.CreatedAt,
			})
		}
		m["history"] = hist
	}
	return m
}
