package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	appcontext "github.com/lucaspose/goci/internal/api/context"
	"github.com/lucaspose/goci/internal/db/repository"
	"github.com/lucaspose/goci/internal/models"
	"github.com/lucaspose/goci/internal/worker"
)

type mockJobRepo struct {
	mu           sync.Mutex
	jobs         []models.Job
	getErr       error
	getFilterErr error
	lastUserID   string
	lastFilters  repository.JobFilters
}

func (m *mockJobRepo) Create(ctx context.Context, jobs *models.Job) error {
	return nil
}

func (m *mockJobRepo) GetByID(ctx context.Context, id string) (*models.Job, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, job := range m.jobs {
		if job.ID == id {
			jobCopy := job
			return &jobCopy, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (m *mockJobRepo) UpdateStatus(ctx context.Context, id string, status models.JobStatus) error {
	return nil
}

func (m *mockJobRepo) Get(ctx context.Context, userID string) ([]models.Job, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.jobs, nil
}

func (m *mockJobRepo) GetFiltered(ctx context.Context, userID string, filters repository.JobFilters) ([]models.Job, error) {
	m.mu.Lock()
	m.lastUserID = userID
	m.lastFilters = filters
	m.mu.Unlock()
	if m.getFilterErr != nil {
		return nil, m.getFilterErr
	}
	return m.jobs, nil
}

func (m *mockJobRepo) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockJobRepo) UpdateLogs(ctx context.Context, id string, logs []string) error {
	return nil
}

func (m *mockJobRepo) snapshot() (string, repository.JobFilters) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastUserID, m.lastFilters
}

func TestParseJobStreamFilters(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/jobs/stream?org_id=org-1&repo_id=repo-1&clone_url=https://github.com/acme/repo.git&clone_url=https://github.com/acme/repo.git&clone_url=%20", nil)

	filters := parseJobStreamFilters(req)

	if filters.OrgID != "org-1" {
		t.Fatalf("expected org_id filter, got %q", filters.OrgID)
	}
	if filters.RepoID != "repo-1" {
		t.Fatalf("expected repo_id filter, got %q", filters.RepoID)
	}
	if len(filters.CloneURLs) != 1 {
		t.Fatalf("expected 1 clone_url after sanitizing, got %d", len(filters.CloneURLs))
	}
	if filters.CloneURLs[0] != "https://github.com/acme/repo.git" {
		t.Fatalf("expected clone_url value to be preserved, got %q", filters.CloneURLs[0])
	}
}

func TestStreamJobs(t *testing.T) {
	t.Run("writes initial sse payload and applies query filters", func(t *testing.T) {
		repo := &mockJobRepo{
			jobs: []models.Job{{
				ID:        "job-1",
				CloneURL:  "https://github.com/acme/repo.git",
				UserID:    "user-1",
				Status:    models.JobPending,
				CreatedAt: time.Unix(1, 0),
			}},
		}
		h := NewJobHandler(nil, repo)

		baseCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req := httptest.NewRequest(http.MethodGet, "/jobs/stream?org_id=org-1&repo_id=repo-1&clone_url=https://github.com/acme/repo.git", nil)
		req = req.WithContext(context.WithValue(baseCtx, appcontext.UserIDKey, "user-1"))
		rr := httptest.NewRecorder()

		done := make(chan struct{})
		go func() {
			h.StreamJobs(rr, req)
			close(done)
		}()

		time.AfterFunc(30*time.Millisecond, cancel)

		select {
		case <-done:
		case <-time.After(250 * time.Millisecond):
			t.Fatal("stream handler did not exit after request cancellation")
		}

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}
		if got := rr.Header().Get("Content-Type"); got != "text/event-stream" {
			t.Fatalf("expected text/event-stream content type, got %q", got)
		}
		body := rr.Body.String()
		if !strings.Contains(body, "event: jobs") {
			t.Fatalf("expected jobs SSE event, got body %q", body)
		}
		if !strings.Contains(body, `"id":"job-1"`) {
			t.Fatalf("expected job payload in stream body, got %q", body)
		}

		userID, filters := repo.snapshot()
		if userID != "user-1" {
			t.Fatalf("expected user id user-1 passed to repository, got %q", userID)
		}
		if filters.OrgID != "org-1" {
			t.Fatalf("expected org_id filter org-1, got %q", filters.OrgID)
		}
		if filters.RepoID != "repo-1" {
			t.Fatalf("expected repo_id filter repo-1, got %q", filters.RepoID)
		}
		if len(filters.CloneURLs) != 1 || filters.CloneURLs[0] != "https://github.com/acme/repo.git" {
			t.Fatalf("unexpected clone_url filters: %#v", filters.CloneURLs)
		}
	})

	t.Run("missing user context returns 500", func(t *testing.T) {
		repo := &mockJobRepo{}
		h := NewJobHandler(nil, repo)
		req := httptest.NewRequest(http.MethodGet, "/jobs/stream", nil)
		rr := httptest.NewRecorder()

		h.StreamJobs(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", rr.Code)
		}
	})

	t.Run("repository error before streaming returns 500", func(t *testing.T) {
		repo := &mockJobRepo{getFilterErr: errors.New("db down")}
		h := NewJobHandler(nil, repo)
		req := httptest.NewRequest(http.MethodGet, "/jobs/stream", nil)
		req = req.WithContext(context.WithValue(req.Context(), appcontext.UserIDKey, "user-1"))
		rr := httptest.NewRecorder()

		h.StreamJobs(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", rr.Code)
		}
	})
}

