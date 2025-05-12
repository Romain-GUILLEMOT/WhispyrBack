package utils

import (
	"crypto/sha256"
	"encoding/hex"
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
		err := revokeTokensByIndex(userID, device, AccessToken)
		if err != nil {
			return "", fmt.Errorf("can't remove old tokens: %w", err)
		}

	}
	if tokenType == RefreshToken {
		err := revokeTokensByIndex(userID, device, RefreshToken)
		if err != nil {
			return "", fmt.Errorf("can't remove old tokens: %w", err)
		}

	}
	indexKey := "device_tokens:" + userID + ":" + device

	var redisKey string
	if tokenType == AccessToken {
		redisKey = "access_token:" + signed
	} else {
		redisKey = "refresh_token:" + signed
	}

	if err := RedisSet(redisKey, userID+device, ttl, indexKey); err != nil {
		return "", fmt.Errorf("can't save the token: %w", err)
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
	newRefreshToken := refreshToken
	Info("TTL du refresh token : " + ttl.String())
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

func GenerateDeviceID(ua string, accept string, lang string, encoding string, ip string) string {
	rawID := fmt.Sprintf("%s|%s|%s|%s|%s", ua, accept, lang, encoding, ip)
	hash := sha256.Sum256([]byte(rawID))
	deviceID := hex.EncodeToString(hash[:])
	return deviceID
}

func revokeTokensByIndex(userID, deviceID string, tokenType TokenType) error {
	indexKey := "device_tokens:" + userID + ":" + deviceID
	tokens, err := Redis.SMembers(Ctx, indexKey).Result()
	if err != nil {
		return err
	}

	if len(tokens) == 0 {
		return nil
	}

	prefix := "access_token:"
	if tokenType == RefreshToken {
		prefix = "refresh_token:"
	}

	pipe := Redis.TxPipeline()
	for _, t := range tokens {
		pipe.Del(Ctx, prefix+t)
	}
	pipe.Del(Ctx, indexKey)
	_, err = pipe.Exec(Ctx)
	return err
}
