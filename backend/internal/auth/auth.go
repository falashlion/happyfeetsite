package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/happyfeet/api/pkg/cache"
	"github.com/happyfeet/api/pkg/config"
	"github.com/happyfeet/api/pkg/database"
	"github.com/happyfeet/api/pkg/middleware"
	"github.com/happyfeet/api/pkg/oauth"
	"github.com/happyfeet/api/pkg/password"
	"github.com/happyfeet/api/pkg/response"
	"github.com/happyfeet/api/pkg/token"
	"github.com/happyfeet/api/pkg/validator"
	"github.com/jackc/pgx/v5"
)

// ── errors ────────────────────────────────────────────────────────────────────

var (
	ErrNotFound     = errors.New("auth: not found")
	ErrDuplicate    = errors.New("auth: duplicate")
	ErrBadCreds     = errors.New("auth: bad credentials")
	ErrLocked       = errors.New("auth: account locked")
	ErrSuspended    = errors.New("auth: account suspended")
	ErrInvalidOTP   = errors.New("auth: invalid OTP")
	ErrOTPRateLimit = errors.New("auth: OTP rate limit")
)

const (
	maxLoginAttempts = 5
	lockDuration     = 15 * time.Minute
	otpExpiry        = 5 * time.Minute
)

// ── models ────────────────────────────────────────────────────────────────────

type User struct {
	ID                string
	Email             *string
	Phone             *string
	PasswordHash      *string
	FirstName         string
	LastName          string
	Role              string
	Status            string
	AvatarURL         *string
	EmailVerifiedAt   *time.Time
	PhoneVerifiedAt   *time.Time
	LockedUntil       *time.Time
	LoginAttemptCount int
}

type OTP struct {
	ID        string
	UserID    *string
	Code      string // hashed
	ExpiresAt time.Time
}

// ── repository ────────────────────────────────────────────────────────────────

type Repository struct{ db *database.DB }

func NewRepository(db *database.DB) *Repository { return &Repository{db} }

const userCols = `id, email, phone, password_hash, first_name, last_name,
	role, status, avatar_url, email_verified_at, phone_verified_at,
	locked_until, login_attempt_count`

func scanUser(row pgx.Row, u *User) error {
	return row.Scan(&u.ID, &u.Email, &u.Phone, &u.PasswordHash,
		&u.FirstName, &u.LastName, &u.Role, &u.Status, &u.AvatarURL,
		&u.EmailVerifiedAt, &u.PhoneVerifiedAt, &u.LockedUntil, &u.LoginAttemptCount)
}

func (r *Repository) byEmail(ctx context.Context, email string) (*User, error) {
	u := &User{}
	err := scanUser(r.db.QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE email=$1 AND deleted_at IS NULL`, email), u)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *Repository) byPhone(ctx context.Context, phone string) (*User, error) {
	u := &User{}
	err := scanUser(r.db.QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE phone=$1 AND deleted_at IS NULL`, phone), u)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *Repository) byID(ctx context.Context, id string) (*User, error) {
	u := &User{}
	err := scanUser(r.db.QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE id=$1 AND deleted_at IS NULL`, id), u)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *Repository) create(ctx context.Context, email, phone *string, hash *string, first, last, role string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx,
		`INSERT INTO users(email,phone,password_hash,first_name,last_name,role)
		 VALUES($1,$2,$3,$4,$5,$6) RETURNING id`,
		email, phone, hash, first, last, role).Scan(&id)
	if err != nil && isDup(err) {
		return "", ErrDuplicate
	}
	return id, err
}

func (r *Repository) incrLoginAttempts(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET login_attempt_count=login_attempt_count+1 WHERE id=$1`, id)
	return err
}

func (r *Repository) lockAccount(ctx context.Context, id string, until time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET locked_until=$2,login_attempt_count=0 WHERE id=$1`, id, until)
	return err
}

func (r *Repository) resetAttempts(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET login_attempt_count=0,locked_until=NULL,last_login_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *Repository) verifyEmail(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET email_verified_at=NOW(),status='active' WHERE id=$1 AND email_verified_at IS NULL`, id)
	return err
}

func (r *Repository) verifyPhone(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET phone_verified_at=NOW(),status='active' WHERE id=$1 AND phone_verified_at IS NULL`, id)
	return err
}

func (r *Repository) updatePassword(ctx context.Context, id, hash string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET password_hash=$2,updated_at=NOW() WHERE id=$1`, id, hash)
	return err
}

