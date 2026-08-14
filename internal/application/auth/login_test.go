package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	domainauth "github.com/santaniello/athena/internal/domain/auth"
	"github.com/santaniello/athena/internal/domain/auth/mocks"
)

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return string(hash)
}

func TestLogin_succeedsAndSavesSession_whenCredentialsAreValid(t *testing.T) {
	// Given a repository with a matching account and a session store
	repo := mocks.NewMockAccountRepository(t)
	sessions := mocks.NewMockSessionStore(t)
	ctx := context.Background()
	const email = "user@example.com"
	const password = "correct horse battery staple"
	account := domainauth.Account{ID: "acc-1", Email: email, PasswordHash: hashPassword(t, password)}

	repo.EXPECT().FindByEmail(ctx, email).Return(account, nil).Once()
	sessions.EXPECT().
		Save(mock.MatchedBy(func(session domainauth.Session) bool {
			return session.AccountID == account.ID && !session.CreatedAt.IsZero()
		})).
		Return(nil).
		Once()

	service := NewService(repo, sessions)

	// When logging in with the correct password
	result, err := service.Login(ctx, email, password)

	// Then it succeeds, returns the account, and writes a session
	require.NoError(t, err)
	assert.Equal(t, account.ID, result.ID)
}

func TestLogin_returnsInvalidCredentials_whenEmailNotFound(t *testing.T) {
	// Given a repository with no account for this email
	repo := mocks.NewMockAccountRepository(t)
	sessions := mocks.NewMockSessionStore(t)
	ctx := context.Background()

	repo.EXPECT().
		FindByEmail(ctx, "missing@example.com").
		Return(domainauth.Account{}, domainauth.ErrAccountNotFound).
		Once()

	service := NewService(repo, sessions)

	// When logging in with that email
	_, err := service.Login(ctx, "missing@example.com", "any-password")

	// Then it fails with the generic invalid-credentials error
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestLogin_returnsInvalidCredentials_whenPasswordDoesNotMatch(t *testing.T) {
	// Given a repository with an account whose stored hash is for a different password
	repo := mocks.NewMockAccountRepository(t)
	sessions := mocks.NewMockSessionStore(t)
	ctx := context.Background()
	const email = "user@example.com"
	account := domainauth.Account{ID: "acc-1", Email: email, PasswordHash: hashPassword(t, "the-real-password")}

	repo.EXPECT().FindByEmail(ctx, email).Return(account, nil).Once()

	service := NewService(repo, sessions)

	// When logging in with the wrong password
	_, err := service.Login(ctx, email, "wrong-password")

	// Then it fails with the generic invalid-credentials error
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestLogin_propagatesUnexpectedRepositoryError(t *testing.T) {
	// Given a repository that fails for a reason unrelated to credentials
	repo := mocks.NewMockAccountRepository(t)
	sessions := mocks.NewMockSessionStore(t)
	ctx := context.Background()
	dbErr := errors.New("database unavailable")

	repo.EXPECT().
		FindByEmail(ctx, "user@example.com").
		Return(domainauth.Account{}, dbErr).
		Once()

	service := NewService(repo, sessions)

	// When logging in
	_, err := service.Login(ctx, "user@example.com", "any-password")

	// Then the unexpected error is surfaced, not masked as invalid credentials
	assert.ErrorIs(t, err, dbErr)
	assert.NotErrorIs(t, err, ErrInvalidCredentials)
}
