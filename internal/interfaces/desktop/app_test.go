package desktop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewApp_returnsNonNilApp(t *testing.T) {
	// Given no prior state
	// When creating a new App
	app := NewApp()

	// Then the App instance is ready to use
	assert.NotNil(t, app)
}

func TestStartup_storesContext(t *testing.T) {
	// Given a new App and a context
	app := NewApp()
	ctx := context.Background()

	// When Startup is called with that context
	app.Startup(ctx)

	// Then the App stores the context for later runtime calls
	assert.Equal(t, ctx, app.ctx)
}
