package admin

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

// ── models ────────────────────────────────────────────────────────────────────

type UserRow struct {
	ID        string
	Email     *string
	Phone     *string
	FirstName string
	LastName  string
	Role      string
	Status    string
	CreatedAt time.Time
}

// ── repository ────────────────────────────────────────────────────────────────

type Repository struct{ db *database.DB }

func NewRepository(db *database.DB) *Repository { return &Repository{db} }

func (r *Repository) ListUsers(ctx context.Context, search, role, status string, limit int, cursor *string) ([]*UserRow, string, int64, error) {
	args := []any{}
	idx := 1
	add := func(v any) int { args = append(args, v); i := idx; idx++; return i }

	cond := "u.deleted_at IS NULL"
	if search != "" {
		cond += " AND (u.first_name ILIKE $" + itoa(add("%"+search+"%")) +
			" OR u.last_name ILIKE $" + itoa(idx-1) +
			" OR u.email ILIKE $" + itoa(idx-1) +
			" OR u.phone ILIKE $" + itoa(idx-1) + ")"
	}
	if role != "" {
		cond += " AND u.role=$" + itoa(add(role))
	}
	if status != "" {
		cond += " AND u.status=$" + itoa(add(status))
	}
	if cursor != nil {
		cond += " AND u.id>$" + itoa(add(*cursor))
	}
	limArg := add(limit + 1)

	rows, err := r.db.Query(ctx,
		"SELECT u.id,u.email,u.phone,u.first_name,u.last_name,u.role,u.status,u.created_at "+
			"FROM users u WHERE "+cond+" ORDER BY u.created_at DESC LIMIT $"+itoa(limArg), args...)
	if err != nil {
		return nil, "", 0, err
	}
	defer rows.Close()

	var users []*UserRow
	for rows.Next() {
		u := &UserRow{}
		if err := rows.Scan(&u.ID, &u.Email, &u.Phone, &u.FirstName, &u.LastName, &u.Role, &u.Status, &u.CreatedAt); err == nil {
			users = append(users, u)
		}
	}
	var cur string
	if len(users) > limit {
		users = users[:limit]
		cur = users[len(users)-1].ID
	}
	var total int64
	_ = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM users u WHERE "+cond, args[:len(args)-1]...).Scan(&total)
	return users, cur, total, nil
}

func (r *Repository) SuspendUser(ctx context.Context, userID, reason string) error {
	res, err := r.db.Exec(ctx,
		`UPDATE users SET status='suspended',updated_at=NOW() WHERE id=$1 AND status!='suspended'`, userID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return errors.New("user not found or already suspended")
	}
	return nil
}

func (r *Repository) ApproveVendor(ctx context.Context, vendorID string, commissionRate *float64) error {
	if commissionRate != nil {
		_, err := r.db.Exec(ctx,
			`UPDATE vendors SET status='active',commission_rate=$2,approved_at=NOW(),updated_at=NOW() WHERE id=$1`,
			vendorID, *commissionRate)
		return err
	}
	_, err := r.db.Exec(ctx,
		`UPDATE vendors SET status='active',approved_at=NOW(),updated_at=NOW() WHERE id=$1`, vendorID)
	return err
}

type Dashboard struct {
	GMVToday       float64
	GMVThisMonth   float64
	OrdersToday    int
	ActiveUsersNow int
	TopProducts    []map[string]any
	PaymentBreakdown map[string]float64
}

