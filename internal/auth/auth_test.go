package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func newTestService() *Service {
	return NewService("test-secret", 15*time.Minute)
}

func TestNewService(t *testing.T) {
	s := newTestService()
	if s.secret != "test-secret" {
		t.Errorf("expected secret %q, got %q", "test-secret", s.secret)
	}
	if s.accessTokenExpiry != 15*time.Minute {
		t.Errorf("expected expiry %v, got %v", 15*time.Minute, s.accessTokenExpiry)
	}
	if s.refreshTokenExpiry <= 0 {
		t.Error("expected positive refresh token expiry")
	}
}

func TestGetSecret(t *testing.T) {
	s := newTestService()
	if s.GetSecret() != "test-secret" {
		t.Errorf("expected %q, got %q", "test-secret", s.GetSecret())
	}
}

func TestGetAccessTokenExpiry(t *testing.T) {
	s := newTestService()
	if s.GetAccessTokenExpiry() != 900 {
		t.Errorf("expected 900, got %d", s.GetAccessTokenExpiry())
	}
}

func TestGetRefreshTokenExpiry(t *testing.T) {
	s := newTestService()
	if s.GetRefreshTokenExpiry() <= 0 {
		t.Errorf("expected positive refresh token expiry, got %d", s.GetRefreshTokenExpiry())
	}
}

func TestCheckPassword(t *testing.T) {
	s := newTestService()
	password := "supersecret"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("valid password", func(t *testing.T) {
		if err := s.CheckPassword(string(hash), password); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		if err := s.CheckPassword(string(hash), "wrongpassword"); err == nil {
			t.Error("expected error for wrong password, got nil")
		}
	})

	t.Run("empty password", func(t *testing.T) {
		if err := s.CheckPassword(string(hash), ""); err == nil {
			t.Error("expected error for empty password, got nil")
		}
	})
}

func TestGenerateAccessToken(t *testing.T) {
	s := newTestService()

	t.Run("valid token", func(t *testing.T) {
		token, err := s.GenerateAccessToken("user-123", "user")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if token == "" {
			t.Error("expected non-empty token")
		}
	})

	t.Run("token contains correct claims", func(t *testing.T) {
		token, err := s.GenerateAccessToken("user-456", "admin")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
			return []byte(s.secret), nil
		})
		if err != nil {
			t.Fatalf("failed to parse token: %v", err)
		}

		claims, ok := parsed.Claims.(jwt.MapClaims)
		if !ok {
			t.Fatal("could not cast claims")
		}
		if claims["user_id"] != "user-456" {
			t.Errorf("expected user_id %q, got %v", "user-456", claims["user_id"])
		}
		if claims["role"] != "admin" {
			t.Errorf("expected role %q, got %v", "admin", claims["role"])
		}
	})

	t.Run("token is invalid with wrong secret", func(t *testing.T) {
		token, err := s.GenerateAccessToken("user-789", "user")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		_, err = jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
			return []byte("wrong-secret"), nil
		})
		if err == nil {
			t.Error("expected error with wrong secret, got nil")
		}
	})
}

func TestGenerateRefreshToken(t *testing.T) {
	s := newTestService()

	t.Run("returns token hash and expiry", func(t *testing.T) {
		token, hash, expiresAt, err := s.GenerateRefreshToken()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if token == "" {
			t.Fatal("expected non-empty token")
		}
		if hash == "" {
			t.Fatal("expected non-empty hash")
		}
		if expiresAt.Before(time.Now()) {
			t.Fatal("expected future expiry")
		}
		expectedHash := s.HashRefreshToken(token)
		if hash != expectedHash {
			t.Fatalf("expected hash %q, got %q", expectedHash, hash)
		}
	})

	t.Run("different tokens produce different hashes", func(t *testing.T) {
		tokenA, hashA, _, err := s.GenerateRefreshToken()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		tokenB, hashB, _, err := s.GenerateRefreshToken()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if tokenA == tokenB {
			t.Fatal("expected different token values")
		}
		if hashA == hashB {
			t.Fatal("expected different token hashes")
		}
	})
}
