package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lucaspose/goci/internal/models"
)

var ErrNotFound = errors.New("not found")


type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Delete(ctx context.Context, id string) error
}

type sqlUserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
	return &sqlUserRepository{db: db}
}

type ProjectRepository interface {
	Create(ctx context.Context, project *models.Project) error
	GetByID(ctx context.Context, id string) (*models.Project, error)
	ListByOwner(ctx context.Context, ownerID string) ([]*models.Project, error)
	Delete(ctx context.Context, id string) error
}

type sqlProjectRepository struct {
	db *sql.DB
}

func NewProjectRepository(db *sql.DB) ProjectRepository {
	return &sqlProjectRepository{db: db}
}

func (r *sqlUserRepository) Create(ctx context.Context, user *models.User) error {
	query := `
	INSERT INTO users (id, email, password_hash, role, created_at)
	VALUES ($1, $2, $3, $4, $5)
	`
	if _, err := r.db.ExecContext(ctx, query, user.ID, user.Email, user.PasswordHash, user.Role, user.CreatedAt); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *sqlUserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	var user models.User
	query := `
	SELECT id, email, password_hash, role, created_at
	FROM users
	WHERE id = $1
	`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &user, nil
}

func (r *sqlUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	query := `
	SELECT id, email, password_hash, role, created_at
	FROM users
	WHERE email = $1
	`
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &user, nil
}

func (r *sqlUserRepository) Delete(ctx context.Context, id string) error {
	query := `
	DELETE FROM users WHERE id = $1
	`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete user by id: %w", err)
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

func (r *sqlProjectRepository) Create(ctx context.Context, project *models.Project) error {
	query := `
	INSERT INTO projects (id, owner_id, name, repo_url, webhook_secret, created_at)
	VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, query, project.ID, project.OwnerID, project.Name, project.RepoURL, project.WebhookSecret, project.CreatedAt)
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

func (r *sqlProjectRepository) GetByID(ctx context.Context, id string) (*models.Project, error) {
	var project models.Project
	query := `
	SELECT id, owner_id, name, repo_url, webhook_secret, created_at
	FROM projects
	WHERE id = $1
	`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&project.ID,
		&project.OwnerID,
		&project.Name,
		&project.RepoURL,
		&project.WebhookSecret,
		&project.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get project by id: %w", err)
	}
	return &project, nil
}

func (r *sqlProjectRepository) ListByOwner(ctx context.Context, ownerID string) ([]*models.Project, error) {
	var projects []*models.Project
	query := `
	SELECT id, owner_id, name, repo_url, webhook_secret, created_at
	FROM projects
	WHERE owner_id = $1
	`
	rows, err := r.db.QueryContext(ctx, query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("get project by owner_id: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var project models.Project
		err := rows.Scan(
			&project.ID,
			&project.OwnerID,
			&project.Name,
			&project.RepoURL,
			&project.WebhookSecret,
			&project.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("get project by owner_id: %w", err)
		}
		projects = append(projects, &project)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list projects by owner: %w", err)
	}
	return projects, nil
}

func (r *sqlProjectRepository) Delete(ctx context.Context, id string) error {
	query := `
	DELETE FROM projects WHERE id = $1
	`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete project by id: %w", err)
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

