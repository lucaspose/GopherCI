package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/lib/pq"
	"github.com/lucaspose/goci/internal/models"
)

type JobsRepository interface {
	Create(ctx context.Context, jobs *models.Job) error
	GetByID(ctx context.Context, id string) (*models.Job, error)
	UpdateStatus(ctx context.Context, id string, status models.JobStatus) error
	Get(ctx context.Context, userID string) ([]models.Job, error)
	Delete(ctx context.Context, id string) error
	UpdateLogs(ctx context.Context, id string, logs []string) error
}

type sqlJobRepository struct {
	db *sql.DB
}

func NewJobRepository(db *sql.DB) JobsRepository {
	return &sqlJobRepository{db: db}
}

func (r *sqlJobRepository) Create(ctx context.Context, job *models.Job) error {
	query := `
	INSERT INTO jobs (id, repo, steps, user_id, status, created_at, logs)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	stepsJSON, err := json.Marshal(job.Steps)
	if err != nil {
		return fmt.Errorf("step to json: %w", err)
	}
	_, err = r.db.ExecContext(ctx, query, job.ID, job.Repo, stepsJSON, job.UserID, job.Status, job.CreatedAt, pq.Array(job.Logs))
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}

func (r *sqlJobRepository) GetByID(ctx context.Context, id string) (*models.Job, error) {
	query := `
	SELECT id, repo, steps, user_id, status, created_at, logs
	FROM jobs
	WHERE id = $1
	`
	var stepsJSON []byte
	var job models.Job
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&job.ID,
		&job.Repo,
		&stepsJSON,
		&job.UserID,
		&job.Status,
		&job.CreatedAt,
		pq.Array(&job.Logs),
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}
	err = json.Unmarshal(stepsJSON, &job.Steps)
	if err != nil {
		return nil, fmt.Errorf("get job steps: %w", err)
	}
	return &job, nil
}

func (r *sqlJobRepository) Get(ctx context.Context, userID string) ([]models.Job, error) {
	var jobs []models.Job
	query := `
	SELECT id, repo, steps, user_id, status, created_at, logs
	FROM jobs
	WHERE user_id = $1
	`
	result, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get jobs: %w", err)
	}
	for result.Next() {
		var job models.Job
		var stepsJSON []byte
		err := result.Scan(
			&job.ID,
			&job.Repo,
			&stepsJSON,
			&job.UserID,
			&job.Status,
			&job.CreatedAt,
			pq.Array(&job.Logs),
		)
		if err != nil {
			return nil, fmt.Errorf("get job: %w", err)
		}
		err = json.Unmarshal(stepsJSON, &job.Steps)
		if err != nil {
			return nil, fmt.Errorf("get job steps: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("get jobs: %w", err)
	}
	return jobs, nil
}

func (r *sqlJobRepository) UpdateStatus(ctx context.Context, id string, status models.JobStatus) error {
	query := `
	UPDATE jobs
	SET status = $1
	WHERE id = $2
	`
	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sqlJobRepository) Delete(ctx context.Context, id string) error {
	query := `
	DELETE FROM jobs
	WHERE id = $1
	`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete job by id: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *sqlJobRepository) UpdateLogs(ctx context.Context, id string, logs []string) error {
	query := `
	UPDATE jobs
	SET logs = $1
	WHERE id = $2
	`
	result, err := r.db.ExecContext(ctx, query, pq.Array(logs), id)
	if err != nil {
		return fmt.Errorf("update logs: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update logs: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
