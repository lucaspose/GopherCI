package models

import "time"

type Role string

const (
	RoleUser Role = "user"
	RoleAdmin Role = "admin"
)

type User struct {
	ID string `json:"id"`
	Email string `json:"email"`
	PasswordHash string `json:"-"`
	Role Role `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}