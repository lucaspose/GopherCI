package handler

import (
	"context"
	"sync"
	"time"

	"github.com/lucaspose/goci/internal/db/repository"
	"github.com/lucaspose/goci/internal/models"
)

type mockRefreshTokenRepo struct {
	mu            sync.Mutex
	byTokenHash   map[string]*models.RefreshToken
	createErr     error
	getErr        error
	revokeErr     error
	revokeUserErr error
}

func newMockRefreshTokenRepo() *mockRefreshTokenRepo {
	return &mockRefreshTokenRepo{
		byTokenHash: make(map[string]*models.RefreshToken),
	}
}

func (m *mockRefreshTokenRepo) Create(ctx context.Context, token *models.RefreshToken) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	tokenCopy := *token
	m.byTokenHash[token.TokenHash] = &tokenCopy
	return nil
}

func (m *mockRefreshTokenRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	token, ok := m.byTokenHash[tokenHash]
	if !ok {
		return nil, repository.ErrNotFound
	}
	tokenCopy := *token
	return &tokenCopy, nil
}

func (m *mockRefreshTokenRepo) RevokeByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	if m.revokeErr != nil {
		return m.revokeErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	token, ok := m.byTokenHash[tokenHash]
	if !ok || token.RevokedAt != nil {
		return repository.ErrNotFound
	}
	t := revokedAt
	token.RevokedAt = &t
	return nil
}

func (m *mockRefreshTokenRepo) RevokeByUserID(ctx context.Context, userID string, revokedAt time.Time) error {
	if m.revokeUserErr != nil {
		return m.revokeUserErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, token := range m.byTokenHash {
		if token.UserID != userID || token.RevokedAt != nil {
			continue
		}
		t := revokedAt
		token.RevokedAt = &t
	}
	return nil
}
