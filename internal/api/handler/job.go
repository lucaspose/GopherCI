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
	"github.com/lucaspose/goci/internal/worker"
)

type JobHandler struct {
	JobWorker *worker.Worker
	JobRepo   repository.JobsRepository
}

type CreateJobRequest struct {
	Repo  string        `json:"repo"`
	Steps []models.Step `json:"steps"`
}

func (c *CreateJobRequest) Validate() error {
	if c.Repo == "" {
		return errors.New("no repo is given")
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
	job := models.Job{
		ID:        uuid.NewString(),
		Repo:      req.Repo,
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

	job, err := j.JobRepo.GetByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
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

func (j *JobHandler) DeleteJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	err := j.JobRepo.Delete(ctx, id)
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
