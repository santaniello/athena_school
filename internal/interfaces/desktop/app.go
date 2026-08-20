// Package desktop holds the Wails bindings: thin adapters exposed to the
// React frontend. They validate input, call use cases, and return results.
package desktop

import (
	"context"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/santaniello/athena/internal/application/auth"
	"github.com/santaniello/athena/internal/application/folder"
	applicationingest "github.com/santaniello/athena/internal/application/ingest"
	applicationknowledge "github.com/santaniello/athena/internal/application/knowledge"
	"github.com/santaniello/athena/internal/application/onboarding"
	"github.com/santaniello/athena/internal/application/study"
	domainauth "github.com/santaniello/athena/internal/domain/auth"
	domainconfig "github.com/santaniello/athena/internal/domain/config"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
)

// App is the Wails-bound struct exposed to the frontend.
type App struct {
	ctx           context.Context
	auth          *auth.Service
	sessions      domainauth.SessionStore
	onboarding    *onboarding.Service
	profiles      domainprofile.Store
	config        domainconfig.Store
	study         *study.Service
	folder        *folder.Service
	knowledge     *applicationknowledge.Service
	ingest        *applicationingest.Service
	index         *applicationknowledge.IndexLoader
	apiKeyUpdater domainllm.APIKeyUpdater
	// emit defaults to wailsruntime.EventsEmit, which calls log.Fatal (i.e.
	// os.Exit) when a.ctx was never produced by the real Wails runtime —
	// exactly the case in tests, which use context.Background(). Tests
	// override this field with a fake to observe emitted events safely.
	emit func(ctx context.Context, eventName string, data ...interface{})
	// openDirectory defaults to wailsruntime.OpenDirectoryDialog, which
	// has the same real-runtime requirement as emit above. Tests override
	// it with a fake to drive PickNotesFolder without a real OS dialog.
	openDirectory func(ctx context.Context, options wailsruntime.OpenDialogOptions) (string, error)
}

// NewApp creates a new App instance backed by the given auth service,
// session store, onboarding service, profile store, config store, study
// service, folder service, knowledge service, notes-import service, the
// knowledge vector index coordinator, and the live LLM client's key updater.
func NewApp(
	authService *auth.Service,
	sessions domainauth.SessionStore,
	onboardingService *onboarding.Service,
	profiles domainprofile.Store,
	config domainconfig.Store,
	studyService *study.Service,
	folderService *folder.Service,
	knowledgeService *applicationknowledge.Service,
	ingestService *applicationingest.Service,
	apiKeyUpdater domainllm.APIKeyUpdater,
	indexLoader *applicationknowledge.IndexLoader,
) *App {
	return &App{
		auth:          authService,
		sessions:      sessions,
		onboarding:    onboardingService,
		profiles:      profiles,
		config:        config,
		study:         studyService,
		folder:        folderService,
		knowledge:     knowledgeService,
		ingest:        ingestService,
		apiKeyUpdater: apiKeyUpdater,
		index:         indexLoader,
		emit:          wailsruntime.EventsEmit,
		openDirectory: wailsruntime.OpenDirectoryDialog,
	}
}

// Startup is called by Wails once the frontend is ready. The context is
// stored so bound methods can call Wails runtime functions later.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}
