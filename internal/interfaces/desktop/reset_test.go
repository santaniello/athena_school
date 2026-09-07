package desktop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationreset "github.com/santaniello/athena/internal/application/reset"
	"github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	resetmocks "github.com/santaniello/athena/internal/domain/reset/mocks"
)

func newTestResetApp(
	t *testing.T,
	resetter *resetmocks.MockResetter,
	vectorStore *knowledgemocks.MockVectorStore,
) (app *App, reloaded *bool) {
	t.Helper()
	service := applicationreset.NewService(resetter, vectorStore)
	app = NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, service)
	app.Startup(context.Background())
	reloaded = new(bool)
	app.reloadApp = func(context.Context) { *reloaded = true }
	return app, reloaded
}

func TestApp_ResetLocalData_resetsStorageThenReloadsTheApp(t *testing.T) {
	// Given an App backed by a resetter and vector store that both succeed
	resetter := resetmocks.NewMockResetter(t)
	resetter.EXPECT().Reset(context.Background()).Return(nil).Once()
	vectorStore := knowledgemocks.NewMockVectorStore(t)
	vectorStore.EXPECT().ReplaceAll(context.Background(), []knowledge.Chunk(nil)).Return(nil).Once()
	app, reloaded := newTestResetApp(t, resetter, vectorStore)

	// When resetting local data
	err := app.ResetLocalData()

	// Then it succeeds and the app is reloaded
	require.NoError(t, err)
	assert.True(t, *reloaded)
}

func TestApp_ResetLocalData_reloadsTheAppEvenWhenTheVectorIndexFailsToClear(t *testing.T) {
	// Given an App whose resetter succeeds but whose vector store fails to
	// clear, after the durable delete has already happened
	resetter := resetmocks.NewMockResetter(t)
	resetter.EXPECT().Reset(context.Background()).Return(nil).Once()
	vectorStore := knowledgemocks.NewMockVectorStore(t)
	vectorStore.EXPECT().ReplaceAll(context.Background(), []knowledge.Chunk(nil)).Return(assert.AnError).Once()
	app, reloaded := newTestResetApp(t, resetter, vectorStore)

	// When resetting local data
	err := app.ResetLocalData()

	// Then it still succeeds and reloads — SQLite is already durably empty,
	// so a stale UI would be worse than a briefly stale in-memory index
	require.NoError(t, err)
	assert.True(t, *reloaded)
}

func TestApp_ResetLocalData_propagatesError_withoutReloadingTheApp(t *testing.T) {
	// Given an App backed by a resetter that fails
	resetter := resetmocks.NewMockResetter(t)
	resetter.EXPECT().Reset(context.Background()).Return(assert.AnError).Once()
	vectorStore := knowledgemocks.NewMockVectorStore(t)
	app, reloaded := newTestResetApp(t, resetter, vectorStore)

	// When resetting local data
	err := app.ResetLocalData()

	// Then the error is surfaced unchanged and the app is never reloaded
	assert.ErrorIs(t, err, assert.AnError)
	assert.False(t, *reloaded)
}
