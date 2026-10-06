package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lucaspose/goci/internal/models"
)

type OrganizationRepository interface {
	Create(ctx context.Context, org *models.Organization) error
	GetByUserID(ctx context.Context, userID string) ([]models.Organization, error)
	GetByID(ctx context.Context, id string) (models.Organization, error)
	Delete(ctx context.Context, id string) error
}

type sqlOrganizationRepository struct {
	db *sql.DB
}

func NewOrganizationRepository(db *sql.DB) *sqlOrganizationRepository {
	return &sqlOrganizationRepository{
		db: db,
	}
}

func (r *sqlOrganizationRepository) Create(ctx context.Context, orgs *models.Organization) error {
	query := `
	INSERT INTO organizations (id , name, owner_id, created_at)
	VALUES ($1, $2, $3, $4)
	`

	_, err := r.db.ExecContext(ctx, query, orgs.ID, orgs.Name, orgs.OwnerID, orgs.CreatedAt)
	if err != nil {
		return fmt.Errorf("create organization: %w", err)
	}
	return nil
}

func (r *sqlOrganizationRepository) GetByUserID(ctx context.Context, userID string) ([]models.Organization, error) {
	var organizations []models.Organization
	query := `
	SELECT id, name, owner_id, created_at
	FROM organizations
	WHERE owner_id = $1
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get organizations by owner id: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var organization models.Organization
		err := rows.Scan(
			&organization.ID,
			&organization.Name,
			&organization.OwnerID,
			&organization.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("get organizations by owner id: %w", err)
		}
		organizations = append(organizations, organization)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list organizations by owner: %w", err)
	}
	return organizations, nil
}

func (r *sqlOrganizationRepository) GetByID(ctx context.Context, id string) (models.Organization, error) {
	var organization models.Organization
	query := `
	SELECT id, name, owner_id, created_at
	FROM organizations
	WHERE id = $1
	`
	result := r.db.QueryRowContext(ctx, query, id)
	err := result.Scan(
		&organization.ID,
		&organization.Name,
		&organization.OwnerID,
		&organization.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return models.Organization{}, ErrNotFound
	}
	if err != nil {
		return models.Organization{}, fmt.Errorf("get organization by id: %w", err)
	}
	return organization, nil
}

func (r *sqlOrganizationRepository) Delete(ctx context.Context, id string) error {
	query := `
	DELETE FROM organizations
	WHERE id = $1
	`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete organization by id: %w", err)
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
