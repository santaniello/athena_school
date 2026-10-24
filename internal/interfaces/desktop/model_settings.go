package desktop

import (
	"github.com/santaniello/athena/internal/application/settings"
	domainconfig "github.com/santaniello/athena/internal/domain/config"
)

// ModelChoices is the desktop-facing DTO for the per-feature model settings.
// An empty field means "use the default".
type ModelChoices struct {
	Study               string `json:"study"`
	ConversationSummary string `json:"conversationSummary"`
	QueryEnrichment     string `json:"queryEnrichment"`
	NoteFormatting      string `json:"noteFormatting"`
	Embedding           string `json:"embedding"`
}

// ModelSettingsResult carries the user's choices and the defaults they fall
// back to, so the UI can show both and offer "Reset to default".
type ModelSettingsResult struct {
	Current  ModelChoices `json:"current"`
	Defaults ModelChoices `json:"defaults"`
}

// SaveModelSettingsResult reports what a save did.
type SaveModelSettingsResult struct {
	EmbeddingModelChanged bool `json:"embeddingModelChanged"`
}

// SelectableModelsResult lists the model IDs the selectors offer.
type SelectableModelsResult struct {
	Chat      []string `json:"chat"`
	Embedding []string `json:"embedding"`
}

func toModelChoices(models domainconfig.ModelSettings) ModelChoices {
	return ModelChoices{
		Study:               models.Study,
		ConversationSummary: models.ConversationSummary,
		QueryEnrichment:     models.QueryEnrichment,
		NoteFormatting:      models.NoteFormatting,
		Embedding:           models.Embedding,
	}
}

func (c ModelChoices) toSettings() domainconfig.ModelSettings {
	return domainconfig.ModelSettings{
		Study:               c.Study,
		ConversationSummary: c.ConversationSummary,
		QueryEnrichment:     c.QueryEnrichment,
		NoteFormatting:      c.NoteFormatting,
		Embedding:           c.Embedding,
	}
}

// GetModelSettings returns the stored model choices and the defaults. See
// specs/phases/phase-02-knowledge-engine/31-llm-model-param.md.
func (a *App) GetModelSettings() (ModelSettingsResult, error) {
	view, err := a.modelSettings.GetModelSettings()
	if err != nil {
		return ModelSettingsResult{}, err
	}
	return ModelSettingsResult{Current: toModelChoices(view.Current), Defaults: toModelChoices(view.Defaults)}, nil
}

// SaveModelSettings validates and saves the choices; they apply from the
// next LLM call. When the embedding model actually changed it also reloads
// the vector index, so retrieval immediately stops returning chunks of the
// previous model (they are kept and return if the user switches back), and
// emits "knowledge-index:status" with the outcome.
func (a *App) SaveModelSettings(input ModelChoices) (SaveModelSettingsResult, error) {
	result, err := a.modelSettings.SaveModelSettings(a.ctx, input.toSettings())
	if err != nil {
		return SaveModelSettingsResult{}, err
	}
	if result.EmbeddingModelChanged {
		a.RetryKnowledgeIndex()
	}
	return SaveModelSettingsResult{EmbeddingModelChanged: result.EmbeddingModelChanged}, nil
}

// ListSelectableModels returns the catalog IDs the model selectors offer.
func (a *App) ListSelectableModels() (SelectableModelsResult, error) {
	models, err := a.modelSettings.ListSelectableModels(a.ctx)
	if err != nil {
		return SelectableModelsResult{}, err
	}
	return SelectableModelsResult{Chat: models.Chat, Embedding: models.Embedding}, nil
}

// Option customizes an App at construction.
type Option func(*App)

// WithModelSettings wires the per-feature model settings use cases.
func WithModelSettings(service *settings.Service) Option {
	return func(a *App) { a.modelSettings = service }
}
