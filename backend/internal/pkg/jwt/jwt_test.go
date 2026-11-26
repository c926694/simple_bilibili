package jwt

import (
	"errors"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	"simple_bilibili/backend/internal/config"
)

func TestIssueAndParse(t *testing.T) {
	cfg := config.JWT{Issuer: "simple_bilibili_test", Secret: "test_secret_with_more_than_thirty_two_bytes", AccessTTL: 15 * time.Minute, RefreshTTL: 7 * 24 * time.Hour}
	manager := New(cfg)
	const userID int64 = 9007199254740997
	pair, err := manager.Issue(userID)
	if err != nil {
		t.Fatal(err)
	}
	if pair.Access.ExpiresIn != 900 || pair.Refresh.ExpiresIn != 604800 || pair.Access.ID == pair.Refresh.ID {
		t.Fatal("签发的凭证时效或标识不符")
	}
	claims, err := manager.Parse(pair.Access.Value, Access)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != userID || claims.ID != pair.Access.ID || claims.TokenType != Access {
		t.Fatalf("访问凭证声明不符: %+v", claims)
	}
	if _, err := manager.Parse(pair.Refresh.Value, Refresh); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Parse(pair.Refresh.Value, Access); !errors.Is(err, ErrInvalidClaims) {
		t.Fatalf("凭证类型不符合预期时返回 %v", err)
	}

	cfg.Secret += "_different"
	if _, err := New(cfg).Parse(pair.Access.Value, Access); !errors.Is(err, gojwt.ErrTokenSignatureInvalid) {
		t.Fatalf("凭证签名无效时返回 %v", err)
	}
	cfg.Secret = "test_secret_with_more_than_thirty_two_bytes"
	cfg.Issuer = "other_issuer"
	if _, err := New(cfg).Parse(pair.Access.Value, Access); !errors.Is(err, gojwt.ErrTokenInvalidIssuer) {
		t.Fatalf("凭证签发者无效时返回 %v", err)
	}
	expired, err := manager.sign(userID, Access, time.Now().Add(-time.Hour), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Parse(expired.Value, Access); !errors.Is(err, gojwt.ErrTokenExpired) {
		t.Fatalf("凭证过期时返回 %v", err)
	}
}
