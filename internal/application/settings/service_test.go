package settings

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	settingsmocks "github.com/santaniello/athena/internal/application/settings/mocks"
	domainconfig "github.com/santaniello/athena/internal/domain/config"
	configmocks "github.com/santaniello/athena/internal/domain/config/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
)

type fixture struct {
	config  *configmocks.MockStore
	catalog *llmmocks.MockModelCatalog
	applier *settingsmocks.MockResolverUpdater
	service *Service
}

func newFixture(t *testing.T) fixture {
	config := configmocks.NewMockStore(t)
	catalog := llmmocks.NewMockModelCatalog(t)
	applier := settingsmocks.NewMockResolverUpdater(t)
	return fixture{config: config, catalog: catalog, applier: applier, service: NewService(config, catalog, applier)}
}

var chatCatalog = []domainllm.ModelInfo{
	{ID: "anthropic/claude-sonnet-4.5", ContextLength: 200000},
	{ID: "openai/gpt-4o-mini", ContextLength: 128000},
	{ID: "openai/gpt-5", ContextLength: 400000},
}

var embeddingCatalog = []domainllm.ModelInfo{
	{ID: "openai/text-embedding-3-small", ContextLength: 8192},
	{ID: "openai/text-embedding-3-large", ContextLength: 8192},
}

func TestGetModelSettings_returnsStoredChoicesAndTheDefaults(t *testing.T) {
	// Given a config with only the study model chosen
	f := newFixture(t)
	f.config.EXPECT().Load().Return(domainconfig.Config{
		OpenRouterKey: "k", Models: domainconfig.ModelSettings{Study: "openai/gpt-5"},
	}, nil).Once()

	// When reading the model settings
	view, err := f.service.GetModelSettings()

	// Then the choice is reported as is, alongside every default
	require.NoError(t, err)
	assert.Equal(t, domainconfig.ModelSettings{Study: "openai/gpt-5"}, view.Current)
	assert.Equal(t, domainconfig.ModelSettings{
		Study:               "anthropic/claude-sonnet-4.5",
		ConversationSummary: "openai/gpt-4o-mini",
		QueryEnrichment:     "openai/gpt-4o-mini",
		NoteFormatting:      "anthropic/claude-sonnet-4.5",
		Embedding:           "openai/text-embedding-3-small",
	}, view.Defaults)
}

func TestGetModelSettings_missingConfigFileMeansAllDefaults(t *testing.T) {
	// Given no config file yet
	f := newFixture(t)
	f.config.EXPECT().Load().Return(domainconfig.Config{}, fs.ErrNotExist).Once()

	// When reading the model settings
	view, err := f.service.GetModelSettings()

	// Then nothing is chosen, so everything resolves to its default
	require.NoError(t, err)
	assert.Equal(t, domainconfig.ModelSettings{}, view.Current)
}

func TestGetModelSettings_returnsAnErrorWhenTheConfigCannotBeRead(t *testing.T) {
	// Given a config that fails to load for another reason
	f := newFixture(t)
	boom := errors.New("disk on fire")
	f.config.EXPECT().Load().Return(domainconfig.Config{}, boom).Once()

	// When reading the model settings
	_, err := f.service.GetModelSettings()

	// Then the error is wrapped, not swallowed
	require.ErrorIs(t, err, boom)
}

func TestApplyStored_pushesTheStoredChoicesIntoTheResolver(t *testing.T) {
	// Given a stored config with several models chosen
	f := newFixture(t)
	f.config.EXPECT().Load().Return(domainconfig.Config{Models: domainconfig.ModelSettings{
		Study: "openai/gpt-5", ConversationSummary: "x/y", QueryEnrichment: "q/r", NoteFormatting: "n/f", Embedding: "e/m",
	}}, nil).Once()
	f.applier.EXPECT().SetOverrides(map[domainllm.TaskType]string{
		domainllm.TaskStudy:               "openai/gpt-5",
		domainllm.TaskConversationSummary: "x/y",
		domainllm.TaskQueryEnrichment:     "q/r",
		domainllm.TaskNoteFormatting:      "n/f",
	}, "e/m").Once()

	// When applying the stored settings at startup
	err := f.service.ApplyStored()

	// Then the resolver receives every choice
	require.NoError(t, err)
}

func TestApplyStored_missingConfigFileLeavesTheResolverOnDefaults(t *testing.T) {
	// Given no config file
	f := newFixture(t)
	f.config.EXPECT().Load().Return(domainconfig.Config{}, fs.ErrNotExist).Once()
	f.applier.EXPECT().SetOverrides(mock.Anything, "").Maybe()

	// When applying at startup
	err := f.service.ApplyStored()

	// Then it is not an error
	require.NoError(t, err)
}

