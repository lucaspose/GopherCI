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
	refreshRepo := newMockRefreshTokenRepo()
	svc := newTestAuth()
	h := NewAuthHandler(repo, refreshRepo, svc)

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
		if resp.RefreshToken == "" {
			t.Error("expected non-empty refresh_token")
		}
		if resp.ExpiresIn != 900 {
			t.Errorf("expected expires_in 900, got %d", resp.ExpiresIn)
		}
		if resp.RefreshExpiresIn != svc.GetRefreshTokenExpiry() {
			t.Errorf("expected refresh_expires_in %d, got %d", svc.GetRefreshTokenExpiry(), resp.RefreshExpiresIn)
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

func TestRefresh(t *testing.T) {
	repo := newMockUserRepo()
	refreshRepo := newMockRefreshTokenRepo()
	svc := newTestAuth()
	h := NewAuthHandler(repo, refreshRepo, svc)

	seedUser(repo, "alice@example.com", "password123")

	loginBody, _ := json.Marshal(LoginRequest{Email: "alice@example.com", Password: "password123"})
	loginReq := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody))
	loginRR := httptest.NewRecorder()
	h.Login(loginRR, loginReq)
	if loginRR.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d", loginRR.Code)
	}
	var loginResp LoginResponse
	if err := json.NewDecoder(loginRR.Body).Decode(&loginResp); err != nil {
		t.Fatalf("could not decode login response: %v", err)
	}

	t.Run("valid refresh rotates token", func(t *testing.T) {
		body, _ := json.Marshal(RefreshTokenRequest{RefreshToken: loginResp.RefreshToken})
		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		h.Refresh(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
		}
		var refreshed LoginResponse
		if err := json.NewDecoder(rr.Body).Decode(&refreshed); err != nil {
			t.Fatalf("could not decode refresh response: %v", err)
		}
		if refreshed.AccessToken == "" {
			t.Fatal("expected non-empty access_token")
		}
		if refreshed.RefreshToken == "" {
			t.Fatal("expected non-empty refresh_token")
		}
		if refreshed.RefreshToken == loginResp.RefreshToken {
			t.Fatal("expected refresh token rotation")
		}

		reuseBody, _ := json.Marshal(RefreshTokenRequest{RefreshToken: loginResp.RefreshToken})
		reuseReq := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(reuseBody))
		reuseRR := httptest.NewRecorder()
		h.Refresh(reuseRR, reuseReq)
		if reuseRR.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on reused token, got %d", reuseRR.Code)
		}
	})

	t.Run("missing refresh token", func(t *testing.T) {
		body, _ := json.Marshal(RefreshTokenRequest{})
		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		h.Refresh(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("unknown refresh token", func(t *testing.T) {
		body, _ := json.Marshal(RefreshTokenRequest{RefreshToken: "bad-token"})
		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		h.Refresh(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})
}

func TestLogout(t *testing.T) {
	repo := newMockUserRepo()
	refreshRepo := newMockRefreshTokenRepo()
	svc := newTestAuth()
	h := NewAuthHandler(repo, refreshRepo, svc)

	seedUser(repo, "alice@example.com", "password123")

	loginBody, _ := json.Marshal(LoginRequest{Email: "alice@example.com", Password: "password123"})
	loginReq := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody))
	loginRR := httptest.NewRecorder()
	h.Login(loginRR, loginReq)
	if loginRR.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d", loginRR.Code)
	}
	var loginResp LoginResponse
	if err := json.NewDecoder(loginRR.Body).Decode(&loginResp); err != nil {
		t.Fatalf("could not decode login response: %v", err)
	}

	body, _ := json.Marshal(RefreshTokenRequest{RefreshToken: loginResp.RefreshToken})
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Logout(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}

	refreshBody, _ := json.Marshal(RefreshTokenRequest{RefreshToken: loginResp.RefreshToken})
	refreshReq := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(refreshBody))
	refreshRR := httptest.NewRecorder()
	h.Refresh(refreshRR, refreshReq)

	if refreshRR.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", refreshRR.Code)
	}
}
