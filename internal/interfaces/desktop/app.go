// Package desktop holds the Wails bindings: thin adapters exposed to the
// React frontend. They validate input, call use cases, and return results.
package desktop

import (
	"context"

	"github.com/santaniello/athena/internal/application/auth"
	domainauth "github.com/santaniello/athena/internal/domain/auth"
)

// App is the Wails-bound struct exposed to the frontend.
type App struct {
	ctx      context.Context
	auth     *auth.Service
	sessions domainauth.SessionStore
}

// NewApp creates a new App instance backed by the given auth service and
// session store.
func NewApp(authService *auth.Service, sessions domainauth.SessionStore) *App {
	return &App{auth: authService, sessions: sessions}
}

// Startup is called by Wails once the frontend is ready. The context is
// stored so bound methods can call Wails runtime functions later.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}