func TestSaveModelSettings_validChoices_persistAndUpdateTheResolver(t *testing.T) {
	// Given a config with a key and a catalog that knows both chosen models
	f := newFixture(t)
	ctx := context.Background()
	f.catalog.EXPECT().ListModels(ctx).Return(chatCatalog, nil).Once()
	f.config.EXPECT().Load().Return(domainconfig.Config{OpenRouterKey: "sk-or-key"}, nil).Once()
	chosen := domainconfig.ModelSettings{Study: "openai/gpt-5", NoteFormatting: "openai/gpt-4o-mini"}
	f.config.EXPECT().Save(domainconfig.Config{OpenRouterKey: "sk-or-key", Models: chosen}).Return(nil).Once()
	f.applier.EXPECT().SetOverrides(map[domainllm.TaskType]string{
		domainllm.TaskStudy:               "openai/gpt-5",
		domainllm.TaskConversationSummary: "",
		domainllm.TaskQueryEnrichment:     "",
		domainllm.TaskNoteFormatting:      "openai/gpt-4o-mini",
	}, "").Once()

	// When saving them
	result, err := f.service.SaveModelSettings(ctx, chosen)

	// Then the key is preserved, the resolver is updated and the embedding
	// model is reported unchanged
	require.NoError(t, err)
	assert.False(t, result.EmbeddingModelChanged)
}

func TestSaveModelSettings_unknownChatModel_isRejectedAndPersistsNothing(t *testing.T) {
	// Given a catalog that does not list the chosen model
	f := newFixture(t)
	ctx := context.Background()
	f.catalog.EXPECT().ListModels(ctx).Return(chatCatalog, nil).Once()

	// When saving an unknown ID
	_, err := f.service.SaveModelSettings(ctx, domainconfig.ModelSettings{Study: "made/up"})

	// Then it is rejected with ErrUnknownModel naming the ID, with no save
	// and no resolver update (the strict mocks fail on any call)
	require.ErrorIs(t, err, ErrUnknownModel)
	assert.Contains(t, err.Error(), "made/up")
}

func TestSaveModelSettings_catalogUnreachable_refusesTheSave(t *testing.T) {
	// Given a catalog that cannot be fetched
	f := newFixture(t)
	ctx := context.Background()
	f.catalog.EXPECT().ListModels(ctx).Return(nil, errors.New("timeout")).Once()

	// When saving a model that cannot be verified
	_, err := f.service.SaveModelSettings(ctx, domainconfig.ModelSettings{Study: "openai/gpt-5"})

	// Then the save is refused rather than trusting the ID
	require.ErrorIs(t, err, ErrCatalogUnavailable)
}

func TestSaveModelSettings_emptyChoices_doNotNeedTheCatalog(t *testing.T) {
	// Given every setting reset to default
	f := newFixture(t)
	ctx := context.Background()
	f.config.EXPECT().Load().Return(domainconfig.Config{OpenRouterKey: "k", Models: domainconfig.ModelSettings{Study: "openai/gpt-5"}}, nil).Once()
	f.config.EXPECT().Save(domainconfig.Config{OpenRouterKey: "k"}).Return(nil).Once()
	f.applier.EXPECT().SetOverrides(mock.Anything, "").Once()

	// When saving all-empty settings with the catalog down (no catalog expectations)
	_, err := f.service.SaveModelSettings(ctx, domainconfig.ModelSettings{})

	// Then resetting works offline
	require.NoError(t, err)
}

func TestSaveModelSettings_trimsWhitespaceAroundIDs(t *testing.T) {
	// Given a chosen ID padded with spaces
	f := newFixture(t)
	ctx := context.Background()
	f.catalog.EXPECT().ListModels(ctx).Return(chatCatalog, nil).Once()
	f.config.EXPECT().Load().Return(domainconfig.Config{}, nil).Once()
	f.config.EXPECT().Save(domainconfig.Config{Models: domainconfig.ModelSettings{Study: "openai/gpt-5"}}).Return(nil).Once()
	f.applier.EXPECT().SetOverrides(mock.Anything, "").Once()

	// When saving
	_, err := f.service.SaveModelSettings(ctx, domainconfig.ModelSettings{Study: "  openai/gpt-5 "})

	// Then the trimmed ID is validated and stored
	require.NoError(t, err)
}

func TestSaveModelSettings_embeddingModel_isValidatedAgainstTheEmbeddingCatalog(t *testing.T) {
	// Given an embedding model listed only in the embeddings catalog
	f := newFixture(t)
	ctx := context.Background()
	f.catalog.EXPECT().ListEmbeddingModels(ctx).Return(embeddingCatalog, nil).Once()
	f.config.EXPECT().Load().Return(domainconfig.Config{}, nil).Once()
	f.config.EXPECT().Save(domainconfig.Config{Models: domainconfig.ModelSettings{Embedding: "openai/text-embedding-3-large"}}).Return(nil).Once()
	f.applier.EXPECT().SetOverrides(mock.Anything, "openai/text-embedding-3-large").Once()

	// When saving it
	result, err := f.service.SaveModelSettings(ctx, domainconfig.ModelSettings{Embedding: "openai/text-embedding-3-large"})

	// Then it is accepted and reported as a change from the default
	require.NoError(t, err)
	assert.True(t, result.EmbeddingModelChanged)
}

