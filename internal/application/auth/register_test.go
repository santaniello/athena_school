package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"

	domainauth "github.com/santaniello/athena/internal/domain/auth"
	"github.com/santaniello/athena/internal/domain/auth/mocks"
)

func TestRegister_createsAccountWithBcryptHashedPassword(t *testing.T) {
	// Given a repository that accepts the new account
	repo := mocks.NewMockAccountRepository(t)
	sessions := mocks.NewMockSessionStore(t)
	ctx := context.Background()
	const email = "user@example.com"
	const password = "correct horse battery staple"

	repo.EXPECT().
		Create(ctx, mock.MatchedBy(func(account domainauth.Account) bool {
			if account.Email != email || account.ID == "" {
				return false
			}
			return bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)) == nil
		})).
		Return(nil).
		Once()

	service := NewService(repo, sessions)

	// When registering with that email and password
	err := service.Register(ctx, email, password)

	// Then it succeeds and the repository received a bcrypt-hashed password
	assert.NoError(t, err)
}

func TestRegister_propagatesDuplicateEmailError(t *testing.T) {
	// Given a repository that already has an account with this email
	repo := mocks.NewMockAccountRepository(t)
	sessions := mocks.NewMockSessionStore(t)
	ctx := context.Background()

	const email = "user@example.com"
	repo.EXPECT().
		Create(ctx, mock.MatchedBy(func(account domainauth.Account) bool {
			return account.Email == email
		})).
		Return(domainauth.ErrEmailAlreadyExists).
		Once()

	service := NewService(repo, sessions)

	// When registering with a duplicate email
	err := service.Register(ctx, email, "some-password")

	// Then the duplicate error is propagated to the caller
	assert.ErrorIs(t, err, domainauth.ErrEmailAlreadyExists)
}
