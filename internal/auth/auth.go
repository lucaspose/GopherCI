package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	secret             string
	accessTokenExpiry  time.Duration
	refreshTokenExpiry time.Duration
}

const (
	defaultRefreshTokenExpiry = 30 * 24 * time.Hour
	refreshTokenSize          = 32
)

func NewService(secret string, expiry time.Duration) *Service {
	return NewServiceWithRefresh(secret, expiry, defaultRefreshTokenExpiry)
}

func NewServiceWithRefresh(secret string, accessExpiry, refreshExpiry time.Duration) *Service {
	if refreshExpiry <= 0 {
		refreshExpiry = defaultRefreshTokenExpiry
	}
	return &Service{
		secret:             secret,
		accessTokenExpiry:  accessExpiry,
		refreshTokenExpiry: refreshExpiry,
	}
}

func (s *Service) CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func (s *Service) GetAccessTokenExpiry() int {
	return int(s.accessTokenExpiry.Seconds())
}

func (s *Service) GetRefreshTokenExpiry() int {
	return int(s.refreshTokenExpiry.Seconds())
}

func (s *Service) GenerateAccessToken(userID string, role string) (string, error) {
	expiration := time.Now().Add(s.accessTokenExpiry).Unix()

	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     expiration,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.secret))
	if err != nil {
		return "", fmt.Errorf("generating access token error: %w", err)
	}
	return signedToken, nil
}

func (s *Service) GenerateRefreshToken() (token string, tokenHash string, expiresAt time.Time, err error) {
	raw := make([]byte, refreshTokenSize)
	if _, err = rand.Read(raw); err != nil {
		return "", "", time.Time{}, fmt.Errorf("generate refresh token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	tokenHash = s.HashRefreshToken(token)
	expiresAt = time.Now().Add(s.refreshTokenExpiry)
	return token, tokenHash, expiresAt, nil
}

func (s *Service) HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
