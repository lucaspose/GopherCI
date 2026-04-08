package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lucaspose/goci/internal/auth"
	"github.com/lucaspose/goci/internal/db/repository"
	"github.com/lucaspose/goci/internal/api/response"
)

type AuthHandler struct {
	users repository.UserRepository
	auth  *auth.Service
}

type LoginRequest struct {
	Email string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn int `json:"expires_in"`
}

func NewAuthHandler(users repository.UserRepository, auth *auth.Service) *AuthHandler {
	return &AuthHandler{
		users: users,
		auth:  auth,
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJSONError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Email == "" || req.Password == "" {
		response.WriteJSONError(w, http.StatusBadRequest, "email and password required")
		return
	}
	user, err := h.users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.WriteJSONError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		response.WriteJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	err = h.auth.CheckPassword(user.PasswordHash, req.Password)
	if err != nil {
		response.WriteJSONError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := h.auth.GenerateAccessToken(user.ID, string(user.Role))
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	response.WriteJSON(w, http.StatusOK, LoginResponse{
		AccessToken: token,
		ExpiresIn: h.auth.GetAccessTokenExpiry(),
	})
}
