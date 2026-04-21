package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lucaspose/goci/internal/api/response"
	"github.com/lucaspose/goci/internal/auth"
	"github.com/lucaspose/goci/internal/db/repository"
	"github.com/lucaspose/goci/internal/models"
)

type AuthHandler struct {
	users         repository.UserRepository
	refreshTokens repository.RefreshTokenRepository
	auth          *auth.Service
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func NewAuthHandler(users repository.UserRepository, refreshTokens repository.RefreshTokenRepository, auth *auth.Service) *AuthHandler {
	return &AuthHandler{
		users:         users,
		refreshTokens: refreshTokens,
		auth:          auth,
	}
}

func (h *AuthHandler) issueTokenPair(ctx context.Context, userID string, role string) (LoginResponse, error) {
	accessToken, err := h.auth.GenerateAccessToken(userID, role)
	if err != nil {
		return LoginResponse{}, err
	}
	refreshToken, refreshTokenHash, refreshExpiresAt, err := h.auth.GenerateRefreshToken()
	if err != nil {
		return LoginResponse{}, err
	}
	err = h.refreshTokens.Create(ctx, &models.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    userID,
		TokenHash: refreshTokenHash,
		CreatedAt: time.Now(),
		ExpiresAt: refreshExpiresAt,
	})
	if err != nil {
		return LoginResponse{}, err
	}

	return LoginResponse{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		ExpiresIn:        h.auth.GetAccessTokenExpiry(),
		RefreshExpiresIn: h.auth.GetRefreshTokenExpiry(),
	}, nil
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
	tokens, err := h.issueTokenPair(r.Context(), user.ID, string(user.Role))
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	response.WriteJSON(w, http.StatusOK, tokens)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJSONError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.RefreshToken == "" {
		response.WriteJSONError(w, http.StatusBadRequest, "refresh_token required")
		return
	}

	now := time.Now()
	hash := h.auth.HashRefreshToken(req.RefreshToken)
	storedToken, err := h.refreshTokens.GetByTokenHash(r.Context(), hash)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.WriteJSONError(w, http.StatusUnauthorized, "invalid refresh token")
			return
		}
		response.WriteJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if storedToken.RevokedAt != nil || now.After(storedToken.ExpiresAt) {
		if storedToken.RevokedAt == nil {
			_ = h.refreshTokens.RevokeByTokenHash(r.Context(), hash, now)
		}
		response.WriteJSONError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	user, err := h.users.GetByID(r.Context(), storedToken.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.WriteJSONError(w, http.StatusUnauthorized, "invalid refresh token")
			return
		}
		response.WriteJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := h.refreshTokens.RevokeByTokenHash(r.Context(), hash, now); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.WriteJSONError(w, http.StatusUnauthorized, "invalid refresh token")
			return
		}
		response.WriteJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	tokens, err := h.issueTokenPair(r.Context(), user.ID, string(user.Role))
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	response.WriteJSON(w, http.StatusOK, tokens)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJSONError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.RefreshToken == "" {
		response.WriteJSONError(w, http.StatusBadRequest, "refresh_token required")
		return
	}
	hash := h.auth.HashRefreshToken(req.RefreshToken)
	err := h.refreshTokens.RevokeByTokenHash(r.Context(), hash, time.Now())
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
