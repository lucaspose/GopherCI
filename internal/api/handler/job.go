package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/lucaspose/goci/internal/api/context"
	"github.com/lucaspose/goci/internal/api/response"
	"github.com/lucaspose/goci/internal/db/repository"
	"github.com/lucaspose/goci/internal/models"
	"github.com/lucaspose/goci/internal/worker"
)

type JobHandler struct {
	JobWorker *worker.Worker
	JobRepo   repository.JobsRepository
	Repos     repository.RepositoryRepository
	Orgs      repository.OrganizationRepository
	SSHKeys   repository.SSHKeyRepository
}

type CreateJobRequest struct {
	CloneURL string        `json:"clone_url"`
	SSHKeyID string        `json:"ssh_key_id"`
	Steps    []models.Step `json:"steps"`
}

const (
	jobsStreamPollInterval      = time.Second
	jobsStreamKeepAliveInterval = 15 * time.Second
	defaultArtifactsDir         = "artifacts"
)

func (c *CreateJobRequest) Validate() error {
	if c.CloneURL == "" {
		return errors.New("no clone_url given")
	}
	if len(c.Steps) == 0 {
		return errors.New("no steps")
	}
	for _, step := range c.Steps {
		if step.Name == "" {
			return errors.New("step name is empty")
		}
		if len(step.Cmd) == 0 {
			return errors.New("no command given")
		}
	}
	return nil
}

func NewJobHandler(JobWorker *worker.Worker, jobRepo repository.JobsRepository) *JobHandler {
	return &JobHandler{
		JobWorker: JobWorker,
		JobRepo:   jobRepo,
	}
}

// WithSSHKeys lets the handler reject jobs that reference another user's SSH key.
func (j *JobHandler) WithSSHKeys(keys repository.SSHKeyRepository) *JobHandler {
	j.SSHKeys = keys
	return j
}

// ownsSSHKey reports whether the SSH key (if any) belongs to the user.
func (j *JobHandler) ownsSSHKey(r *http.Request, userID, keyID string) (bool, error) {
	if keyID == "" || j.SSHKeys == nil {
		return true, nil
	}
	key, err := j.SSHKeys.GetByID(r.Context(), keyID)
	if errors.Is(err, repository.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return key.UserID == userID, nil
}

// WithRepositories enables the repository-scoped job routes.
func (j *JobHandler) WithRepositories(repos repository.RepositoryRepository, orgs repository.OrganizationRepository) *JobHandler {
	j.Repos = repos
	j.Orgs = orgs
	return j
}

func (j *JobHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobRequest
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
	if owned, err := j.ownsSSHKey(r, userID, req.SSHKeyID); err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	} else if !owned {
		response.WriteJSONError(w, http.StatusBadRequest, "ssh key not found")
		return
	}
	job := models.Job{
		ID:        uuid.NewString(),
		CloneURL:  req.CloneURL,
		SSHKeyID:  req.SSHKeyID,
		Steps:     req.Steps,
		UserID:    userID,
		Status:    models.JobPending,
		CreatedAt: time.Now(),
	}
	j.JobWorker.JobQueue <- job
	response.WriteJSON(w, http.StatusAccepted, "job created")
}

func (j *JobHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	userID, _ := ctx.Value(appcontext.UserIDKey).(string)

	job, err := j.JobRepo.GetByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && job.UserID != userID) {
		response.WriteJSONError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusOK, job)
}

func (j *JobHandler) GetJobs(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(appcontext.UserIDKey).(string)
	if !ok {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	jobs, err := j.JobRepo.Get(r.Context(), userID)
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusOK, jobs)
}

