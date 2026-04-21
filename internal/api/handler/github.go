package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lucaspose/goci/internal/api/response"
	"github.com/lucaspose/goci/internal/auth"
	"github.com/lucaspose/goci/internal/db/repository"
	"github.com/lucaspose/goci/internal/models"
)

type GitHubHandler struct {
	ClientID       string
	ClientSecret   string
	UserRepository repository.UserRepository
	RefreshTokens  repository.RefreshTokenRepository
	AuthService    *auth.Service
}

type githubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type GitHubRepo struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	CloneUrl string `json:"clone_url"`
	SSHURL   string `json:"ssh_url"`
	Private  bool   `json:"private"`
}

type githubUser struct {
	Email string `json:"email"`
	Login string `json:"login"`
}

type GitHubOrg struct {
	Login string `json:"login"`
}

func NewGitHubHandler(clientID, clientSecret string, userRepo repository.UserRepository, refreshTokens repository.RefreshTokenRepository, authService *auth.Service) *GitHubHandler {
	return &GitHubHandler{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		UserRepository: userRepo,
		RefreshTokens:  refreshTokens,
		AuthService:    authService,
	}
}

func (h *GitHubHandler) issueTokenPair(r *http.Request, userID string, role string) (LoginResponse, error) {
	accessToken, err := h.AuthService.GenerateAccessToken(userID, role)
	if err != nil {
		return LoginResponse{}, err
	}
	refreshToken, refreshTokenHash, refreshExpiresAt, err := h.AuthService.GenerateRefreshToken()
	if err != nil {
		return LoginResponse{}, err
	}
	err = h.RefreshTokens.Create(r.Context(), &models.RefreshToken{
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
		ExpiresIn:        h.AuthService.GetAccessTokenExpiry(),
		RefreshExpiresIn: h.AuthService.GetRefreshTokenExpiry(),
	}, nil
}

func (h *GitHubHandler) RedirectToGitHub(w http.ResponseWriter, r *http.Request) {
	url := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&scope=repo,read:org&redirect_uri=http://localhost:8080/auth/github/callback",
		h.ClientID,
	)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *GitHubHandler) exchangeCode(code string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"client_id":     h.ClientID,
		"client_secret": h.ClientSecret,
		"code":          code,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github token exchange failed: %d", resp.StatusCode)
	}
	var tokenResp githubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}
	return tokenResp.AccessToken, nil
}

func (h *GitHubHandler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	token, err := h.exchangeCode(code)
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	http.Redirect(w, r, fmt.Sprintf("http://localhost:9999/callback?token=%s", token), http.StatusTemporaryRedirect)
}

func (h *GitHubHandler) GetOrganizations(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		response.WriteJSONError(w, http.StatusBadRequest, "no token")
		return
	}
	req, err := http.NewRequest("GET", "https://api.github.com/user/orgs", nil)
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		response.WriteJSONError(w, http.StatusBadRequest, "bad request")
		return
	}
	var orgs []GitHubOrg
	if err := json.NewDecoder(resp.Body).Decode(&orgs); err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusOK, orgs)
}

func (h *GitHubHandler) GetRepositories(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		response.WriteJSONError(w, http.StatusBadRequest, "no token")
		return
	}
	req, err := http.NewRequest("GET", "https://api.github.com/user/repos?type=all", nil)
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		response.WriteJSONError(w, http.StatusBadRequest, "bad request")
		return
	}
	var repos []GitHubRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusOK, repos)
}

func (h *GitHubHandler) getGitHubUser(token string) (*githubUser, error) {
	url := "https://api.github.com/user"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github user fetch failed: %d", resp.StatusCode)
	}
	var user githubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (h *GitHubHandler) ExchangeGitHubToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GithubToken string `json:"github_token"`
	}
	defer r.Body.Close()
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	githubUser, err := h.getGitHubUser(body.GithubToken)
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	user, err := h.UserRepository.GetByEmail(r.Context(), githubUser.Email)
	if errors.Is(err, repository.ErrNotFound) {
		newUser := &models.User{
			ID:           uuid.NewString(),
			Email:        githubUser.Email,
			PasswordHash: "",
			Role:         models.RoleUser,
			CreatedAt:    time.Now(),
		}
		h.UserRepository.Create(r.Context(), newUser)
		user = newUser
	} else if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	tokens, err := h.issueTokenPair(r, user.ID, string(user.Role))
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusOK, tokens)
}
