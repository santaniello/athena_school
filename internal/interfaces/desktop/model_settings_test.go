package desktop

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	applicationknowledge "github.com/santaniello/athena/internal/application/knowledge"
	"github.com/santaniello/athena/internal/application/settings"
	domainconfig "github.com/santaniello/athena/internal/domain/config"
	configmocks "github.com/santaniello/athena/internal/domain/config/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
)

type modelSettingsFixture struct {
	config  *configmocks.MockStore
	catalog *llmmocks.MockModelCatalog
	app     *App
	emitted []IndexStatusResult
}

func newModelSettingsApp(t *testing.T, loader *applicationknowledge.IndexLoader) *modelSettingsFixture {
	t.Helper()
	f := &modelSettingsFixture{
		config:  configmocks.NewMockStore(t),
		catalog: llmmocks.NewMockModelCatalog(t),
	}
	service := settings.NewService(f.config, f.catalog, domainllm.NewOverridableResolver())
	f.app = NewApp(nil, nil, nil, nil, nil, nil, nil, nil, loader, nil, WithModelSettings(service))
	f.app.Startup(context.Background())
	f.app.emit = func(_ context.Context, event string, data ...interface{}) {
		if event == eventKnowledgeIndexStatus {
			f.emitted = append(f.emitted, data[0].(IndexStatusResult))
		}
	}
	return f
}

func TestApp_GetModelSettings_returnsChoicesAndDefaultsForTheUI(t *testing.T) {
	// Given a config with the study model chosen
	f := newModelSettingsApp(t, nil)
	f.config.EXPECT().Load().Return(domainconfig.Config{Models: domainconfig.ModelSettings{Study: "openai/gpt-5"}}, nil).Once()

	// When the frontend asks for the model settings
	result, err := f.app.GetModelSettings()

	// Then it gets the choice (empty meaning default) next to every default
	require.NoError(t, err)
	assert.Equal(t, ModelChoices{Study: "openai/gpt-5"}, result.Current)
	assert.Equal(t, ModelChoices{
		Study:               "anthropic/claude-sonnet-4.5",
		ConversationSummary: "openai/gpt-4o-mini",
		QueryEnrichment:     "openai/gpt-4o-mini",
		NoteFormatting:      "anthropic/claude-sonnet-4.5",
		Embedding:           "openai/text-embedding-3-small",
	}, result.Defaults)
}

func TestApp_GetModelSettings_propagatesALoadFailure(t *testing.T) {
	// Given a config that cannot be read
	f := newModelSettingsApp(t, nil)
	f.config.EXPECT().Load().Return(domainconfig.Config{}, errors.New("corrupt")).Once()

	// When the frontend asks for the model settings
	_, err := f.app.GetModelSettings()

	// Then the error reaches the frontend
	assert.Error(t, err)
}

func TestApp_SaveModelSettings_chatModelChange_savesWithoutTouchingTheIndex(t *testing.T) {
	// Given a catalog that knows the chosen chat model (the nil index loader
	// would panic if the binding tried to reload it)
	f := newModelSettingsApp(t, nil)
	f.catalog.EXPECT().ListModels(context.Background()).Return([]domainllm.ModelInfo{{ID: "openai/gpt-5"}}, nil).Once()
	f.config.EXPECT().Load().Return(domainconfig.Config{OpenRouterKey: "k"}, nil).Once()
	f.config.EXPECT().Save(domainconfig.Config{OpenRouterKey: "k", Models: domainconfig.ModelSettings{Study: "openai/gpt-5"}}).Return(nil).Once()

	// When saving it
	result, err := f.app.SaveModelSettings(ModelChoices{Study: "openai/gpt-5"})

	// Then the embedding model is reported unchanged and no index event fires
	require.NoError(t, err)
	assert.False(t, result.EmbeddingModelChanged)
	assert.Empty(t, f.emitted)
}

func TestApp_SaveModelSettings_unknownModel_isRejected(t *testing.T) {
	// Given a catalog that does not list the model
	f := newModelSettingsApp(t, nil)
	f.catalog.EXPECT().ListModels(context.Background()).Return([]domainllm.ModelInfo{{ID: "openai/gpt-5"}}, nil).Once()

	// When saving an unknown ID
	_, err := f.app.SaveModelSettings(ModelChoices{Study: "made/up"})

	// Then the sentinel reaches the frontend and nothing is saved
	require.ErrorIs(t, err, settings.ErrUnknownModel)
}

func TestApp_SaveModelSettings_embeddingChange_reloadsTheIndexAndEmitsItsStatus(t *testing.T) {
	// Given a ready index loader and a catalog that lists the new embedding model
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	resolver := domainllm.NewOverridableResolver()
	loader := applicationknowledge.NewIndexLoader(chunks, store, resolver)
	f := newModelSettingsApp(t, loader)
	// The app's own resolver is the one the service updates; share it with the loader.
	service := settings.NewService(f.config, f.catalog, resolver)
	f.app.modelSettings = service
	f.catalog.EXPECT().ListEmbeddingModels(ctx).Return([]domainllm.ModelInfo{{ID: "openai/text-embedding-3-large"}}, nil).Once()
	f.config.EXPECT().Load().Return(domainconfig.Config{}, nil).Once()
	f.config.EXPECT().Save(mock.Anything).Return(nil).Once()
	// The reload must ask for chunks of the NEW model only
	chunks.EXPECT().ListCurrent(f.app.ctx, "openai/text-embedding-3-large").Return(domainknowledge.ChunkLoadResult{}, nil).Once()
	store.EXPECT().ReplaceAll(f.app.ctx, []domainknowledge.Chunk{}).Return(nil).Once()

	// When saving the new embedding model
	result, err := f.app.SaveModelSettings(ModelChoices{Embedding: "openai/text-embedding-3-large"})

	// Then the change is reported, the index was reloaded for the new model,
	// and the frontend got the fresh index status
	require.NoError(t, err)
	assert.True(t, result.EmbeddingModelChanged)
	require.Len(t, f.emitted, 1)
	assert.Equal(t, string(domainknowledge.IndexStateReady), f.emitted[0].State)
}

func TestApp_ListSelectableModels_returnsBothCatalogsForTheSelectors(t *testing.T) {
	// Given chat and embedding catalogs
	f := newModelSettingsApp(t, nil)
	f.catalog.EXPECT().ListModels(context.Background()).Return([]domainllm.ModelInfo{{ID: "b/b"}, {ID: "a/a"}}, nil).Once()
	f.catalog.EXPECT().ListEmbeddingModels(context.Background()).Return([]domainllm.ModelInfo{{ID: "e/e"}}, nil).Once()

	// When the frontend lists the selectable models
	result, err := f.app.ListSelectableModels()

	// Then it gets sorted ID lists
	require.NoError(t, err)
	assert.Equal(t, []string{"a/a", "b/b"}, result.Chat)
	assert.Equal(t, []string{"e/e"}, result.Embedding)
}

func TestApp_ListSelectableModels_catalogFailure_isReturnedToTheFrontend(t *testing.T) {
	// Given an unreachable catalog
	f := newModelSettingsApp(t, nil)
	f.catalog.EXPECT().ListModels(context.Background()).Return(nil, errors.New("offline")).Once()

	// When listing
	_, err := f.app.ListSelectableModels()

	// Then the unavailable sentinel reaches the frontend
	require.ErrorIs(t, err, settings.ErrCatalogUnavailable)
}
