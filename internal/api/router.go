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
	mux.HandleFunc("/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handlers.Auth.Refresh(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handlers.Auth.Logout(w, r)
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
	mux.HandleFunc("/jobs/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.Job.StreamJobs))
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
	mux.HandleFunc("/jobs/{id}/artifact", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.Job.DownloadJobArtifact))
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
	mux.HandleFunc("/organizations", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.Organizations.CreateOrganization))
			handler.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.Organizations.GetOrganizations))
			handler.ServeHTTP(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/organizations/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.Organizations.DeleteOrganization))
			handler.ServeHTTP(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/organizations/{orgId}/repositories", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.Repo.CreateRepository))
			handler.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.Repo.GetRepositories))
			handler.ServeHTTP(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/organizations/{orgId}/repositories/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			handler := middleware.RequireAuth(authService)(http.HandlerFunc(handlers.Repo.DeleteRepository))
			handler.ServeHTTP(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/auth/github", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handlers.Github.RedirectToGitHub(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/auth/github/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handlers.Github.Callback(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/auth/github/organizations", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handlers.Github.GetOrganizations(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/auth/github/repositories", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handlers.Github.GetRepositories(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	mux.HandleFunc("/auth/github/exchange", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handlers.Github.ExchangeGitHubToken(w, r)
			return
		}
		response.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	return middleware.Logging()(rateLimiter.Limit()(mux))
}
