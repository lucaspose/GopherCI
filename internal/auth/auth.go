package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Service struct{
	secret string
	accessTokenExpiry time.Duration
}

func NewService(secret string, expiry time.Duration) *Service {
	return &Service{
		secret: secret,
		accessTokenExpiry: expiry,
	}
}

func (s *Service) CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func (s *Service) GetAccessTokenExpiry() int {
	return int(s.accessTokenExpiry.Seconds())
}

func (s *Service) GenerateAccessToken(userID string, role string) (string, error) {
	expiration := time.Now().Add(s.accessTokenExpiry).Unix()

	claims := jwt.MapClaims{
		"user_id": userID,
		"role": role,
		"exp": expiration,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256,claims)
	signedToken, err := token.SignedString([]byte(s.secret))
	if err != nil {
		return "", fmt.Errorf("generating access token error: %w", err)
	}
	return signedToken, nil
}