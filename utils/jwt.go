package utils

import (
	"fmt"
	"github.com/Romain-GUILLEMOT/WhispyrBack/config"
	"github.com/google/uuid"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

type CustomClaims struct {
	UserID    string    `json:"user_id"`
	TokenType TokenType `json:"type"`
	Device    string    `json:"device"`
	jwt.RegisteredClaims
}

func generateToken(userID string, device string, tokenType TokenType, ttl time.Duration) (string, error) {
	cfg := config.GetConfig()

	claims := CustomClaims{
		UserID:    userID,
		TokenType: tokenType,
		Device:    device,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("can't sign the token: %w", err)
	}
	if tokenType == AccessToken {
		if err := RedisSet("access_token:"+signed, userID+device, ttl); err != nil {
			return "", fmt.Errorf("can't save the token: %w", err)

		}
	} else {
		if err := RedisSet("refresh_token:"+signed, userID+device, ttl); err != nil {
			return "", fmt.Errorf("can't save the token: %w", err)
		}
	}

	return signed, nil
}

func GenerateAccessToken(userID string, device string) (string, error) {
	return generateToken(userID, device, AccessToken, 15*time.Minute)
}

func GenerateRefreshToken(userID string, device string) (string, error) {
	return generateToken(userID, device, RefreshToken, 180*24*time.Hour)
}

func VerifyToken(tokenStr string, tokenType TokenType) (*CustomClaims, error, time.Duration) {
	cfg := config.GetConfig()
	// Check if the token is in Redis
	key := "refresh_token:" + tokenStr
	if tokenType == AccessToken {
		key = "access_token:" + tokenStr
	}

	redisTTL, err := RedisTTL(key)
	if err != nil {
		return nil, fmt.Errorf("failed to check token existence: %w", err), 0
	}

	if redisTTL < 1 {
		return nil, fmt.Errorf("token not found"), 0
	}
	parsed, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil {
		return nil, err, 0
	}

	claims, ok := parsed.Claims.(*CustomClaims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid token"), 0
	}
	stored, err := RedisGet(key)
	if err != nil || stored != claims.UserID+claims.Device {
		return nil, fmt.Errorf("invalid token"), 0
	}
	return claims, nil, redisTTL
}

func RefreshAccessToken(refreshToken string) (string, string, error) {
	claims, err, ttl := VerifyToken(refreshToken, RefreshToken)
	if err != nil {
		return "", "", err
	}
	if ttl < 1 {
		return "", "", fmt.Errorf("expired token")
	}

	if claims.TokenType != RefreshToken {
		return "", "", fmt.Errorf("not a refresh token")
	}
	accessToken, err := GenerateAccessToken(claims.UserID, claims.Device)
	newRefreshToken := ""
	if ttl < 30*24*time.Hour {
		err = RedisDel("refresh_token:" + refreshToken)
		if err != nil {
			return "", "", err
		}
		newRefreshToken, err = GenerateRefreshToken(claims.UserID, claims.Device)
		if err != nil {
			return "", "", err
		}
		Info("🆕 Refresh token", "user_id", claims.UserID, "device", claims.Device)

	}
	return accessToken, newRefreshToken, err
}

func CheckUserToken(accessToken string) (*uuid.UUID, bool) {
	claims, err, ttl := VerifyToken(accessToken, AccessToken)
	if err != nil {
		return nil, true
	}
	if claims.TokenType != AccessToken {
		return nil, true
	}
	if ttl < 1 {
		return nil, true
	}
	parsedUUID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, false
	}
	return &parsedUUID, ttl < 60
}
func ShouldRefreshAccessToken(tokenStr string) (bool, error) {
	cfg := config.GetConfig()
	parsed, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.JWTSecret), nil
	}, jwt.WithLeeway(5*time.Second)) // marge de sécurité

	if err != nil {
		return true, nil
	}

	claims, ok := parsed.Claims.(*CustomClaims)
	if !ok || !parsed.Valid {
		return false, fmt.Errorf("invalid token")
	}
	if claims.ExpiresAt == nil {
		return false, fmt.Errorf("missing expiration")
	}

	// Refresh si expire dans moins de 60s
	return time.Until(claims.ExpiresAt.Time) < 60*time.Second, nil
}
