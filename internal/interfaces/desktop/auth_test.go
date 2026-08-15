package desktop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/santaniello/athena/internal/application/auth"
	domainauth "github.com/santaniello/athena/internal/domain/auth"
	"github.com/santaniello/athena/internal/domain/auth/mocks"
)

func newTestApp(t *testing.T, accounts domainauth.AccountRepository, sessions domainauth.SessionStore) *App {
	t.Helper()
	app := NewApp(auth.NewService(accounts, sessions), sessions, nil, nil, nil)
	app.Startup(context.Background())
	return app
}

func matchAccountWithEmail(email string) interface{} {
	return mock.MatchedBy(func(account domainauth.Account) bool {
		return account.Email == email && account.ID != "" && account.PasswordHash != ""
	})
}

func matchSessionForAccount(accountID string) interface{} {
	return mock.MatchedBy(func(session domainauth.Session) bool {
		return session.AccountID == accountID
	})
}

func TestApp_Register_createsAccount(t *testing.T) {
	// Given an App backed by an account repository that accepts new accounts
	accounts := mocks.NewMockAccountRepository(t)
	accounts.EXPECT().
		Create(context.Background(), matchAccountWithEmail("new@athena.dev")).
		Return(nil).
		Once()
	app := newTestApp(t, accounts, mocks.NewMockSessionStore(t))

	// When registering a new account
	err := app.Register("new@athena.dev", "s3cr3t-password")

	// Then it succeeds
	require.NoError(t, err)
}

func TestApp_Register_propagatesDuplicateEmailError(t *testing.T) {
	// Given an App backed by an account repository that rejects a duplicate email
	accounts := mocks.NewMockAccountRepository(t)
	accounts.EXPECT().
		Create(context.Background(), matchAccountWithEmail("dup@athena.dev")).
		Return(domainauth.ErrEmailAlreadyExists).
		Once()
	app := newTestApp(t, accounts, mocks.NewMockSessionStore(t))

	// When registering with an already-used email
	err := app.Register("dup@athena.dev", "s3cr3t-password")

	// Then the domain sentinel error is surfaced unchanged
	assert.ErrorIs(t, err, domainauth.ErrEmailAlreadyExists)
}

func TestApp_Login_returnsLoginResult_onValidCredentials(t *testing.T) {
	// Given an App backed by an account repository with a matching account
	hash, err := bcrypt.GenerateFromPassword([]byte("s3cr3t-password"), bcrypt.DefaultCost)
	require.NoError(t, err)
	account := domainauth.Account{ID: "acc-1", Email: "user@athena.dev", PasswordHash: string(hash)}
	accounts := mocks.NewMockAccountRepository(t)
	accounts.EXPECT().FindByEmail(context.Background(), "user@athena.dev").Return(account, nil).Once()
	sessions := mocks.NewMockSessionStore(t)
	sessions.EXPECT().Save(matchSessionForAccount("acc-1")).Return(nil).Once()
	app := newTestApp(t, accounts, sessions)

	// When logging in with the matching password
	result, err := app.Login("user@athena.dev", "s3cr3t-password")

	// Then it returns the account info without the password hash
	require.NoError(t, err)
	assert.Equal(t, LoginResult{AccountID: "acc-1", Email: "user@athena.dev"}, result)
}

func TestApp_Login_returnsInvalidCredentials_onWrongPassword(t *testing.T) {
	// Given an App backed by an account repository with a matching account
	hash, err := bcrypt.GenerateFromPassword([]byte("s3cr3t-password"), bcrypt.DefaultCost)
	require.NoError(t, err)
	account := domainauth.Account{ID: "acc-1", Email: "user@athena.dev", PasswordHash: string(hash)}
	accounts := mocks.NewMockAccountRepository(t)
	accounts.EXPECT().FindByEmail(context.Background(), "user@athena.dev").Return(account, nil).Once()
	app := newTestApp(t, accounts, mocks.NewMockSessionStore(t))

	// When logging in with the wrong password
	result, err := app.Login("user@athena.dev", "wrong-password")

	// Then it fails with the invalid-credentials sentinel and an empty result
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
	assert.Equal(t, LoginResult{}, result)
}

func TestApp_ResetLocalAccount_deletesAccount(t *testing.T) {
	// Given an App backed by an account repository with an existing account
	account := domainauth.Account{ID: "acc-1", Email: "user@athena.dev"}
	accounts := mocks.NewMockAccountRepository(t)
	accounts.EXPECT().FindByEmail(context.Background(), "user@athena.dev").Return(account, nil).Once()
	accounts.EXPECT().Delete(context.Background(), "acc-1").Return(nil).Once()
	app := newTestApp(t, accounts, mocks.NewMockSessionStore(t))

	// When resetting the local account
	err := app.ResetLocalAccount("user@athena.dev")

	// Then it succeeds
	require.NoError(t, err)
}

func TestApp_ResetLocalAccount_propagatesAccountNotFoundError(t *testing.T) {
	// Given an App backed by an account repository with no matching account
	accounts := mocks.NewMockAccountRepository(t)
	accounts.EXPECT().
		FindByEmail(context.Background(), "missing@athena.dev").
		Return(domainauth.Account{}, domainauth.ErrAccountNotFound).
		Once()
	app := newTestApp(t, accounts, mocks.NewMockSessionStore(t))

	// When resetting an account that does not exist
	err := app.ResetLocalAccount("missing@athena.dev")

	// Then the domain sentinel error is surfaced unchanged
	assert.ErrorIs(t, err, domainauth.ErrAccountNotFound)
}
