package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	appcontext "github.com/lucaspose/goci/internal/api/context"
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
