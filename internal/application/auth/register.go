package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	domainauth "github.com/santaniello/athena/internal/domain/auth"
)

// Register creates a new local account with a bcrypt-hashed password. A
// duplicate email is rejected with domainauth.ErrEmailAlreadyExists.
func (s *Service) Register(ctx context.Context, email, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("auth: hashing password: %w", err)
	}

	account := domainauth.Account{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	}
	return s.accounts.Create(ctx, account)
}
