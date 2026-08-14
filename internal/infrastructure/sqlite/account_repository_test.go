package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/auth"
)

func newTestRepository(t *testing.T) *AccountRepository {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewAccountRepository(db)
}

func TestAccountRepository_CreateThenFindByEmail_returnsStoredAccount(t *testing.T) {
	// Given a repository and a new account
	repo := newTestRepository(t)
	ctx := context.Background()
	account := auth.Account{
		ID:           "acc-1",
		Email:        "user@example.com",
		PasswordHash: "hashed-password",
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
	}

	// When creating it and then looking it up by email
	require.NoError(t, repo.Create(ctx, account))
	found, err := repo.FindByEmail(ctx, "user@example.com")

	// Then the stored account matches what was created
	require.NoError(t, err)
	assert.Equal(t, account.ID, found.ID)
	assert.Equal(t, account.Email, found.Email)
	assert.Equal(t, account.PasswordHash, found.PasswordHash)
	assert.True(t, account.CreatedAt.Equal(found.CreatedAt))
}

func TestAccountRepository_Create_rejectsDuplicateEmail(t *testing.T) {
	// Given a repository with an existing account
	repo := newTestRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, auth.Account{ID: "acc-1", Email: "user@example.com", PasswordHash: "h1"}))

	// When creating another account with the same email
	err := repo.Create(ctx, auth.Account{ID: "acc-2", Email: "user@example.com", PasswordHash: "h2"})

	// Then it is rejected with ErrEmailAlreadyExists
	assert.ErrorIs(t, err, auth.ErrEmailAlreadyExists)
}

func TestAccountRepository_FindByEmail_returnsNotFoundForUnknownEmail(t *testing.T) {
	// Given an empty repository
	repo := newTestRepository(t)

	// When looking up an email that has no account
	_, err := repo.FindByEmail(context.Background(), "missing@example.com")

	// Then it returns ErrAccountNotFound
	assert.ErrorIs(t, err, auth.ErrAccountNotFound)
}

func TestAccountRepository_UpdatePassword_changesStoredHash(t *testing.T) {
	// Given a repository with an existing account
	repo := newTestRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, auth.Account{ID: "acc-1", Email: "user@example.com", PasswordHash: "old-hash"}))

	// When updating its password hash
	err := repo.UpdatePassword(ctx, "acc-1", "new-hash")

	// Then the stored hash reflects the update
	require.NoError(t, err)
	found, findErr := repo.FindByEmail(ctx, "user@example.com")
	require.NoError(t, findErr)
	assert.Equal(t, "new-hash", found.PasswordHash)
}

func TestAccountRepository_UpdatePassword_returnsNotFoundForUnknownID(t *testing.T) {
	// Given an empty repository
	repo := newTestRepository(t)

	// When updating the password of an account that does not exist
	err := repo.UpdatePassword(context.Background(), "missing-id", "new-hash")

	// Then it returns ErrAccountNotFound
	assert.ErrorIs(t, err, auth.ErrAccountNotFound)
}

func TestAccountRepository_Delete_removesAccountSoEmailCanBeReused(t *testing.T) {
	// Given a repository with an existing account
	repo := newTestRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, auth.Account{ID: "acc-1", Email: "user@example.com", PasswordHash: "hash"}))

	// When deleting it and creating a new account with the same email
	require.NoError(t, repo.Delete(ctx, "acc-1"))
	err := repo.Create(ctx, auth.Account{ID: "acc-2", Email: "user@example.com", PasswordHash: "new-hash"})

	// Then the re-registration succeeds
	require.NoError(t, err)
}

func TestAccountRepository_Delete_returnsNotFoundForUnknownID(t *testing.T) {
	// Given an empty repository
	repo := newTestRepository(t)

	// When deleting an account that does not exist
	err := repo.Delete(context.Background(), "missing-id")

	// Then it returns ErrAccountNotFound
	assert.True(t, errors.Is(err, auth.ErrAccountNotFound))
}
