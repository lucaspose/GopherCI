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
)

type RepositoryHandler struct {
	RepositoryRepo   repository.RepositoryRepository
	OrganizationRepo repository.OrganizationRepository
}

type CreateRepositoryRequest struct {
	Name string `json:"name"`
	Repo string `json:"repo"`
}

func NewRepositoryHandler(repositoryRepo repository.RepositoryRepository, organizationRepo repository.OrganizationRepository) *RepositoryHandler {
	return &RepositoryHandler{
		RepositoryRepo:   repositoryRepo,
		OrganizationRepo: organizationRepo,
	}
}

func (h *RepositoryHandler) CreateRepository(w http.ResponseWriter, r *http.Request) {
	var req CreateRepositoryRequest
	defer r.Body.Close()
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		response.WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		response.WriteJSONError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Repo == "" {
		response.WriteJSONError(w, http.StatusBadRequest, "repo is required")
		return
	}
	orgId := r.PathValue("orgId")
	userID, ok := r.Context().Value(appcontext.UserIDKey).(string)
	if !ok {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	org, err := h.OrganizationRepo.GetByID(r.Context(), orgId)
	if errors.Is(err, repository.ErrNotFound) {
		response.WriteJSONError(w, http.StatusNotFound, "organization not found")
		return
	}
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if org.OwnerID != userID {
		response.WriteJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	err = h.RepositoryRepo.Create(r.Context(), &models.Repository{
		ID:        uuid.NewString(),
		Name:      req.Name,
		Repo:      req.Repo,
		OrgID:     orgId,
		CreatedAt: time.Now(),
	})
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *RepositoryHandler) GetRepositories(w http.ResponseWriter, r *http.Request) {
	orgId := r.PathValue("orgId")
	result, err := h.RepositoryRepo.GetByOrgID(r.Context(), orgId)
	if errors.Is(err, repository.ErrNotFound) {
		response.WriteJSONError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusOK, result)
}

func (h *RepositoryHandler) DeleteRepository(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(appcontext.UserIDKey).(string)
	id := r.PathValue("id")
	orgId := r.PathValue("orgId")

	if !ok {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	repo, err := h.RepositoryRepo.GetByID(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		response.WriteJSONError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	org, err := h.OrganizationRepo.GetByID(r.Context(), orgId)
	if errors.Is(err, repository.ErrNotFound) {
		response.WriteJSONError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if org.OwnerID != userID {
		response.WriteJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err = h.RepositoryRepo.Delete(r.Context(), repo.ID); err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