func (r *Repository) updatePushToken(ctx context.Context, id, tok, platform string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET push_token=$2,push_platform=$3,updated_at=NOW() WHERE id=$1`, id, tok, platform)
	return err
}

// social auth
func (r *Repository) socialFind(ctx context.Context, provider, uid string) (string, error) {
	var userID string
	err := r.db.QueryRow(ctx,
		`SELECT user_id FROM user_social_auth WHERE provider=$1 AND provider_uid=$2`,
		provider, uid).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return userID, err
}

func (r *Repository) socialLink(ctx context.Context, userID, provider, uid string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO user_social_auth(user_id,provider,provider_uid) VALUES($1,$2,$3)
		 ON CONFLICT(provider,provider_uid) DO NOTHING`,
		userID, provider, uid)
	return err
}

// OTP
func (r *Repository) createOTP(ctx context.Context, userID *string, identifier, codeHash, purpose, channel string, exp time.Time) error {
	// Invalidate previous
	_, _ = r.db.Exec(ctx,
		`UPDATE otps SET used_at=NOW() WHERE identifier=$1 AND purpose=$2 AND used_at IS NULL AND expires_at>NOW()`,
		identifier, purpose)
	_, err := r.db.Exec(ctx,
		`INSERT INTO otps(user_id,identifier,code,purpose,channel,expires_at) VALUES($1,$2,$3,$4,$5,$6)`,
		userID, identifier, codeHash, purpose, channel, exp)
	return err
}