func (j *JobHandler) StreamJobs(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(appcontext.UserIDKey).(string)
	if !ok {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		response.WriteJSONError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	filters := parseJobStreamFilters(r)
	jobs, err := j.JobRepo.GetFiltered(r.Context(), userID, filters)
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	payload, err := json.Marshal(jobs)
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if err := writeSSEEvent(w, flusher, "jobs", payload); err != nil {
		return
	}
	lastPayload := payload

	pollTicker := time.NewTicker(jobsStreamPollInterval)
	defer pollTicker.Stop()
	keepAliveTicker := time.NewTicker(jobsStreamKeepAliveInterval)
	defer keepAliveTicker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-pollTicker.C:
			jobs, err := j.JobRepo.GetFiltered(r.Context(), userID, filters)
			if err != nil {
				if err := writeSSEEvent(w, flusher, "error", []byte(`{"error":"internal server error"}`)); err != nil {
					return
				}
				continue
			}
			payload, err := json.Marshal(jobs)
			if err != nil {
				if err := writeSSEEvent(w, flusher, "error", []byte(`{"error":"internal server error"}`)); err != nil {
					return
				}
				continue
			}
			if bytes.Equal(payload, lastPayload) {
				continue
			}
			if err := writeSSEEvent(w, flusher, "jobs", payload); err != nil {
				return
			}
			lastPayload = payload
		case <-keepAliveTicker.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func parseJobStreamFilters(r *http.Request) repository.JobFilters {
	query := r.URL.Query()
	cloneURLs := sanitizeCloneURLs(query["clone_url"])
	return repository.JobFilters{
		OrgID:     strings.TrimSpace(query.Get("org_id")),
		RepoID:    strings.TrimSpace(query.Get("repo_id")),
		CloneURLs: cloneURLs,
	}
}

func sanitizeCloneURLs(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	cloneURLs := make([]string, 0, len(raw))
	for _, cloneURL := range raw {
		cloneURL = strings.TrimSpace(cloneURL)
		if cloneURL == "" {
			continue
		}
		if _, ok := seen[cloneURL]; ok {
			continue
		}
		seen[cloneURL] = struct{}{}
		cloneURLs = append(cloneURLs, cloneURL)
	}
	return cloneURLs
}

func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, event string, payload []byte) error {
	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func (j *JobHandler) DownloadJobArtifact(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(appcontext.UserIDKey).(string)
	if !ok {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	jobID := r.PathValue("id")
	job, err := j.JobRepo.GetByID(r.Context(), jobID)
	if errors.Is(err, repository.ErrNotFound) {
		response.WriteJSONError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if job.UserID != userID {
		response.WriteJSONError(w, http.StatusForbidden, "forbidden")
		return
	}

	artifactPath, err := j.artifactPath(jobID)
	if err != nil {
		response.WriteJSONError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	fileInfo, err := os.Stat(artifactPath)
	if errors.Is(err, os.ErrNotExist) {
		response.WriteJSONError(w, http.StatusNotFound, "artifact not found")
		return
	}
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if fileInfo.IsDir() {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	fileName := fmt.Sprintf("%s-build.zip", jobID)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	http.ServeFile(w, r, artifactPath)
}

func (j *JobHandler) artifactPath(jobID string) (string, error) {
	if strings.TrimSpace(jobID) == "" {
		return "", errors.New("empty job id")
	}
	if strings.Contains(jobID, "/") || strings.Contains(jobID, "\\") || strings.Contains(jobID, "..") {
		return "", errors.New("invalid job id")
	}

	artifactsDir := defaultArtifactsDir
	if j.JobWorker != nil && strings.TrimSpace(j.JobWorker.ArtifactsDir) != "" {
		artifactsDir = strings.TrimSpace(j.JobWorker.ArtifactsDir)
	}

	absArtifactsDir, err := filepath.Abs(artifactsDir)
	if err != nil {
		return "", fmt.Errorf("resolve artifacts dir: %w", err)
	}

	artifactPath := filepath.Clean(filepath.Join(absArtifactsDir, fmt.Sprintf("%s-build.zip", jobID)))
	relPath, err := filepath.Rel(absArtifactsDir, artifactPath)
	if err != nil {
		return "", fmt.Errorf("resolve artifact path: %w", err)
	}
	if strings.HasPrefix(relPath, "..") {
		return "", errors.New("artifact path escape")
	}

	return artifactPath, nil
}

func (j *JobHandler) DeleteJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	userID, _ := ctx.Value(appcontext.UserIDKey).(string)

	job, err := j.JobRepo.GetByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && job.UserID != userID) {
		response.WriteJSONError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	err = j.JobRepo.Delete(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		response.WriteJSONError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type createRepoJobRequest struct {
	SSHKeyID string        `json:"ssh_key_id"`
	Steps    []models.Step `json:"steps"`
}

// ownedRepository returns the repository {repoId} of organization {orgId}
// if that organization belongs to the current user.
func (j *JobHandler) ownedRepository(r *http.Request, userID string) (models.Repository, bool, error) {
	orgID := r.PathValue("orgId")
	repoID := r.PathValue("repoId")
	org, err := j.Orgs.GetByID(r.Context(), orgID)
	if errors.Is(err, repository.ErrNotFound) {
		return models.Repository{}, false, nil
	}
	if err != nil {
		return models.Repository{}, false, err
	}
	if org.OwnerID != userID {
		return models.Repository{}, false, nil
	}
	repo, err := j.Repos.GetByID(r.Context(), repoID)
	if errors.Is(err, repository.ErrNotFound) {
		return models.Repository{}, false, nil
	}
	if err != nil {
		return models.Repository{}, false, err
	}
	return repo, repo.OrgID == orgID, nil
}

// CreateRepoJob runs a pipeline on a repository registered in an organization.
func (j *JobHandler) CreateRepoJob(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(appcontext.UserIDKey).(string)
	if !ok {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	var body createRepoJobRequest
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	repo, found, err := j.ownedRepository(r, userID)
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if !found {
		response.WriteJSONError(w, http.StatusNotFound, "repository not found")
		return
	}
	req := CreateJobRequest{CloneURL: repo.Repo, SSHKeyID: body.SSHKeyID, Steps: body.Steps}
	if err := req.Validate(); err != nil {
		response.WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if owned, err := j.ownsSSHKey(r, userID, req.SSHKeyID); err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	} else if !owned {
		response.WriteJSONError(w, http.StatusBadRequest, "ssh key not found")
		return
	}
	j.JobWorker.JobQueue <- models.Job{
		ID:        uuid.NewString(),
		CloneURL:  req.CloneURL,
		SSHKeyID:  req.SSHKeyID,
		Steps:     req.Steps,
		UserID:    userID,
		Status:    models.JobPending,
		CreatedAt: time.Now(),
	}
	response.WriteJSON(w, http.StatusAccepted, "job created")
}

// GetRepoJobs lists the current user's jobs for one repository of an organization.
func (j *JobHandler) GetRepoJobs(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(appcontext.UserIDKey).(string)
	if !ok {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	jobs, err := j.JobRepo.GetFiltered(r.Context(), userID, repository.JobFilters{
		OrgID:  r.PathValue("orgId"),
		RepoID: r.PathValue("repoId"),
	})
	if err != nil {
		response.WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	response.WriteJSON(w, http.StatusOK, jobs)
}
