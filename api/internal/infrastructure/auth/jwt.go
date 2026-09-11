package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID string `json:"sub"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret []byte
	expire time.Duration
}

func NewJWTManager(secret string, expire time.Duration) *JWTManager {
	return &JWTManager{
		secret: []byte(secret),
		expire: expire,
	}
}

// SignedToken 签发结果
type SignedToken struct {
	Value     string
	ExpiresAt time.Time
}

func (m *JWTManager) Sign(userID string) (*SignedToken, error) {
	now := time.Now()
	expiresAt := now.Add(m.expire)
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	value, err := token.SignedString(m.secret)
	if err != nil {
		return nil, err
	}
	return &SignedToken{Value: value, ExpiresAt: expiresAt}, nil
}

func (m *JWTManager) Verify(tokenString string) (*Claims, error) {
	return m.parseClaims(tokenString, false)
}

// VerifyAllowExpired 校验签名并解析 claims，不校验 exp（用于 refresh）
func (m *JWTManager) VerifyAllowExpired(tokenString string) (*Claims, error) {
	return m.parseClaims(tokenString, true)
}

func (m *JWTManager) parseClaims(tokenString string, allowExpired bool) (*Claims, error) {
	opts := []jwt.ParserOption{}
	if allowExpired {
		opts = append(opts, jwt.WithoutClaimsValidation())
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	}, opts...)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || claims.UserID == "" {
		return nil, fmt.Errorf("invalid token")
	}
	if !allowExpired && !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}
