package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainauth "github.com/santaniello/athena/internal/domain/auth"
	"github.com/santaniello/athena/internal/domain/auth/mocks"
)

func TestResetLocalAccount_deletesTheAccountForThatEmail(t *testing.T) {
	// Given a repository with an existing account for this email
	repo := mocks.NewMockAccountRepository(t)
	sessions := mocks.NewMockSessionStore(t)
	ctx := context.Background()
	const email = "user@example.com"
	account := domainauth.Account{ID: "acc-1", Email: email}

	repo.EXPECT().FindByEmail(ctx, email).Return(account, nil).Once()
	repo.EXPECT().Delete(ctx, account.ID).Return(nil).Once()

	service := NewService(repo, sessions)

	// When resetting the local account for that email
	err := service.ResetLocalAccount(ctx, email)

	// Then the account is deleted so the email can be registered again
	require.NoError(t, err)
}

func TestResetLocalAccount_returnsNotFound_whenNoAccountExistsForEmail(t *testing.T) {
	// Given a repository with no account for this email
	repo := mocks.NewMockAccountRepository(t)
	sessions := mocks.NewMockSessionStore(t)
	ctx := context.Background()

	repo.EXPECT().
		FindByEmail(ctx, "missing@example.com").
		Return(domainauth.Account{}, domainauth.ErrAccountNotFound).
		Once()

	service := NewService(repo, sessions)

	// When resetting a local account that does not exist
	err := service.ResetLocalAccount(ctx, "missing@example.com")

	// Then the not-found error is surfaced instead of silently succeeding
	assert.ErrorIs(t, err, domainauth.ErrAccountNotFound)
}
