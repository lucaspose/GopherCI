package models

import (
	"time"
)

type JobStatus string

const (
	JobPending JobStatus = "pending"
	JobRunning JobStatus = "running"
	JobSuccess JobStatus = "success"
	JobFailed  JobStatus = "failed"
)

type Job struct {
	ID        string    `json:"id"`
    CloneURL  string    `json:"clone_url"`
	Steps     []Step    `json:"steps"`
	UserID    string    `json:"user_id"`
	Status    JobStatus `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	Logs      []string  `json:"logs"`
	SSHKeyID  string    `json:"ssh_key_id"`
	RepoID    string    `json:"repo_id"`
}
