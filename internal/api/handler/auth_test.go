package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lucaspose/goci/internal/auth"
	"github.com/lucaspose/goci/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func newTestAuth() *auth.Service {
	return auth.NewService("test-secret", 15*time.Minute)
}

func seedUser(repo *mockUserRepo, email, password string) *models.User {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	u := &models.User{
		ID:           "user-001",
		Email:        email,
		PasswordHash: string(hash),
		Role:         models.RoleUser,
		CreatedAt:    time.Now(),
	}
	repo.users[u.ID] = u
	repo.byEmail[u.Email] = u
	return u
}

func TestLogin(t *testing.T) {
	repo := newMockUserRepo()
	svc := newTestAuth()
	h := NewAuthHandler(repo, svc)

	seedUser(repo, "alice@example.com", "password123")

	t.Run("valid credentials", func(t *testing.T) {
		body, _ := json.Marshal(LoginRequest{Email: "alice@example.com", Password: "password123"})
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		h.Login(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
		}
		var resp LoginResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("could not decode response: %v", err)
		}
		if resp.AccessToken == "" {
			t.Error("expected non-empty access_token")
		}
		if resp.ExpiresIn != 900 {
			t.Errorf("expected expires_in 900, got %d", resp.ExpiresIn)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		body, _ := json.Marshal(LoginRequest{Email: "alice@example.com", Password: "wrongpass"})
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		h.Login(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("unknown email", func(t *testing.T) {
		body, _ := json.Marshal(LoginRequest{Email: "nobody@example.com", Password: "password123"})
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		h.Login(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("missing email", func(t *testing.T) {
		body, _ := json.Marshal(LoginRequest{Password: "password123"})
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		h.Login(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("missing password", func(t *testing.T) {
		body, _ := json.Marshal(LoginRequest{Email: "alice@example.com"})
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		h.Login(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("invalid json body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader([]byte("not-json")))
		rr := httptest.NewRecorder()
		h.Login(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})
}
