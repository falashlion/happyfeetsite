package user

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

var ErrNotFound = errors.New("user: not found")

// ── models ────────────────────────────────────────────────────────────────────

type Profile struct {
	ID                string
	Email             *string
	Phone             *string
	FirstName         string
	LastName          string
	Role              string
	Status            string
	AvatarURL         *string
	EmailVerifiedAt   *time.Time
	PhoneVerifiedAt   *time.Time
	PreferredCurrency string
	PreferredLanguage string
	CreatedAt         time.Time
}

type Address struct {
	ID             string
	Label          string
	RecipientName  string
	RecipientPhone *string
	Street         string
	City           string
	State          *string
	PostalCode     *string
	Country        string
	Latitude       *float64
	Longitude      *float64
	IsDefault      bool
	CreatedAt      time.Time
}

// ── repository ────────────────────────────────────────────────────────────────

type Repository struct{ db *database.DB }

func NewRepository(db *database.DB) *Repository { return &Repository{db} }

func (r *Repository) GetByID(ctx context.Context, id string) (*Profile, error) {
	p := &Profile{}
	err := r.db.QueryRow(ctx, `
		SELECT id,email,phone,first_name,last_name,role,status,avatar_url,
		       email_verified_at,phone_verified_at,preferred_currency,preferred_language,created_at
		FROM users WHERE id=$1 AND deleted_at IS NULL`, id).Scan(
		&p.ID, &p.Email, &p.Phone, &p.FirstName, &p.LastName,
		&p.Role, &p.Status, &p.AvatarURL, &p.EmailVerifiedAt, &p.PhoneVerifiedAt,
		&p.PreferredCurrency, &p.PreferredLanguage, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (r *Repository) Update(ctx context.Context, id, first, last string) (*Profile, error) {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET first_name=$2,last_name=$3,updated_at=NOW() WHERE id=$1`, id, first, last)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) ListAddresses(ctx context.Context, userID string) ([]*Address, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id,label,recipient_name,recipient_phone,street,city,state,
		       postal_code,country,latitude,longitude,is_default,created_at
		FROM user_addresses WHERE user_id=$1 ORDER BY is_default DESC,created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var addrs []*Address
	for rows.Next() {
		a := &Address{}
		if err := rows.Scan(&a.ID, &a.Label, &a.RecipientName, &a.RecipientPhone,
			&a.Street, &a.City, &a.State, &a.PostalCode, &a.Country,
			&a.Latitude, &a.Longitude, &a.IsDefault, &a.CreatedAt); err != nil {
			continue
		}
		addrs = append(addrs, a)
	}
	return addrs, nil
}

type AddrInput struct {
	Label          string   `json:"label"            validate:"required,max=50"`
	RecipientName  string   `json:"recipient_name"   validate:"required"`
	RecipientPhone *string  `json:"recipient_phone"  validate:"omitempty,e164"`
	Street         string   `json:"street"           validate:"required"`
	City           string   `json:"city"             validate:"required"`
	State          *string  `json:"state"`
	PostalCode     *string  `json:"postal_code"`
	Country        string   `json:"country"          validate:"required,len=2"`
	Latitude       *float64 `json:"latitude"         validate:"omitempty,gte=-90,lte=90"`
	Longitude      *float64 `json:"longitude"        validate:"omitempty,gte=-180,lte=180"`
	IsDefault      bool     `json:"is_default"`
}

func (r *Repository) CreateAddress(ctx context.Context, userID string, in AddrInput) (*Address, error) {
	if in.IsDefault {
		_, _ = r.db.Exec(ctx, `UPDATE user_addresses SET is_default=false WHERE user_id=$1`, userID)
	}
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO user_addresses(user_id,label,recipient_name,recipient_phone,
		    street,city,state,postal_code,country,latitude,longitude,is_default)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`,
		userID, in.Label, in.RecipientName, in.RecipientPhone,
		in.Street, in.City, in.State, in.PostalCode, in.Country,
		in.Latitude, in.Longitude, in.IsDefault).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.getAddr(ctx, id, userID)
}

func (r *Repository) UpdateAddress(ctx context.Context, id, userID string, in AddrInput) (*Address, error) {
	if in.IsDefault {
		_, _ = r.db.Exec(ctx, `UPDATE user_addresses SET is_default=false WHERE user_id=$1 AND id!=$2`, userID, id)
	}
	_, err := r.db.Exec(ctx, `
		UPDATE user_addresses
		SET label=$3,recipient_name=$4,recipient_phone=$5,street=$6,city=$7,
		    state=$8,postal_code=$9,country=$10,latitude=$11,longitude=$12,
		    is_default=$13,updated_at=NOW()
		WHERE id=$1 AND user_id=$2`,
		id, userID, in.Label, in.RecipientName, in.RecipientPhone,
		in.Street, in.City, in.State, in.PostalCode, in.Country,
		in.Latitude, in.Longitude, in.IsDefault)
	if err != nil {
		return nil, err
	}
	return r.getAddr(ctx, id, userID)
}

func (r *Repository) DeleteAddress(ctx context.Context, id, userID string) error {
	res, err := r.db.Exec(ctx, `DELETE FROM user_addresses WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) getAddr(ctx context.Context, id, userID string) (*Address, error) {
	a := &Address{}
	err := r.db.QueryRow(ctx, `
		SELECT id,label,recipient_name,recipient_phone,street,city,state,
		       postal_code,country,latitude,longitude,is_default,created_at
		FROM user_addresses WHERE id=$1 AND user_id=$2`, id, userID).Scan(
		&a.ID, &a.Label, &a.RecipientName, &a.RecipientPhone,
		&a.Street, &a.City, &a.State, &a.PostalCode, &a.Country,
		&a.Latitude, &a.Longitude, &a.IsDefault, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

// ── handler ───────────────────────────────────────────────────────────────────

type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo} }

// GetMe godoc
// @Summary      Get current user profile
// @Tags         users
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /users/me [get]
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	p, err := h.repo.GetByID(r.Context(), middleware.GetUserID(r.Context()))
	if errors.Is(err, ErrNotFound) {
		response.NotFound(w, "User", rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, fmtProfile(p))
}

// UpdateMe godoc
// @Summary      Update user profile
// @Tags         users
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /users/me [patch]
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	p, err := h.repo.Update(r.Context(), middleware.GetUserID(r.Context()), req.FirstName, req.LastName)
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, fmtProfile(p))
}

// ListAddresses godoc
// @Summary      List addresses
// @Tags         users
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /users/me/addresses [get]
func (h *Handler) ListAddresses(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	addrs, err := h.repo.ListAddresses(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, map[string]any{"data": fmtAddresses(addrs)})
}

// CreateAddress godoc
// @Summary      Add delivery address
// @Tags         users
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Success      201  {object}  map[string]any
// @Router       /users/me/addresses [post]
func (h *Handler) CreateAddress(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req AddrInput
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	a, err := h.repo.CreateAddress(r.Context(), middleware.GetUserID(r.Context()), req)
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Created(w, fmtAddress(a))
}

// UpdateAddress godoc
// @Summary      Update address
// @Tags         users
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        addressId  path  string  true  "Address UUID"
// @Success      200  {object}  map[string]any
// @Router       /users/me/addresses/{addressId} [put]
func (h *Handler) UpdateAddress(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req AddrInput
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	a, err := h.repo.UpdateAddress(r.Context(), chi.URLParam(r, "addressId"), middleware.GetUserID(r.Context()), req)
	if errors.Is(err, ErrNotFound) {
		response.NotFound(w, "Address", rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, fmtAddress(a))
}

// DeleteAddress godoc
// @Summary      Delete address
// @Tags         users
// @Security     BearerAuth
// @Param        addressId  path  string  true  "Address UUID"
// @Success      204
// @Router       /users/me/addresses/{addressId} [delete]
func (h *Handler) DeleteAddress(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	err := h.repo.DeleteAddress(r.Context(), chi.URLParam(r, "addressId"), middleware.GetUserID(r.Context()))
	if errors.Is(err, ErrNotFound) {
		response.NotFound(w, "Address", rid)
		return
	}
	response.NoContent(w)
}

// ── formatters ────────────────────────────────────────────────────────────────

func fmtProfile(p *Profile) map[string]any {
	return map[string]any{
		"id": p.ID, "email": p.Email, "phone": p.Phone,
		"first_name": p.FirstName, "last_name": p.LastName,
		"role": p.Role, "status": p.Status, "avatar_url": p.AvatarURL,
		"email_verified_at": p.EmailVerifiedAt, "phone_verified_at": p.PhoneVerifiedAt,
		"preferred_currency": p.PreferredCurrency, "preferred_language": p.PreferredLanguage,
		"created_at": p.CreatedAt,
	}
}

func fmtAddresses(addrs []*Address) []map[string]any {
	out := make([]map[string]any, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, fmtAddress(a))
	}
	return out
}

func fmtAddress(a *Address) map[string]any {
	return map[string]any{
		"id": a.ID, "label": a.Label, "recipient_name": a.RecipientName,
		"recipient_phone": a.RecipientPhone, "street": a.Street, "city": a.City,
		"state": a.State, "postal_code": a.PostalCode, "country": a.Country,
		"latitude": a.Latitude, "longitude": a.Longitude,
		"is_default": a.IsDefault, "created_at": a.CreatedAt,
	}
}
