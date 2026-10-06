package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appcontext "github.com/lucaspose/goci/internal/api/context"
	"github.com/lucaspose/goci/internal/auth"
)

func newTestAuthService() *auth.Service {
	return auth.NewService("test-secret", 15*time.Minute)
}

func dummyHandler(t *testing.T, expectedUserID string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(appcontext.UserIDKey).(string)
		if !ok || userID != expectedUserID {
			t.Errorf("expected user_id %q in context, got %q", expectedUserID, userID)
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireAuth(t *testing.T) {
	svc := newTestAuthService()
	middleware := RequireAuth(svc)

	t.Run("no authorization header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called")
		})).ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("malformed authorization header", func(t *testing.T) {
		cases := []string{"token", "Basic abc123", "Bearer", "Bearer a b"}
		for _, h := range cases {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", h)
			rr := httptest.NewRecorder()
			middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("handler should not be called for header %q", h)
			})).ServeHTTP(rr, req)
			if rr.Code != http.StatusUnauthorized {
				t.Errorf("header %q: expected 401, got %d", h, rr.Code)
			}
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer notavalidtoken")
		rr := httptest.NewRecorder()
		middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called")
		})).ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("valid token passes user_id to context", func(t *testing.T) {
		token, err := svc.GenerateAccessToken("user-abc", "user")
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		middleware(dummyHandler(t, "user-abc")).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("token signed with wrong secret is rejected", func(t *testing.T) {
		wrongSvc := auth.NewService("wrong-secret", 15*time.Minute)
		token, err := wrongSvc.GenerateAccessToken("user-xyz", "user")
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called")
		})).ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})
}
