package desktop

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/santaniello/athena/internal/application/auth"
	domainauth "github.com/santaniello/athena/internal/domain/auth"
	"github.com/santaniello/athena/internal/domain/auth/mocks"
)

func TestNewApp_returnsNonNilApp(t *testing.T) {
	// Given an auth service and session store
	authService := auth.NewService(mocks.NewMockAccountRepository(t), mocks.NewMockSessionStore(t))
	sessions := mocks.NewMockSessionStore(t)

	// When creating a new App
	app := NewApp(authService, sessions, nil, nil, nil, nil, nil, nil, nil)

	// Then the App instance is ready to use
	assert.NotNil(t, app)
}

func TestStartup_storesContext(t *testing.T) {
	// Given a new App and a context
	authService := auth.NewService(mocks.NewMockAccountRepository(t), mocks.NewMockSessionStore(t))
	app := NewApp(authService, mocks.NewMockSessionStore(t), nil, nil, nil, nil, nil, nil, nil)
	ctx := context.Background()

	// When Startup is called with that context
	app.Startup(ctx)

	// Then the App stores the context for later runtime calls
	assert.Equal(t, ctx, app.ctx)
}

func TestHasLocalSession_returnsTrue_whenSessionStoreHasSession(t *testing.T) {
	// Given an App backed by a session store with a saved session
	sessions := mocks.NewMockSessionStore(t)
	sessions.EXPECT().Load().Return(domainauth.Session{AccountID: "acc-1"}, nil).Once()
	authService := auth.NewService(mocks.NewMockAccountRepository(t), mocks.NewMockSessionStore(t))
	app := NewApp(authService, sessions, nil, nil, nil, nil, nil, nil, nil)

	// When checking for a local session
	has := app.HasLocalSession()

	// Then it reports true
	assert.True(t, has)
}

func TestHasLocalSession_returnsFalse_whenSessionStoreHasNoSession(t *testing.T) {
	// Given an App backed by a session store with no saved session
	sessions := mocks.NewMockSessionStore(t)
	sessions.EXPECT().Load().Return(domainauth.Session{}, errors.New("session: reading session file: no such file")).Once()
	authService := auth.NewService(mocks.NewMockAccountRepository(t), mocks.NewMockSessionStore(t))
	app := NewApp(authService, sessions, nil, nil, nil, nil, nil, nil, nil)

	// When checking for a local session
	has := app.HasLocalSession()

	// Then it reports false
	assert.False(t, has)
}
