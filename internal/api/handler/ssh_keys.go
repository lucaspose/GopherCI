package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/lucaspose/goci/internal/api/context"
	"github.com/lucaspose/goci/internal/api/response"
	"github.com/lucaspose/goci/internal/crypto"
	"github.com/lucaspose/goci/internal/db/repository"
	"github.com/lucaspose/goci/internal/models"
)

type SSHKeyHandler struct {
	SshKeyRepo    repository.SSHKeyRepository
	EncryptionKey string
}

type CreateSSHKeyRequest struct {
	Name       string `json:"name"`
	PrivateKey string `json:"private_key"`
}

func (c *CreateSSHKeyRequest) Validate() error {
	if c.Name == "" {
		return errors.New("no ssh key name")
	}
	if c.PrivateKey == "" {
		return errors.New("no ssh key given")
	}
	return nil
}

func NewSSHKeyHandler(sshKeyRepo repository.SSHKeyRepository, key string) *SSHKeyHandler {
	return &SSHKeyHandler{
		SshKeyRepo:    sshKeyRepo,
		EncryptionKey: key,
	}
}

func (h *SSHKeyHandler) CreateSSHKey(w http.ResponseWriter, r *http.Request) {
	var req CreateSSHKeyRequest
	defer r.Body.Close()
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		response.WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := req.Validate(); err != nil {
		response.WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	userID, ok := r.Context().Value(appcontext.UserIDKey).(string)
	if !ok {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	cryptedSSHKey, err := crypto.Encrypt(req.PrivateKey, []byte(h.EncryptionKey))
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	sshKey := models.SSHKey{
		ID:         uuid.NewString(),
		UserID:     userID,
		Name:       req.Name,
		PrivateKey: cryptedSSHKey,
		CreatedAt:  time.Now(),
	}
	if err = h.SshKeyRepo.Create(r.Context(), &sshKey); err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *SSHKeyHandler) GetSSHKeys(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(appcontext.UserIDKey).(string)
	if !ok {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	sshKey, err := h.SshKeyRepo.GetByUserID(r.Context(), userID)
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusOK, sshKey)
}

func (h *SSHKeyHandler) DeleteSSHKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	userID, ok := ctx.Value(appcontext.UserIDKey).(string)
	if !ok {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	sshKey, err := h.SshKeyRepo.GetByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		response.WriteJSONError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if sshKey.UserID != userID {
		response.WriteJSONError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := h.SshKeyRepo.Delete(ctx, sshKey.ID); err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
