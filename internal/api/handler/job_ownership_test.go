package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appcontext "github.com/lucaspose/goci/internal/api/context"
	"github.com/lucaspose/goci/internal/db/repository"
	"github.com/lucaspose/goci/internal/models"
)

func jobRequest(method, jobID, userID string) *http.Request {
	req := httptest.NewRequest(method, "/jobs/"+jobID, nil)
	req.SetPathValue("id", jobID)
	return req.WithContext(context.WithValue(req.Context(), appcontext.UserIDKey, userID))
}

func TestJobsAreOnlyVisibleToTheirOwner(t *testing.T) {
	repo := &mockJobRepo{jobs: []models.Job{{ID: "job-1", UserID: "owner"}}}
	h := NewJobHandler(nil, repo)

	tests := []struct {
		name   string
		call   func(http.ResponseWriter, *http.Request)
		method string
		user   string
		want   int
	}{
		{"owner can read", h.GetJob, http.MethodGet, "owner", http.StatusOK},
		{"other user cannot read", h.GetJob, http.MethodGet, "intruder", http.StatusNotFound},
		{"other user cannot delete", h.DeleteJob, http.MethodDelete, "intruder", http.StatusNotFound},
		{"owner can delete", h.DeleteJob, http.MethodDelete, "owner", http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.call(rec, jobRequest(tt.method, "job-1", tt.user))
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

type stubSSHKeys struct{ keys map[string]models.SSHKey }

func (s stubSSHKeys) Create(context.Context, *models.SSHKey) error { return nil }
func (s stubSSHKeys) GetByUserID(context.Context, string) ([]models.SSHKey, error) {
	return nil, nil
}
func (s stubSSHKeys) Delete(context.Context, string) error { return nil }
func (s stubSSHKeys) GetByID(_ context.Context, id string) (models.SSHKey, error) {
	key, ok := s.keys[id]
	if !ok {
		return models.SSHKey{}, repository.ErrNotFound
	}
	return key, nil
}

func TestCreateJobRejectsSomeoneElsesSSHKey(t *testing.T) {
	keys := stubSSHKeys{keys: map[string]models.SSHKey{"key-1": {ID: "key-1", UserID: "owner"}}}
	h := NewJobHandler(nil, &mockJobRepo{}).WithSSHKeys(keys)

	body := `{"clone_url":"git@github.com:owner/private.git","ssh_key_id":"key-1","steps":[{"name":"x","cmd":["true"]}]}`
	req := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), appcontext.UserIDKey, "intruder"))
	rec := httptest.NewRecorder()

	h.CreateJob(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