func TestSaveModelSettings_chatModelIsNotAcceptedAsEmbeddingModel(t *testing.T) {
	// Given a chat model ID chosen as the embedding model
	f := newFixture(t)
	ctx := context.Background()
	f.catalog.EXPECT().ListEmbeddingModels(ctx).Return(embeddingCatalog, nil).Once()

	// When saving it
	_, err := f.service.SaveModelSettings(ctx, domainconfig.ModelSettings{Embedding: "openai/gpt-5"})

	// Then it is rejected
	require.ErrorIs(t, err, ErrUnknownModel)
}

func TestSaveModelSettings_embeddingChange_isReportedOnlyWhenTheEffectiveValueChanges(t *testing.T) {
	tests := []struct {
		name    string
		before  string
		after   string
		changed bool
	}{
		{name: "default to another model", before: "", after: "openai/text-embedding-3-large", changed: true},
		{name: "another model back to empty", before: "openai/text-embedding-3-large", after: "", changed: true},
		{name: "empty to the explicit default is no change", before: "", after: "openai/text-embedding-3-small", changed: false},
		{name: "explicit default to empty is no change", before: "openai/text-embedding-3-small", after: "", changed: false},
		{name: "same value again is no change", before: "openai/text-embedding-3-large", after: "openai/text-embedding-3-large", changed: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a stored embedding choice
			f := newFixture(t)
			ctx := context.Background()
			if tt.after != "" {
				f.catalog.EXPECT().ListEmbeddingModels(ctx).Return(embeddingCatalog, nil).Once()
			}
			f.config.EXPECT().Load().Return(domainconfig.Config{Models: domainconfig.ModelSettings{Embedding: tt.before}}, nil).Once()
			f.config.EXPECT().Save(mock.Anything).Return(nil).Once()
			f.applier.EXPECT().SetOverrides(mock.Anything, tt.after).Once()

			// When saving another one
			result, err := f.service.SaveModelSettings(ctx, domainconfig.ModelSettings{Embedding: tt.after})

			// Then the change flag follows the effective value
			require.NoError(t, err)
			assert.Equal(t, tt.changed, result.EmbeddingModelChanged)
		})
	}
}

func TestSaveModelSettings_saveFailure_doesNotUpdateTheResolver(t *testing.T) {
	// Given a config store that fails to write
	f := newFixture(t)
	ctx := context.Background()
	boom := errors.New("read-only filesystem")
	f.config.EXPECT().Load().Return(domainconfig.Config{}, nil).Once()
	f.config.EXPECT().Save(mock.Anything).Return(boom).Once()

	// When saving empty settings
	_, err := f.service.SaveModelSettings(ctx, domainconfig.ModelSettings{})

	// Then the error surfaces and the resolver keeps its previous state
	// (no SetOverrides expectation: any call would fail the test)
	require.ErrorIs(t, err, boom)
}

func TestSaveModelSettings_loadFailure_isReturnedWithoutSaving(t *testing.T) {
	// Given a config that cannot be read
	f := newFixture(t)
	boom := errors.New("corrupt yaml")
	f.config.EXPECT().Load().Return(domainconfig.Config{}, boom).Once()

	// When saving
	_, err := f.service.SaveModelSettings(context.Background(), domainconfig.ModelSettings{})

	// Then it fails without overwriting a file it could not read
	require.ErrorIs(t, err, boom)
}

func TestListSelectableModels_returnsSortedIDsOfBothCatalogs(t *testing.T) {
	// Given chat and embedding catalogs
	f := newFixture(t)
	ctx := context.Background()
	f.catalog.EXPECT().ListModels(ctx).Return([]domainllm.ModelInfo{{ID: "b/b"}, {ID: "a/a"}, {ID: ""}}, nil).Once()
	f.catalog.EXPECT().ListEmbeddingModels(ctx).Return([]domainllm.ModelInfo{{ID: "z/emb"}, {ID: "e/emb"}}, nil).Once()

	// When listing the selectable models
	models, err := f.service.ListSelectableModels(ctx)

	// Then blank IDs are dropped and each list is sorted
	require.NoError(t, err)
	assert.Equal(t, []string{"a/a", "b/b"}, models.Chat)
	assert.Equal(t, []string{"e/emb", "z/emb"}, models.Embedding)
}

func TestListSelectableModels_catalogFailure_isReportedAsUnavailable(t *testing.T) {
	// Given a chat catalog that fails
	f := newFixture(t)
	ctx := context.Background()
	f.catalog.EXPECT().ListModels(ctx).Return(nil, errors.New("offline")).Once()

	// When listing
	_, err := f.service.ListSelectableModels(ctx)

	// Then the UI can show a retry state
	require.ErrorIs(t, err, ErrCatalogUnavailable)
}
