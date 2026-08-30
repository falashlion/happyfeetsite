package token

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/happyfeet/api/pkg/config"
)

var (
	ErrExpired   = errors.New("token expired")
	ErrInvalid   = errors.New("token invalid")
	ErrWrongType = errors.New("token wrong type")
)

type TokenType string

const (
	TypeAccess  TokenType = "access"
	TypeRefresh TokenType = "refresh"
)

type Claims struct {
	jwt.RegisteredClaims
	UserID string    `json:"uid"`
	Role   string    `json:"role"`
	Type   TokenType `json:"typ"`
	JTI    string    `json:"jti"`
}

type Maker struct{ cfg *config.JWTConfig }

func NewMaker(cfg *config.JWTConfig) *Maker { return &Maker{cfg} }

func (m *Maker) CreateAccessToken(userID, role string) (string, *Claims, error) {
	return m.mint(userID, role, TypeAccess, m.cfg.AccessTokenExpiry, m.cfg.AccessSecret)
}

func (m *Maker) CreateRefreshToken(userID, role string) (string, *Claims, error) {
	return m.mint(userID, role, TypeRefresh, m.cfg.RefreshTokenExpiry, m.cfg.RefreshSecret)
}

func (m *Maker) mint(userID, role string, typ TokenType, exp time.Duration, secret string) (string, *Claims, error) {
	jti := uuid.NewString()
	now := time.Now()
	c := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(exp)),
			Issuer:    "happyfeet-api",
			ID:        jti,
		},
		UserID: userID, Role: role, Type: typ, JTI: jti,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		return "", nil, fmt.Errorf("token: sign: %w", err)
	}
	return signed, c, nil
}

func (m *Maker) VerifyAccessToken(s string) (*Claims, error) {
	return m.verify(s, TypeAccess, m.cfg.AccessSecret)
}

func (m *Maker) VerifyRefreshToken(s string) (*Claims, error) {
	return m.verify(s, TypeRefresh, m.cfg.RefreshSecret)
}

func (m *Maker) verify(s string, want TokenType, secret string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(s, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("bad alg: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpired
		}
		return nil, ErrInvalid
	}
	c, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return nil, ErrInvalid
	}
	if c.Type != want {
		return nil, ErrWrongType
	}
	return c, nil
}

func (m *Maker) AccessExpirySeconds() int {
	return int(m.cfg.AccessTokenExpiry.Seconds())
}
