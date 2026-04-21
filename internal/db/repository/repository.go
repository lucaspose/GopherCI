package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lucaspose/goci/internal/models"
)

type RepositoryRepository interface {
	Create(ctx context.Context, repo *models.Repository) error
	GetByOrgID(ctx context.Context, orgID string) ([]models.Repository, error)
	GetByID(ctx context.Context, id string) (models.Repository, error)
	Delete(ctx context.Context, id string) error
}

type sqlRepositoryRepository struct {
	db *sql.DB
}

func NewRepositoryRepository(db *sql.DB) *sqlRepositoryRepository {
	return &sqlRepositoryRepository{
		db: db,
	}
}

func (r *sqlRepositoryRepository) Create(ctx context.Context, repo *models.Repository) error {
	query := `
	INSERT INTO repositories (id, name, repo, org_id, created_at)
	VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.ExecContext(ctx, query, repo.ID, repo.Name, repo.Repo, repo.OrgID, repo.CreatedAt)
	if err != nil {
		return fmt.Errorf("create repository: %w", err)
	}
	return nil
}

func (r *sqlRepositoryRepository) GetByOrgID(ctx context.Context, orgID string) ([]models.Repository, error) {
	var repositories []models.Repository
	query := `
	SELECT id, name, repo, org_id, created_at
	FROM repositories
	WHERE org_id = $1
	`
	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("get repositories by organization id: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var repository models.Repository
		err := rows.Scan(
			&repository.ID,
			&repository.Name,
			&repository.Repo,
			&repository.OrgID,
			&repository.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("get repositories by organization id: %w", err)
		}
		repositories = append(repositories, repository)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list repository by organization id: %w", err)
	}
	return repositories, nil
}

func (r *sqlRepositoryRepository) GetByID(ctx context.Context, id string) (models.Repository, error) {
	var repository models.Repository
	query := `
	SELECT id, name, repo, org_id, created_at
	FROM repositories
	WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)
	err := row.Scan(
		&repository.ID,
		&repository.Name,
		&repository.Repo,
		&repository.OrgID,
		&repository.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return models.Repository{}, ErrNotFound
	}
	if err != nil {
		return models.Repository{}, fmt.Errorf("get repository by id: %w", err)
	}
	return repository, nil
}

func (r *sqlRepositoryRepository) Delete(ctx context.Context, id string) error {
	query := `
	DELETE FROM repositories
	WHERE id = $1
	`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete repository by id: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete repository by id: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}