func TestDownloadJobArtifact(t *testing.T) {
	t.Run("downloads artifact when job belongs to user", func(t *testing.T) {
		artifactsDir := t.TempDir()
		artifactPath := filepath.Join(artifactsDir, "job-1-build.zip")
		artifactContent := []byte("zip-content")
		if err := os.WriteFile(artifactPath, artifactContent, 0644); err != nil {
			t.Fatalf("write artifact file: %v", err)
		}

		repo := &mockJobRepo{
			jobs: []models.Job{{ID: "job-1", UserID: "user-1"}},
		}
		h := NewJobHandler(&worker.Worker{ArtifactsDir: artifactsDir}, repo)

		req := httptest.NewRequest(http.MethodGet, "/jobs/job-1/artifact", nil)
		req.SetPathValue("id", "job-1")
		req = req.WithContext(context.WithValue(req.Context(), appcontext.UserIDKey, "user-1"))
		rr := httptest.NewRecorder()

		h.DownloadJobArtifact(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}
		if got := rr.Header().Get("Content-Disposition"); !strings.Contains(got, "job-1-build.zip") {
			t.Fatalf("expected content disposition with artifact filename, got %q", got)
		}
		if rr.Body.String() != string(artifactContent) {
			t.Fatalf("unexpected artifact body: %q", rr.Body.String())
		}
	})

	t.Run("returns 404 when artifact does not exist", func(t *testing.T) {
		repo := &mockJobRepo{
			jobs: []models.Job{{ID: "job-2", UserID: "user-1"}},
		}
		h := NewJobHandler(&worker.Worker{ArtifactsDir: t.TempDir()}, repo)

		req := httptest.NewRequest(http.MethodGet, "/jobs/job-2/artifact", nil)
		req.SetPathValue("id", "job-2")
		req = req.WithContext(context.WithValue(req.Context(), appcontext.UserIDKey, "user-1"))
		rr := httptest.NewRecorder()

		h.DownloadJobArtifact(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rr.Code)
		}
	})

	t.Run("returns 403 when user does not own job", func(t *testing.T) {
		repo := &mockJobRepo{
			jobs: []models.Job{{ID: "job-3", UserID: "user-owner"}},
		}
		h := NewJobHandler(&worker.Worker{ArtifactsDir: t.TempDir()}, repo)

		req := httptest.NewRequest(http.MethodGet, "/jobs/job-3/artifact", nil)
		req.SetPathValue("id", "job-3")
		req = req.WithContext(context.WithValue(req.Context(), appcontext.UserIDKey, "user-1"))
		rr := httptest.NewRecorder()

		h.DownloadJobArtifact(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", rr.Code)
		}
	})

	t.Run("returns 404 when job does not exist", func(t *testing.T) {
		h := NewJobHandler(&worker.Worker{ArtifactsDir: t.TempDir()}, &mockJobRepo{})

		req := httptest.NewRequest(http.MethodGet, "/jobs/job-404/artifact", nil)
		req.SetPathValue("id", "job-404")
		req = req.WithContext(context.WithValue(req.Context(), appcontext.UserIDKey, "user-1"))
		rr := httptest.NewRecorder()

		h.DownloadJobArtifact(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rr.Code)
		}
	})

	t.Run("returns 500 when user context missing", func(t *testing.T) {
		h := NewJobHandler(&worker.Worker{ArtifactsDir: t.TempDir()}, &mockJobRepo{})

		req := httptest.NewRequest(http.MethodGet, "/jobs/job-1/artifact", nil)
		req.SetPathValue("id", "job-1")
		rr := httptest.NewRecorder()

		h.DownloadJobArtifact(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", rr.Code)
		}
	})
}
