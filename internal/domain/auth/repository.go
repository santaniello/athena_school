package auth

import (
	"context"
	"errors"
)

// ErrAccountNotFound is returned when no account matches the given email.
var ErrAccountNotFound = errors.New("account not found")

// ErrEmailAlreadyExists is returned when Create is called with an email
// that already has an account.
var ErrEmailAlreadyExists = errors.New("email already exists")

// AccountRepository persists local Accounts. Today the only implementation
// is SQLite-backed (internal/infrastructure/sqlite); a future remote
// implementation can satisfy the same port without touching use cases.
type AccountRepository interface {
	Create(ctx context.Context, account Account) error
	FindByEmail(ctx context.Context, email string) (Account, error)
	UpdatePassword(ctx context.Context, id string, passwordHash string) error
	Delete(ctx context.Context, id string) error
}
