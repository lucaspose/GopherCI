package models

import "time"

type SSHKey struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Name       string    `json:"name"`
	PrivateKey string    `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
}
