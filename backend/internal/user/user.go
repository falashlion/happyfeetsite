package user

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/happyfeet/api/pkg/database"
	"github.com/happyfeet/api/pkg/middleware"
	"github.com/happyfeet/api/pkg/response"
	"github.com/happyfeet/api/pkg/validator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"
)

var (
	ErrNotFound   = errors.New("user: not found")
	ErrPhoneTaken = errors.New("user: phone already registered")
)

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

// ProfilePatch carries only the fields a request actually sent.
//
// Pointers, not plain strings: a PATCH has to leave an omitted field alone, and
// a *string is the only way to tell "absent" from "sent as empty". The previous
// shape took plain strings and wrote them unconditionally, so a request that
// set one field silently blanked every other one.
type ProfilePatch struct {
	FirstName *string `json:"first_name" validate:"omitempty,max=100"`
	LastName  *string `json:"last_name"  validate:"omitempty,max=100"`
	// Phone is a login identifier, so it can be changed but not cleared here —
	// see Normalise.
	Phone *string `json:"phone" validate:"omitempty,e164,max=25"`
}

// Normalise trims the patch and rejects the values the database would accept
// but a person would not. It runs before any SQL so the caller gets a 400
// naming the field rather than a surprise further down.
func (p *ProfilePatch) Normalise() error {
	for field, v := range map[string]*string{"first_name": p.FirstName, "last_name": p.LastName} {
		if v == nil {
			continue
		}
		if *v = strings.TrimSpace(*v); *v == "" {
			return fmt.Errorf("%s cannot be empty", field)
		}
	}
	if p.Phone != nil {
		// Dropping the only way into an account is not a profile edit. Over
		// HTTP the e164 rule rejects "" first; this keeps Normalise correct for
		// any caller that does not run the validator.
		if *p.Phone = strings.TrimSpace(*p.Phone); *p.Phone == "" {
			return errors.New("phone cannot be removed here — contact support to change how you sign in")
		}
	}
	return nil
}

// Empty reports whether the patch asks for nothing.
func (p ProfilePatch) Empty() bool {
	return p.FirstName == nil && p.LastName == nil && p.Phone == nil
}

// buildUpdate renders the patch as a single UPDATE touching only the columns
// that were sent. Split out from Update so the "leave the rest alone" property
// can be asserted without a database.
func buildUpdate(id string, p ProfilePatch) (string, []any) {
	args := []any{id}
	sets := []string{"updated_at=NOW()"}

	add := func(col string, val any) int {
		args = append(args, val)
		sets = append(sets, fmt.Sprintf("%s=$%d", col, len(args)))
		return len(args)
	}

	if p.FirstName != nil {
		add("first_name", *p.FirstName)
	}
	if p.LastName != nil {
		add("last_name", *p.LastName)
	}
	if p.Phone != nil {
		args = append(args, *p.Phone)
		n := len(args)
		// Both ::text casts are load-bearing. The placeholder appears as an
		// assignment to a varchar column and inside a comparison, and without
		// matching casts Postgres deduces two types for it and rejects the
		// whole statement (SQLSTATE 42P08).
		sets = append(sets, fmt.Sprintf("phone=$%d::text", n))
		// A new number has not been proved, and phone_verified_at is what the
		// OTP sign-in trusts. On the right-hand side of an UPDATE `phone` still
		// holds the old value, so re-sending the same number keeps its
		// verification instead of silently revoking it.
		sets = append(sets, fmt.Sprintf(
			"phone_verified_at = CASE WHEN phone IS DISTINCT FROM $%d::text THEN NULL ELSE phone_verified_at END", n))
	}

	return fmt.Sprintf(
		"UPDATE users SET %s WHERE id=$1 AND deleted_at IS NULL",
		strings.Join(sets, ",")), args
}

// Update applies a partial profile change and returns the stored profile.
func (r *Repository) Update(ctx context.Context, id string, p ProfilePatch) (*Profile, error) {
	if p.Empty() {
		return r.GetByID(ctx, id)
	}

	sql, args := buildUpdate(id, p)
	tag, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrPhoneTaken
		}
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return r.GetByID(ctx, id)
}

// isUniqueViolation reads the SQLSTATE rather than matching on the driver's
// message text, so a reworded error cannot turn a 409 into a 500.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
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
// @Summary      Update user profile (partial — omitted fields are left alone)
// @Tags         users
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  ProfilePatch  true  "Fields to change"
// @Success      200  {object}  map[string]any
// @Failure      409  {object}  map[string]any  "Phone already registered"
// @Router       /users/me [patch]
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req ProfilePatch
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	if err := req.Normalise(); err != nil {
		response.BadRequest(w, err.Error(), rid)
		return
	}

	p, err := h.repo.Update(r.Context(), middleware.GetUserID(r.Context()), req)
	switch {
	case errors.Is(err, ErrNotFound):
		response.NotFound(w, "User", rid)
	case errors.Is(err, ErrPhoneTaken):
		// Deliberately does not say whose account holds it.
		response.Conflict(w, "That phone number is already registered", rid)
	case err != nil:
		// Without this the cause never leaves the process and a 500 here is
		// indistinguishable from any other.
		log.Error().Err(err).Str("rid", rid).Msg("update profile")
		response.InternalError(w, rid)
	default:
		response.Ok(w, fmtProfile(p))
	}
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
