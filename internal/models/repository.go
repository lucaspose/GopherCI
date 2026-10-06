package models

import "time"

type Repository struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Repo      string    `json:"repo"`
	OrgID     string    `json:"org_id"`
	CreatedAt time.Time `json:"created_at"`
}
