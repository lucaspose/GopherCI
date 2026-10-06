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

type OrganizationHandler struct {
	OrganizationRepo repository.OrganizationRepository
}

type CreateOrganizationRequest struct {
	Name string `json:"name"`
}

func NewOrganizationHandler(organizationRepo repository.OrganizationRepository) *OrganizationHandler {
	return &OrganizationHandler{
		OrganizationRepo: organizationRepo,
	}
}

func (h *OrganizationHandler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	var req CreateOrganizationRequest
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
	userID, ok := r.Context().Value(appcontext.UserIDKey).(string)
	if !ok {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	organization := models.Organization{
		ID:        uuid.NewString(),
		Name:      req.Name,
		OwnerID:   userID,
		CreatedAt: time.Now(),
	}
	if err = h.OrganizationRepo.Create(r.Context(), &organization); err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *OrganizationHandler) GetOrganizations(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(appcontext.UserIDKey).(string)
	if !ok {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	organizations, err := h.OrganizationRepo.GetByUserID(r.Context(), userID)
	if errors.Is(err, repository.ErrNotFound) {
		response.WriteJSONError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusOK, organizations)
}

func (h *OrganizationHandler) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(appcontext.UserIDKey).(string)
	id := r.PathValue("id")

	if !ok {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	organization, err := h.OrganizationRepo.GetByID(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		response.WriteJSONError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if organization.OwnerID != userID {
		response.WriteJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	err = h.OrganizationRepo.Delete(r.Context(), organization.ID)
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
