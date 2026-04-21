package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lucaspose/goci/internal/models"
)

type RefreshTokenRepository interface {
	Create(ctx context.Context, token *models.RefreshToken) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error)
	RevokeByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) error
	RevokeByUserID(ctx context.Context, userID string, revokedAt time.Time) error
}

type sqlRefreshTokenRepository struct {
	db *sql.DB
}

func NewRefreshTokenRepository(db *sql.DB) RefreshTokenRepository {
	return &sqlRefreshTokenRepository{db: db}
}

func (r *sqlRefreshTokenRepository) Create(ctx context.Context, token *models.RefreshToken) error {
	query := `
	INSERT INTO refresh_tokens (id, user_id, token_hash, created_at, expires_at, revoked_at)
	VALUES ($1, $2, $3, $4, $5, $6)
	`
	if _, err := r.db.ExecContext(ctx, query, token.ID, token.UserID, token.TokenHash, token.CreatedAt, token.ExpiresAt, token.RevokedAt); err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

func (r *sqlRefreshTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	query := `
	SELECT id, user_id, token_hash, created_at, expires_at, revoked_at
	FROM refresh_tokens
	WHERE token_hash = $1
	`
	var token models.RefreshToken
	var revokedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.CreatedAt,
		&token.ExpiresAt,
		&revokedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	if revokedAt.Valid {
		t := revokedAt.Time
		token.RevokedAt = &t
	}
	return &token, nil
}

func (r *sqlRefreshTokenRepository) RevokeByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	query := `
	UPDATE refresh_tokens
	SET revoked_at = $1
	WHERE token_hash = $2
	AND revoked_at IS NULL
	`
	result, err := r.db.ExecContext(ctx, query, revokedAt, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
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

func (r *sqlRefreshTokenRepository) RevokeByUserID(ctx context.Context, userID string, revokedAt time.Time) error {
	query := `
	UPDATE refresh_tokens
	SET revoked_at = $1
	WHERE user_id = $2
	AND revoked_at IS NULL
	`
	if _, err := r.db.ExecContext(ctx, query, revokedAt, userID); err != nil {
		return fmt.Errorf("revoke refresh token by user id: %w", err)
	}
	return nil
}
