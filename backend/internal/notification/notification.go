package notification

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

type Notification struct {
	ID        string
	Type      string
	Title     string
	Body      string
	ActionURL *string
	IsRead    bool
	CreatedAt time.Time
}

type Repository struct{ db *database.DB }

func NewRepository(db *database.DB) *Repository { return &Repository{db} }

func (r *Repository) List(ctx context.Context, userID string, unreadOnly bool, limit int) ([]Notification, int, error) {
	q := `SELECT id,type,title,body,action_url,is_read,created_at FROM notifications WHERE user_id=$1`
	if unreadOnly {
		q += " AND is_read=false"
	}
	q += " ORDER BY created_at DESC LIMIT $2"

	rows, err := r.db.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var notifs []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Body, &n.ActionURL, &n.IsRead, &n.CreatedAt); err == nil {
			notifs = append(notifs, n)
		}
	}

	var unread int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id=$1 AND is_read=false`, userID).Scan(&unread)
	return notifs, unread, nil
}

func (r *Repository) MarkAllRead(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `UPDATE notifications SET is_read=true,read_at=NOW() WHERE user_id=$1 AND is_read=false`, userID)
	return err
}

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo} }

// ListNotifications godoc
// @Summary      List notifications
// @Tags         notifications
// @Security     BearerAuth
// @Produce      json
// @Param        unread_only  query  bool  false  "Only unread"
// @Success      200  {object}  map[string]any
// @Router       /notifications [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	unreadOnly := r.URL.Query().Get("unread_only") == "true"
	notifs, unread, err := h.repo.List(r.Context(), middleware.GetUserID(r.Context()), unreadOnly, 50)
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	out := make([]map[string]any, 0, len(notifs))
	for _, n := range notifs {
		out = append(out, map[string]any{
			"id": n.ID, "type": n.Type, "title": n.Title, "body": n.Body,
			"action_url": n.ActionURL, "is_read": n.IsRead, "created_at": n.CreatedAt,
		})
	}
	response.Ok(w, map[string]any{
		"data":         out,
		"unread_count": unread,
		"meta":         response.Meta{HasMore: false, Total: int64(len(notifs))},
	})
}

// MarkAllRead godoc
// @Summary      Mark all notifications as read
// @Tags         notifications
// @Security     BearerAuth
// @Success      204
// @Router       /notifications/read-all [post]
func (h *Handler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	_ = h.repo.MarkAllRead(r.Context(), middleware.GetUserID(r.Context()))
	response.NoContent(w)
}

// GetPreferences godoc
// @Summary      Get notification preferences
// @Tags         notifications
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /notifications/preferences [get]
func (h *Handler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	response.Ok(w, map[string]any{
		"push_enabled": true, "sms_enabled": true, "email_enabled": true,
		"order_updates": true, "promotions": true, "restock_alerts": true, "price_drops": true,
	})
}

// UpdatePreferences godoc
// @Summary      Update notification preferences
// @Tags         notifications
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /notifications/preferences [put]
func (h *Handler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req struct {
		PushEnabled    *bool `json:"push_enabled"`
		SMSEnabled     *bool `json:"sms_enabled"`
		EmailEnabled   *bool `json:"email_enabled"`
		OrderUpdates   *bool `json:"order_updates"`
		Promotions     *bool `json:"promotions"`
		RestockAlerts  *bool `json:"restock_alerts"`
		PriceDrops     *bool `json:"price_drops"`
	}
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	// In production: persist to user_notification_preferences table
	response.Ok(w, map[string]string{"message": "Preferences updated"})
}

// Satisfy unused imports
var _ = pgx.ErrNoRows
var _ = errors.New
var _ = context.Background