func (r *Repository) GetDashboard(ctx context.Context) (*Dashboard, error) {
	d := &Dashboard{PaymentBreakdown: map[string]float64{}}

	// GMV today
	_ = r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(total_amount),0) FROM orders
		WHERE status IN ('PAID','PROCESSING','SHIPPED','DELIVERED')
		AND placed_at >= NOW()::date`).Scan(&d.GMVToday)

	// GMV this month
	_ = r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(total_amount),0) FROM orders
		WHERE status IN ('PAID','PROCESSING','SHIPPED','DELIVERED')
		AND placed_at >= date_trunc('month',NOW())`).Scan(&d.GMVThisMonth)

	// Orders today
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE placed_at >= NOW()::date`).Scan(&d.OrdersToday)

	// Active users (logged in last 24h)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE last_login_at >= NOW() - INTERVAL '24 hours'`).Scan(&d.ActiveUsersNow)

	// Top products by sales
	rows, err := r.db.Query(ctx, `
		SELECT p.id,p.name,p.base_price,COALESCE(SUM(oi.quantity),0) AS sold
		FROM products p
		LEFT JOIN order_items oi ON oi.product_id=p.id
		GROUP BY p.id ORDER BY sold DESC LIMIT 5`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, name string
			var price float64
			var sold int
			if err := rows.Scan(&id, &name, &price, &sold); err == nil {
				d.TopProducts = append(d.TopProducts, map[string]any{
					"id": id, "name": name, "base_price": price, "total_sold": sold,
				})
			}
		}
	}

	// Payment breakdown
	pbRows, err := r.db.Query(ctx, `
		SELECT provider::text,COALESCE(SUM(amount),0) FROM payments
		WHERE status='COMPLETED' AND created_at >= date_trunc('month',NOW())
		GROUP BY provider`)
	if err == nil {
		defer pbRows.Close()
		for pbRows.Next() {
			var provider string
			var total float64
			if err := pbRows.Scan(&provider, &total); err == nil {
				d.PaymentBreakdown[provider] = total
			}
		}
	}

	return d, nil
}

// ── handler ───────────────────────────────────────────────────────────────────

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo} }

// AdminListUsers godoc
// @Summary      List all users (admin)
// @Tags         admin
// @Security     BearerAuth
// @Produce      json
// @Param        q       query  string  false  "Search by name/email/phone"
// @Param        role    query  string  false  "Filter by role"
// @Param        status  query  string  false  "Filter by status"
// @Success      200  {object}  map[string]any
// @Router       /admin/users [get]
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	q := r.URL.Query()
	users, cur, total, err := h.repo.ListUsers(r.Context(),
		q.Get("q"), q.Get("role"), q.Get("status"), 20, nil)
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		out = append(out, map[string]any{
			"id": u.ID, "email": u.Email, "phone": u.Phone,
			"first_name": u.FirstName, "last_name": u.LastName,
			"role": u.Role, "status": u.Status, "created_at": u.CreatedAt,
		})
	}
	var cursor *string
	if cur != "" {
		cursor = &cur
	}
	response.Paginated(w, out, &response.Meta{Cursor: cursor, HasMore: cur != "", Total: total})
}

// AdminSuspendUser godoc
// @Summary      Suspend user account (admin)
// @Tags         admin
// @Security     BearerAuth
// @Param        userId  path  string  true  "User UUID"
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /admin/users/{userId}/suspend [post]
func (h *Handler) SuspendUser(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req struct {
		Reason string `json:"reason" validate:"required"`
	}
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	if err := h.repo.SuspendUser(r.Context(), chi.URLParam(r, "userId"), req.Reason); err != nil {
		response.NotFound(w, "User", rid)
		return
	}
	response.Ok(w, map[string]string{"message": "User suspended"})
}

// AdminApproveVendor godoc
// @Summary      Approve vendor application (admin)
// @Tags         admin
// @Security     BearerAuth
// @Param        vendorId  path  string  true  "Vendor UUID"
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /admin/vendors/{vendorId}/approve [post]
func (h *Handler) ApproveVendor(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req struct {
		CommissionRate *float64 `json:"commission_rate"`
	}
	_ = validator.Decode(r, &req)
	if err := h.repo.ApproveVendor(r.Context(), chi.URLParam(r, "vendorId"), req.CommissionRate); err != nil {
		response.NotFound(w, "Vendor", rid)
		return
	}
	response.Ok(w, map[string]string{"message": "Vendor approved"})
}

// AdminDashboard godoc
// @Summary      Get admin dashboard metrics
// @Tags         admin
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /admin/analytics/dashboard [get]
func (h *Handler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	d, err := h.repo.GetDashboard(r.Context())
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, map[string]any{
		"gmv_today":          map[string]any{"amount": d.GMVToday, "currency": "XAF"},
		"gmv_this_month":     map[string]any{"amount": d.GMVThisMonth, "currency": "XAF"},
		"orders_today":       d.OrdersToday,
		"active_users_now":   d.ActiveUsersNow,
		"top_products":       d.TopProducts,
		"payment_breakdown":  d.PaymentBreakdown,
	})
}

// ── helpers ───────────────────────────────────────────────────────────────────

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

var _ = pgx.ErrNoRows
var _ = context.Background
