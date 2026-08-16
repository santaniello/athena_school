// Package desktop holds the Wails bindings: thin adapters exposed to the
// React frontend. They validate input, call use cases, and return results.
package desktop

import (
	"context"

	"github.com/santaniello/athena/internal/application/auth"
	"github.com/santaniello/athena/internal/application/onboarding"
	"github.com/santaniello/athena/internal/application/study"
	domainauth "github.com/santaniello/athena/internal/domain/auth"
	domainconfig "github.com/santaniello/athena/internal/domain/config"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
)

// App is the Wails-bound struct exposed to the frontend.
type App struct {
	ctx        context.Context
	auth       *auth.Service
	sessions   domainauth.SessionStore
	onboarding *onboarding.Service
	profiles   domainprofile.Store
	config     domainconfig.Store
	study      *study.Service
}

// NewApp creates a new App instance backed by the given auth service,
// session store, onboarding service, profile store, config store and study
// service.
func NewApp(
	authService *auth.Service,
	sessions domainauth.SessionStore,
	onboardingService *onboarding.Service,
	profiles domainprofile.Store,
	config domainconfig.Store,
	studyService *study.Service,
) *App {
	return &App{
		auth:       authService,
		sessions:   sessions,
		onboarding: onboardingService,
		profiles:   profiles,
		config:     config,
		study:      studyService,
	}
}

// Startup is called by Wails once the frontend is ready. The context is
// stored so bound methods can call Wails runtime functions later.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}
