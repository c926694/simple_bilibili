package jwt

import (
	"crypto/rand"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"simple_bilibili/backend/internal/config"
)

const (
	Access  = "access"
	Refresh = "refresh"
)

var ErrInvalidClaims = errors.New("凭证声明无效")

type Claims struct {
	UserID    int64  `json:"user_id,string"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

type Token struct {
	Value     string
	ID        string
	ExpiresAt time.Time
	ExpiresIn int64
}

type Pair struct {
	Access  Token
	Refresh Token
}

type Manager struct {
	key        []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
	parser     *jwt.Parser
}

func New(cfg config.JWT) *Manager {
	return &Manager{
		key: []byte(cfg.Secret), issuer: cfg.Issuer,
		accessTTL: cfg.AccessTTL, refreshTTL: cfg.RefreshTTL,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithIssuer(cfg.Issuer),
			jwt.WithExpirationRequired(),
		),
	}
}

func (m *Manager) Issue(userID int64) (Pair, error) {
	now := time.Now().UTC().Truncate(time.Second)
	access, err := m.sign(userID, Access, now, m.accessTTL)
	if err != nil {
		return Pair{}, err
	}
	refresh, err := m.sign(userID, Refresh, now, m.refreshTTL)
	if err != nil {
		return Pair{}, err
	}
	return Pair{Access: access, Refresh: refresh}, nil
}

func (m *Manager) Parse(value, tokenType string) (*Claims, error) {
	claims := &Claims{}
	_, err := m.parser.ParseWithClaims(value, claims, func(*jwt.Token) (any, error) {
		return m.key, nil
	})
	if err != nil {
		return nil, err
	}
	if claims.UserID <= 0 || claims.ID == "" || claims.TokenType != tokenType {
		return nil, ErrInvalidClaims
	}
	return claims, nil
}

func (m *Manager) sign(userID int64, tokenType string, now time.Time, ttl time.Duration) (Token, error) {
	expiry := now.Add(ttl).Truncate(time.Second)
	claims := Claims{
		UserID: userID, TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: m.issuer, ID: rand.Text(),
			IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expiry),
		},
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.key)
	if err != nil {
		return Token{}, err
	}
	return Token{Value: value, ID: claims.ID, ExpiresAt: expiry, ExpiresIn: expiry.Unix() - now.Unix()}, nil
}
