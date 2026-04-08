package api

import (
	"net/http"

	"github.com/lucaspose/goci/internal/api/handler"
	"github.com/lucaspose/goci/internal/api/middleware"
	"github.com/lucaspose/goci/internal/api/response"
	"github.com/lucaspose/goci/internal/auth"
)

func NewRouter(handlers *handler.Handlers, authService *auth.Service) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handlers.User.CreateUser(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request)  {
		if r.Method == http.MethodPost {
			handlers.Auth.Login(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.User.GetMe))
			handler.ServeHTTP(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	return mux
}
