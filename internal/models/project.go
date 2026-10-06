package models

import "time"

type Project struct {
	ID            string    `json:"id"`
	OwnerID       string    `json:"owner_id"`
	Name          string    `json:"name"`
	RepoURL       string    `json:"repo_url"`
	WebhookSecret string    `json:"-"`
	CreatedAt     time.Time `json:"created_at"`
}