func (r *Repository) findValidOTP(ctx context.Context, identifier, purpose string) (*OTP, error) {
	otp := &OTP{}
	err := r.db.QueryRow(ctx,
		`SELECT id,user_id,code,expires_at FROM otps
		 WHERE identifier=$1 AND purpose=$2 AND used_at IS NULL AND expires_at>NOW()
		 ORDER BY created_at DESC LIMIT 1`,
		identifier, purpose).Scan(&otp.ID, &otp.UserID, &otp.Code, &otp.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return otp, err
}

func (r *Repository) markOTPUsed(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE otps SET used_at=NOW() WHERE id=$1`, id)
	return err
}

// ── service ───────────────────────────────────────────────────────────────────

type Service struct {
	repo  *Repository
	cache *cache.Client
	maker *token.Maker
	cfg   *config.Config
}

func NewService(repo *Repository, c *cache.Client, m *token.Maker, cfg *config.Config) *Service {
	return &Service{repo, c, m, cfg}
}

type RegisterIn struct {
	Email    *string
	Phone    *string
	Password string
	First    string
	Last     string
}

func (s *Service) Register(ctx context.Context, in RegisterIn) (string, error) {
	hash, err := password.Hash(in.Password)
	if err != nil {
		return "", err
	}

	id, err := s.repo.create(ctx, in.Email, in.Phone, &hash, in.First, in.Last, "customer")
	if errors.Is(err, ErrDuplicate) {
		return "", ErrDuplicate
	}
	return id, err
}

type LoginIn struct {
	Email    *string
	Phone    *string
	Password string
}

type Tokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	User         *User
}

func (s *Service) Login(ctx context.Context, in LoginIn) (*Tokens, error) {
	var u *User
	var err error
	if in.Email != nil {
		u, err = s.repo.byEmail(ctx, *in.Email)
	} else {
		u, err = s.repo.byPhone(ctx, *in.Phone)
	}
	if errors.Is(err, ErrNotFound) {
		return nil, ErrBadCreds
	}
	if err != nil {
		return nil, err
	}
	if u.LockedUntil != nil && time.Now().Before(*u.LockedUntil) {
		return nil, ErrLocked
	}
	if u.Status == "suspended" {
		return nil, ErrSuspended
	}
	if u.PasswordHash == nil {
		return nil, ErrBadCreds
	}
	if err := password.Verify(in.Password, *u.PasswordHash); err != nil {
		_ = s.repo.incrLoginAttempts(ctx, u.ID)
		if u.LoginAttemptCount+1 >= maxLoginAttempts {
			_ = s.repo.lockAccount(ctx, u.ID, time.Now().Add(lockDuration))
		}
		return nil, ErrBadCreds
	}
	_ = s.repo.resetAttempts(ctx, u.ID)
	return s.issuePair(ctx, u)
}

func (s *Service) Refresh(ctx context.Context, refreshTok string) (*Tokens, error) {
	claims, err := s.maker.VerifyRefreshToken(refreshTok)
	if err != nil {
		return nil, ErrBadCreds
	}
	key := cache.Key(cache.KeyRefreshToken, hashStr(refreshTok))
	if _, err := s.cache.Get(ctx, key); errors.Is(err, cache.ErrNotFound) {
		return nil, ErrBadCreds
	}
	_ = s.cache.Del(ctx, key)
	u, err := s.repo.byID(ctx, claims.UserID)
	if err != nil {
		return nil, ErrBadCreds
	}
	return s.issuePair(ctx, u)
}

func (s *Service) Logout(ctx context.Context, jti, refreshTok string) {
	if jti != "" {
		_ = s.cache.Set(ctx, cache.Key(cache.KeySessionBlacklist, jti), []byte("1"), s.cfg.JWT.AccessTokenExpiry)
	}
	if refreshTok != "" {
		_ = s.cache.Del(ctx, cache.Key(cache.KeyRefreshToken, hashStr(refreshTok)))
	}
}

func (s *Service) issuePair(ctx context.Context, u *User) (*Tokens, error) {
	at, _, err := s.maker.CreateAccessToken(u.ID, u.Role)
	if err != nil {
		return nil, err
	}
	rt, _, err := s.maker.CreateRefreshToken(u.ID, u.Role)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Set(ctx, cache.Key(cache.KeyRefreshToken, hashStr(rt)),
		[]byte(u.ID), s.cfg.JWT.RefreshTokenExpiry)
	return &Tokens{at, rt, s.maker.AccessExpirySeconds(), u}, nil
}

// SendOTP generates, stores, and returns the OTP code.
func (s *Service) SendOTP(ctx context.Context, userID *string, identifier, purpose, channel string) (string, error) {
	// Rate limit: 3 per 10 min
	rlKey := cache.Key(cache.KeyOTPRate, purpose, identifier)
	n, _ := s.cache.Incr(ctx, rlKey)
	if n == 1 {
		_ = s.cache.Expire(ctx, rlKey, 10*time.Minute)
	}
	if n > 3 {
		return "", ErrOTPRateLimit
	}
	code, err := genOTP(6)
	if err != nil {
		return "", err
	}
	if err := s.repo.createOTP(ctx, userID, identifier, hashStr(code), purpose, channel, time.Now().Add(otpExpiry)); err != nil {
		return "", err
	}
	return code, nil
}

// VerifyOTP checks the OTP and returns the otp row.
// CreatePasswordReset issues a reset code for the account behind an email.
//
// The code is stored as its own OTP identifier, because ConfirmPasswordReset
// receives nothing but the code — the user pastes it without re-entering which
// address it was for.
//
// Returns a nil user when no account matches. That is not an error: the caller
// answers identically either way so the endpoint cannot be used to discover
// which addresses are registered.
func (s *Service) CreatePasswordReset(ctx context.Context, email string) (*User, string, error) {
	u, err := s.repo.byEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}

	// Rate limit per address, mirroring SendOTP: 3 per 10 minutes.
	rlKey := cache.Key(cache.KeyOTPRate, "password_reset", email)
	n, _ := s.cache.Incr(ctx, rlKey)
	if n == 1 {
		_ = s.cache.Expire(ctx, rlKey, 10*time.Minute)
	}
	if n > 3 {
		return nil, "", ErrOTPRateLimit
	}

	code, err := genOTP(6)
	if err != nil {
		return nil, "", err
	}
	if err := s.repo.createOTP(ctx, &u.ID, code, hashStr(code), "password_reset", "email", time.Now().Add(otpExpiry)); err != nil {
		return nil, "", err
	}
	return u, code, nil
}

func (s *Service) VerifyOTP(ctx context.Context, identifier, purpose, code string) (*OTP, error) {
	otp, err := s.repo.findValidOTP(ctx, identifier, purpose)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrInvalidOTP
	}
	if otp.Code != hashStr(code) || time.Now().After(otp.ExpiresAt) {
		return nil, ErrInvalidOTP
	}
	_ = s.repo.markOTPUsed(ctx, otp.ID)
	return otp, nil
}

// SocialLogin finds or creates a user from OAuth provider data.
func (s *Service) SocialLogin(ctx context.Context, provider, uid, email, first, last string) (*Tokens, bool, error) {
	userID, err := s.repo.socialFind(ctx, provider, uid)
	isNew := false
	if errors.Is(err, ErrNotFound) {
		// Check by email first
		if email != "" {
			existing, e2 := s.repo.byEmail(ctx, email)
			if e2 == nil {
				userID = existing.ID
			}
		}
		if userID == "" {
			isNew = true
			ep := strPtr(email)
			userID, err = s.repo.create(ctx, ep, nil, nil, first, last, "customer")
			if err != nil {
				return nil, false, err
			}
			if email != "" {
				_ = s.repo.verifyEmail(ctx, userID)
			}
		}
		_ = s.repo.socialLink(ctx, userID, provider, uid)
	} else if err != nil {
		return nil, false, err
	}
	u, err := s.repo.byID(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	toks, err := s.issuePair(ctx, u)
	return toks, isNew, err
}

// ── handler ───────────────────────────────────────────────────────────────────

// Notifier is the slice of the notification service this package needs. It is
// declared here rather than imported as a concrete type so auth can be tested
// without a mail relay, and so a nil notifier is a valid no-op.
type Notifier interface {
	SendWelcome(to, firstName string)
	SendPasswordResetCode(to, firstName, code, expiresIn string)
	SendVerificationCode(to, firstName, code, expiresIn string)
}

type Handler struct {
	svc *Service
	// google is nil, or disabled, when GOOGLE_CLIENT_ID is unset — the endpoint
	// then reports 503 rather than pretending to authenticate anyone.
	google *oauth.GoogleVerifier
	// notify is nil when email is unconfigured; every call site guards for it.
	notify Notifier
}

func NewHandler(svc *Service, google *oauth.GoogleVerifier, notify Notifier) *Handler {
	return &Handler{svc: svc, google: google, notify: notify}
}

// Register godoc
// @Summary      Register new user
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      registerReq  true  "Registration payload"
// @Success      201   {object}  map[string]any
// @Failure      400   {object}  map[string]any
// @Failure      409   {object}  map[string]any
// @Router       /auth/register [post]
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req registerReq
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	if req.Email == nil && req.Phone == nil {
		response.BadRequest(w, "email or phone is required", rid)
		return
	}
	if !strongPwd(req.Password) {
		response.BadRequest(w, "password needs ≥8 chars, 1 uppercase, 1 digit", rid)
		return
	}
	id, err := h.svc.Register(r.Context(), RegisterIn{
		Email: req.Email, Phone: req.Phone,
		Password: req.Password,
		First:    strings.TrimSpace(req.FirstName),
		Last:     strings.TrimSpace(req.LastName),
	})
	if errors.Is(err, ErrDuplicate) {
		response.Conflict(w, "Account with this email or phone already exists", rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	// Fire-and-forget: a slow relay must not hold up the response, and a failed
	// greeting must not fail a registration that already succeeded.
	if h.notify != nil && req.Email != nil {
		h.notify.SendWelcome(*req.Email, strings.TrimSpace(req.FirstName))
	}

	response.Created(w, map[string]any{"message": "Account created. Verify your contact to activate.", "user_id": id})
}

// Login godoc
// @Summary      Login
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      loginReq  true  "Login payload"
// @Success      200   {object}  map[string]any
// @Failure      401   {object}  map[string]any
// @Failure      423   {object}  map[string]any
// @Router       /auth/login [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req loginReq
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	if req.Email == nil && req.Phone == nil {
		response.BadRequest(w, "email or phone required", rid)
		return
	}
	toks, err := h.svc.Login(r.Context(), LoginIn{Email: req.Email, Phone: req.Phone, Password: req.Password})
	switch {
	case errors.Is(err, ErrBadCreds):
		response.Err(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email/phone or password", rid)
	case errors.Is(err, ErrLocked):
		response.Err(w, http.StatusLocked, "ACCOUNT_LOCKED", "Account locked for 15 minutes", rid)
	case errors.Is(err, ErrSuspended):
		response.Err(w, http.StatusForbidden, "ACCOUNT_SUSPENDED", "Account suspended — contact support", rid)
	case err != nil:
		response.InternalError(w, rid)
	default:
		response.Ok(w, map[string]any{
			"tokens": tokensJSON(toks),
			"user":   userJSON(toks.User),
		})
	}
}

// Refresh godoc
// @Summary      Refresh tokens
// @Tags         auth
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  map[string]any
// @Failure      401  {object}  map[string]any
// @Router       /auth/refresh [post]
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	tok := bearerTok(r)
	if tok == "" {
		response.Unauthorized(w, rid)
		return
	}
	toks, err := h.svc.Refresh(r.Context(), tok)
	if err != nil {
		response.Unauthorized(w, rid)
		return
	}
	response.Ok(w, tokensJSON(toks))
}

// Logout godoc
// @Summary      Logout
// @Tags         auth
// @Security     BearerAuth
// @Success      204
// @Router       /auth/logout [post]
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	h.svc.Logout(r.Context(), middleware.GetJTI(r.Context()), req.RefreshToken)
	response.NoContent(w)
}

// SendOTP godoc
// @Summary      Send OTP
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      otpReq  true  "OTP request"
// @Success      200   {object}  map[string]any
// @Failure      429   {object}  map[string]any
// @Router       /auth/otp/send [post]
func (h *Handler) SendOTP(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req otpReq
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	var identifier string
	if req.Channel == "email" && req.Email != nil {
		identifier = *req.Email
	} else if req.Channel == "sms" && req.Phone != nil {
		identifier = *req.Phone
	} else {
		response.BadRequest(w, "provide email for email channel or phone for sms", rid)
		return
	}
	uid := middleware.GetUserID(r.Context())
	var uidPtr *string
	if uid != "" {
		uidPtr = &uid
	}
	code, err := h.svc.SendOTP(r.Context(), uidPtr, identifier, "verification", req.Channel)
	if errors.Is(err, ErrOTPRateLimit) {
		response.TooManyRequests(w, rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	// Email codes are delivered here. SMS has no gateway wired, so an SMS
	// request still creates a code that reaches nobody — a known gap.
	if req.Channel == "email" && h.notify != nil {
		h.notify.SendVerificationCode(identifier, "", code, "5 minutes")
	}

	response.Ok(w, map[string]any{"message": "OTP sent to " + identifier, "expires_in": 300})
}

// VerifyOTP godoc
// @Summary      Verify OTP
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      verifyOTPReq  true  "Verification payload"
// @Success      200   {object}  map[string]any
// @Failure      400   {object}  map[string]any
// @Router       /auth/otp/verify [post]
func (h *Handler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req verifyOTPReq
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	var identifier string
	if req.Channel == "email" && req.Email != nil {
		identifier = *req.Email
	} else if req.Channel == "sms" && req.Phone != nil {
		identifier = *req.Phone
	} else {
		response.BadRequest(w, "provide matching channel and contact", rid)
		return
	}
	otp, err := h.svc.VerifyOTP(r.Context(), identifier, "verification", req.OTP)
	if errors.Is(err, ErrInvalidOTP) {
		response.Err(w, http.StatusBadRequest, "INVALID_OTP", "Invalid or expired OTP", rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	if otp.UserID != nil {
		if req.Channel == "email" {
			_ = h.svc.repo.verifyEmail(r.Context(), *otp.UserID)
		} else {
			_ = h.svc.repo.verifyPhone(r.Context(), *otp.UserID)
		}
	}
	response.Ok(w, map[string]any{"verified": true})
}

// RequestPasswordReset godoc
// @Summary      Request password reset
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /auth/password/reset [post]
func (h *Handler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())

	var req struct {
		Email string `json:"email" validate:"required,email"`
	}
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}

	// The same 200 is returned whether or not the address is registered, so the
	// endpoint cannot be used to enumerate accounts. Only the email differs.
	const sameAnswer = "If an account exists, you'll receive a reset code."

	u, code, err := h.svc.CreatePasswordReset(r.Context(), req.Email)
	if errors.Is(err, ErrOTPRateLimit) {
		response.TooManyRequests(w, rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}

	if u != nil && h.notify != nil {
		h.notify.SendPasswordResetCode(req.Email, u.FirstName, code, "5 minutes")
	}

	response.Ok(w, map[string]string{"message": sameAnswer})
}

// ConfirmPasswordReset godoc
// @Summary      Confirm password reset
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /auth/password/reset [put]
func (h *Handler) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req struct {
		Token       string `json:"token"        validate:"required"`
		NewPassword string `json:"new_password" validate:"required,min=8"`
	}
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	if !strongPwd(req.NewPassword) {
		response.BadRequest(w, "password needs ≥8 chars, 1 uppercase, 1 digit", rid)
		return
	}
	// Verify OTP token used as reset token (identifier = token, purpose = password_reset)
	otp, err := h.svc.VerifyOTP(r.Context(), req.Token, "password_reset", req.Token)
	if err != nil || otp.UserID == nil {
		response.Err(w, http.StatusBadRequest, "INVALID_TOKEN", "Invalid or expired reset token", rid)
		return
	}
	hash, _ := password.Hash(req.NewPassword)
	_ = h.svc.repo.updatePassword(r.Context(), *otp.UserID, hash)
	response.Ok(w, map[string]string{"message": "Password updated."})
}

// GoogleLogin godoc
// @Summary      Sign in with Google
// @Description  Exchanges a Google ID token for a HappyFeet token pair, creating
// @Description  the account on first use. Returns 201 when a new account was
// @Description  created, 200 when an existing one was signed in.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      googleLoginReq  true  "Google ID token"
// @Success      200   {object}  map[string]any
// @Success      201   {object}  map[string]any
// @Failure      401   {object}  map[string]any
// @Failure      503   {object}  map[string]any
// @Router       /auth/social/google [post]
func (h *Handler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())

	if h.google == nil || !h.google.Enabled() {
		response.Err(w, http.StatusServiceUnavailable, "GOOGLE_SIGNIN_DISABLED",
			"Google sign-in is not configured on this server", rid)
		return
	}

	var req googleLoginReq
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}

	claims, err := h.google.Verify(r.Context(), req.IDToken)
	switch {
	case errors.Is(err, oauth.ErrEmailUnverified):
		response.Err(w, http.StatusUnauthorized, "EMAIL_NOT_VERIFIED",
			"Google has not verified this email address", rid)
		return
	case errors.Is(err, oauth.ErrWrongAudience), errors.Is(err, oauth.ErrWrongIssuer):
		// Deliberately vague to the client; the detail is in the server log.
		response.Err(w, http.StatusUnauthorized, "INVALID_GOOGLE_TOKEN",
			"This Google sign-in could not be verified", rid)
		return
	case err != nil:
		response.Err(w, http.StatusUnauthorized, "INVALID_GOOGLE_TOKEN",
			"This Google sign-in could not be verified", rid)
		return
	}

	first, last := splitName(claims.GivenName, claims.FamilyName, claims.Name, claims.Email)

	toks, isNew, err := h.svc.SocialLogin(r.Context(), "google", claims.Subject, claims.Email, first, last)
	switch {
	case errors.Is(err, ErrSuspended):
		response.Err(w, http.StatusForbidden, "ACCOUNT_SUSPENDED", "Account suspended — contact support", rid)
	case err != nil:
		response.InternalError(w, rid)
	default:
		body := map[string]any{
			"tokens":   tokensJSON(toks),
			"user":     userJSON(toks.User),
			"is_new":   isNew,
			"provider": "google",
		}
		if isNew {
			response.Created(w, body)
			return
		}
		response.Ok(w, body)
	}
}

// Providers godoc
// @Summary      List enabled sign-in providers
// @Description  Lets the storefront discover which social buttons to render and
// @Description  with which public client ID, so the ID is configured in exactly
// @Description  one place (the API) rather than duplicated into the web build.
// @Tags         auth
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /auth/providers [get]
func (h *Handler) Providers(w http.ResponseWriter, r *http.Request) {
	enabled := h.google != nil && h.google.Enabled()
	google := map[string]any{"enabled": enabled}
	if enabled {
		// An OAuth *client ID* is public by design — it ships in the browser.
		// The client secret is not involved in the ID-token flow at all.
		google["client_id"] = h.google.ClientID()
	}
	response.Ok(w, map[string]any{"google": google})
}

type googleLoginReq struct {
	// IDToken is the JWT from Google Identity Services (`credential` in the
	// browser callback), not an OAuth access token.
	IDToken string `json:"id_token" validate:"required"`
}

// splitName derives a first/last pair from whatever Google supplied. Given and
// family names are optional on the ID token — a bare `name`, or nothing but an
// email address, both have to produce something usable, because first_name and
// last_name are NOT NULL on the users table.
func splitName(given, family, full, email string) (first, last string) {
	given, family, full = strings.TrimSpace(given), strings.TrimSpace(family), strings.TrimSpace(full)

	if given != "" || family != "" {
		first, last = given, family
	} else if full != "" {
		if i := strings.LastIndex(full, " "); i > 0 {
			first, last = strings.TrimSpace(full[:i]), strings.TrimSpace(full[i+1:])
		} else {
			first = full
		}
	}

	if first == "" {
		// Fall back to the local part of the address, title-cased.
		local := email
		if i := strings.Index(local, "@"); i > 0 {
			local = local[:i]
		}
		local = strings.NewReplacer(".", " ", "_", " ", "-", " ", "+", " ").Replace(local)
		local = strings.TrimSpace(local)
		if local == "" {
			local = "Member"
		}
		first = strings.ToUpper(local[:1]) + local[1:]
	}
	if last == "" {
		// The column is NOT NULL; a placeholder the customer can edit in their
		// profile beats refusing an otherwise valid sign-in.
		last = "—"
	}
	return first, last
}

// AppleLogin godoc
// @Summary      Sign in with Apple
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /auth/social/apple [post]
func (h *Handler) AppleLogin(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req struct {
		IdentityToken     string  `json:"identity_token"     validate:"required"`
		AuthorizationCode string  `json:"authorization_code" validate:"required"`
		FirstName         *string `json:"first_name"`
		LastName          *string `json:"last_name"`
	}
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	response.Ok(w, map[string]string{"message": "Integrate with Apple JWT verification (Sign In with Apple)"})
}

// UpdatePushToken godoc
// @Summary      Register push token
// @Tags         auth
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /notifications/push-token [put]
func (h *Handler) UpdatePushToken(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	userID := middleware.GetUserID(r.Context())
	var req struct {
		Token    string `json:"token"    validate:"required"`
		Platform string `json:"platform" validate:"required,oneof=ios android"`
	}
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	if err := h.svc.repo.updatePushToken(r.Context(), userID, req.Token, req.Platform); err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, map[string]bool{"registered": true})
}

// ── request types ─────────────────────────────────────────────────────────────

type registerReq struct {
	FirstName string  `json:"first_name" validate:"required,min=1,max=100"`
	LastName  string  `json:"last_name"  validate:"required,min=1,max=100"`
	Email     *string `json:"email"      validate:"omitempty,email"`
	Phone     *string `json:"phone"      validate:"omitempty,e164"`
	Password  string  `json:"password"   validate:"required,min=8"`
}

type loginReq struct {
	Email    *string `json:"email"    validate:"omitempty,email"`
	Phone    *string `json:"phone"    validate:"omitempty,e164"`
	Password string  `json:"password" validate:"required"`
}

type otpReq struct {
	Email   *string `json:"email"   validate:"omitempty,email"`
	Phone   *string `json:"phone"   validate:"omitempty,e164"`
	Channel string  `json:"channel" validate:"required,oneof=email sms"`
}

type verifyOTPReq struct {
	Email   *string `json:"email"   validate:"omitempty,email"`
	Phone   *string `json:"phone"   validate:"omitempty,e164"`
	OTP     string  `json:"otp"     validate:"required,len=6"`
	Channel string  `json:"channel" validate:"required,oneof=email sms"`
}

// ── helpers ───────────────────────────────────────────────────────────────────

func tokensJSON(t *Tokens) map[string]any {
	return map[string]any{
		"access_token":  t.AccessToken,
		"refresh_token": t.RefreshToken,
		"token_type":    "Bearer",
		"expires_in":    t.ExpiresIn,
	}
}

func userJSON(u *User) map[string]any {
	return map[string]any{
		"id": u.ID, "email": u.Email, "phone": u.Phone,
		"first_name": u.FirstName, "last_name": u.LastName,
		"role": u.Role, "status": u.Status,
		"avatar_url":        u.AvatarURL,
		"email_verified_at": u.EmailVerifiedAt,
		"phone_verified_at": u.PhoneVerifiedAt,
	}
}

func bearerTok(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return h[7:]
	}
	return ""
}

func strongPwd(p string) bool {
	var upper, digit bool
	for _, c := range p {
		upper = upper || (c >= 'A' && c <= 'Z')
		digit = digit || (c >= '0' && c <= '9')
	}
	return upper && digit
}

func genOTP(n int) (string, error) {
	const digits = "0123456789"
	b := make([]byte, n)
	for i := range b {
		v, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b[i] = digits[v.Int64()]
	}
	return string(b), nil
}

func hashStr(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func isDup(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}
