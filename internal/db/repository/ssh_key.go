package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lucaspose/goci/internal/models"
)

type SSHKeyRepository interface {
	Create(ctx context.Context, key *models.SSHKey) error
	GetByUserID(ctx context.Context, userID string) ([]models.SSHKey, error)
	GetByID(ctx context.Context, id string) (models.SSHKey, error)
	Delete(ctx context.Context, id string) error
}

type sqlSSHKeyRepository struct {
	db *sql.DB
}

func NewSSHKeyRepository(db *sql.DB) SSHKeyRepository {
	return &sqlSSHKeyRepository{db: db}
}

func (s *sqlSSHKeyRepository) Create(ctx context.Context, key *models.SSHKey) error {
	query := `
	INSERT INTO ssh_keys (id, user_id, name, private_key, created_at)
	VALUES ($1, $2, $3, $4, $5)
	`
	_, err := s.db.ExecContext(ctx, query, key.ID, key.UserID, key.Name, key.PrivateKey, key.CreatedAt)
	if err != nil {
		return fmt.Errorf("create ssh key: %w", err)
	}
	return nil
}

func (s *sqlSSHKeyRepository) GetByUserID(ctx context.Context, userID string) ([]models.SSHKey, error) {
	var sshKeys []models.SSHKey
	query := `
	SELECT id, user_id, name, private_key, created_at
	FROM ssh_keys
	WHERE user_id = $1
	`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get ssh key by owner: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key models.SSHKey
		err := rows.Scan(
			&key.ID,
			&key.UserID,
			&key.Name,
			&key.PrivateKey,
			&key.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("get ssh keys by owner: %w", err)
		}
		sshKeys = append(sshKeys, key)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list ssh keys by owner: %w", err)
	}
	return sshKeys, nil
}

func (s *sqlSSHKeyRepository) GetByID(ctx context.Context, id string) (models.SSHKey, error) {
	query := `
	SELECT id, user_id, name, private_key, created_at
	FROM ssh_keys
	WHERE id = $1
	`
	var sshKey models.SSHKey
	result := s.db.QueryRowContext(ctx, query, id)
	err := result.Scan(
		&sshKey.ID,
		&sshKey.UserID,
		&sshKey.Name,
		&sshKey.PrivateKey,
		&sshKey.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return models.SSHKey{}, ErrNotFound
	}
	if err != nil {
		return models.SSHKey{}, fmt.Errorf("get ssh key by id: %w", err)
	}
	return sshKey, nil
}

func (s *sqlSSHKeyRepository) Delete(ctx context.Context, id string) error {
	query := `
	DELETE FROM ssh_keys
	WHERE id = $1
	`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete ssh keys by id: %w", err)
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