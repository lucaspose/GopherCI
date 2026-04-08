package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/lucaspose/goci/internal/api/context"
	"github.com/lucaspose/goci/internal/api/response"
	"github.com/lucaspose/goci/internal/db/repository"
	"github.com/lucaspose/goci/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type UserHandler struct {
	repo repository.UserRepository
}

type createUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type createUserResponse struct {
	ID string `json:"id"`
	Email string `json:"email"`
}

type GetMeResponse struct {
	ID string `json:"id"`
	Email string `json:"email"`
}

func NewUserHandler(repo repository.UserRepository) *UserHandler {
	return &UserHandler{repo: repo}
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var input createUserRequest

	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Email == "" || input.Password == "" {
		response.WriteJSONError(w, http.StatusBadRequest, "missing email or password")
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	user := models.User{
		ID:           uuid.New().String(),
		Email:        input.Email,
		PasswordHash: string(hashedPassword),
		Role:         models.RoleUser,
		CreatedAt:    time.Now(),
	}
	if err := h.repo.Create(r.Context(), &user); err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "failed to create user")
		return
	}
	response.WriteJSON(w, http.StatusCreated, createUserResponse{
		ID: user.ID,
		Email: user.Email,
	})
}

func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctx.Value(appcontext.UserIDKey).(string)
	if !ok {
		response.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	user, err := h.repo.GetByID(ctx, userID)
	if errors.Is(err, repository.ErrNotFound) {
		response.WriteJSONError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusOK, GetMeResponse{
		ID: user.ID,
		Email: user.Email,
	})
}