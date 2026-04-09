package api

import (
	"net/http"

	"github.com/lucaspose/goci/internal/api/handler"
	"github.com/lucaspose/goci/internal/api/middleware"
	"github.com/lucaspose/goci/internal/api/response"
	"github.com/lucaspose/goci/internal/auth"
	"golang.org/x/time/rate"
)

func NewRouter(handlers *handler.Handlers, authService *auth.Service) http.Handler {
	mux := http.NewServeMux()
	rateLimiter := middleware.NewRateLimiter(rate.Limit(2), 2)
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handlers.User.CreateUser(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.User.DeleteUser))
			handler.ServeHTTP(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.Job.CreateJob))
			handler.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.Job.GetJobs))
			handler.ServeHTTP(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.Job.GetJob))
			handler.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodDelete {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.Job.DeleteJob))
			handler.ServeHTTP(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/ssh-keys", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.SshKey.CreateSSHKey))
			handler.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.SshKey.GetSSHKeys))
			handler.ServeHTTP(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/ssh-keys/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.SshKey.DeleteSSHKey))
			handler.ServeHTTP(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	return middleware.Logging()(rateLimiter.Limit()(mux))
}
