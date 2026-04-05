package models

import "time"

type PipelineStatus string
type JobStatus string
type LogStream string

const (
	PipelinePending PipelineStatus = "pending"
	PipelineRunning PipelineStatus = "running"
	PipelineSuccess PipelineStatus = "success"
	PipelineFailed  PipelineStatus = "failed"
	PipelineCancelled PipelineStatus = "cancelled"
)

const (
	JobPending JobStatus = "pending"
	JobRunning JobStatus = "running"
	JobSuccess JobStatus = "success"
	JobFailed  JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

const (
	LogStdout LogStream = "stdout"
	LogStderr LogStream = "stderr"
)

type Pipeline struct {
	ID string `json:"id"`
	ProjectID string `json:"project_id"`
	CommitSHA string `json:"commit_sha"`
	Branch string `json:"branch"`
	Status PipelineStatus `json:"status"`
	StartedAt *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	CreatedAt time.Time `json:"created_at"`
}

type Job struct {
	ID string `json:"id"`
	PipelineID string `json:"pipeline_id"`
	StepName string `json:"step_name"`
	Status JobStatus `json:"status"`
	ExitCode *int `json:"exit_code"`
	DurationMs *int64 `json:"duration_ms"`
	WorkerID *string `json:"worker_id"`
	CreatedAt time.Time `json:"created_at"`
}
type JobLog struct {
	ID string `json:"id"`
	JobID string `json:"job_id"`
	Line string `json:"line"`
	Stream LogStream `json:"stream"`
	CreatedAt time.Time `json:"created_at"`
}