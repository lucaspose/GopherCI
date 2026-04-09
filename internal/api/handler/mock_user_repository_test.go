package handler

import (
	"context"

	"github.com/lucaspose/goci/internal/models"
	"github.com/lucaspose/goci/internal/db/repository"
)

type mockUserRepo struct {
	users  map[string]*models.User
	byEmail map[string]*models.User
	createErr error
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users:   make(map[string]*models.User),
		byEmail: make(map[string]*models.User),
	}
}

func (m *mockUserRepo) Create(ctx context.Context, user *models.User) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.users[user.ID] = user
	m.byEmail[user.Email] = user
	return nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	u, ok := m.byEmail[email]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) Delete(ctx context.Context, id string) error {
	if _, ok := m.users[id]; !ok {
		return repository.ErrNotFound
	}
	u := m.users[id]
	delete(m.users, id)
	delete(m.byEmail, u.Email)
	return nil
}
