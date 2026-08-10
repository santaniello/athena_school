// Package desktop holds the Wails bindings: thin adapters exposed to the
// React frontend. They validate input, call use cases, and return results.
package desktop

import "context"

// App is the Wails-bound struct exposed to the frontend.
type App struct {
	ctx context.Context
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{}
}

// Startup is called by Wails once the frontend is ready. The context is
// stored so bound methods can call Wails runtime functions later.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}
