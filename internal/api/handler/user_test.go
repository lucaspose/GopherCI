package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appcontext "github.com/lucaspose/goci/internal/api/context"
	"github.com/lucaspose/goci/internal/models"
)

func TestCreateUser(t *testing.T) {
	t.Run("valid request creates user", func(t *testing.T) {
		repo := newMockUserRepo()
		h := NewUserHandler(repo)

		body, _ := json.Marshal(createUserRequest{Email: "bob@example.com", Password: "pass1234"})
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		h.CreateUser(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d — body: %s", rr.Code, rr.Body.String())
		}
		var resp createUserResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("could not decode response: %v", err)
		}
		if resp.ID == "" {
			t.Error("expected non-empty ID")
		}
		if resp.Email != "bob@example.com" {
			t.Errorf("expected email %q, got %q", "bob@example.com", resp.Email)
		}
	})

	t.Run("missing email returns 400", func(t *testing.T) {
		repo := newMockUserRepo()
		h := NewUserHandler(repo)

		body, _ := json.Marshal(createUserRequest{Password: "pass1234"})
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		h.CreateUser(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("missing password returns 400", func(t *testing.T) {
		repo := newMockUserRepo()
		h := NewUserHandler(repo)

		body, _ := json.Marshal(createUserRequest{Email: "bob@example.com"})
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		h.CreateUser(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("invalid json returns 400", func(t *testing.T) {
		repo := newMockUserRepo()
		h := NewUserHandler(repo)

		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader([]byte("not-json")))
		rr := httptest.NewRecorder()
		h.CreateUser(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("repo error returns 500", func(t *testing.T) {
		repo := newMockUserRepo()
		repo.createErr = errors.New("db down")
		h := NewUserHandler(repo)

		body, _ := json.Marshal(createUserRequest{Email: "bob@example.com", Password: "pass1234"})
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		h.CreateUser(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", rr.Code)
		}
	})
}

func TestGetMe(t *testing.T) {
	repo := newMockUserRepo()
	existingUser := &models.User{
		ID:        "user-me",
		Email:     "me@example.com",
		Role:      models.RoleUser,
		CreatedAt: time.Now(),
	}
	repo.users[existingUser.ID] = existingUser

	h := NewUserHandler(repo)

	t.Run("returns user when authenticated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		ctx := context.WithValue(req.Context(), appcontext.UserIDKey, "user-me")
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		h.GetMe(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
		}
		var resp GetMeResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("could not decode response: %v", err)
		}
		if resp.ID != "user-me" {
			t.Errorf("expected ID %q, got %q", "user-me", resp.ID)
		}
		if resp.Email != "me@example.com" {
			t.Errorf("expected email %q, got %q", "me@example.com", resp.Email)
		}
	})

	t.Run("no user_id in context returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		rr := httptest.NewRecorder()
		h.GetMe(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("user not found in repo returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		ctx := context.WithValue(req.Context(), appcontext.UserIDKey, "ghost-user")
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		h.GetMe(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})
}
