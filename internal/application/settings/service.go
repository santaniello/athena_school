// Package settings implements the use cases behind the Settings screen's
// model choices: which model serves each AI feature. See
// specs/phases/phase-02-knowledge-engine/31-llm-model-param.md.
package settings

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	domainconfig "github.com/santaniello/athena/internal/domain/config"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// ResolverUpdater receives the user's choices so the next LLM call uses
// them without a restart. Defined here (consumer side) per Go convention;
// implemented by *domainllm.OverridableResolver.
type ResolverUpdater interface {
	SetOverrides(overrides map[domainllm.TaskType]string, embedding string)
}

// Service reads and saves the per-feature model choices.
type Service struct {
	config   domainconfig.Store
	catalog  domainllm.ModelCatalog
	resolver ResolverUpdater
}

// NewService creates a Service persisting through config, verifying model
// IDs against catalog and publishing accepted choices to resolver.
func NewService(config domainconfig.Store, catalog domainllm.ModelCatalog, resolver ResolverUpdater) *Service {
	return &Service{config: config, catalog: catalog, resolver: resolver}
}

// ModelSettingsView is what the Settings screen shows: the user's choices
// (empty means default) next to the defaults they fall back to.
type ModelSettingsView struct {
	Current  domainconfig.ModelSettings
	Defaults domainconfig.ModelSettings
}

// SaveResult reports what a save did.
type SaveResult struct {
	// EmbeddingModelChanged is true only when the effective embedding model
	// (empty meaning the default) differs from before the save. The caller
	// must then reload the vector index.
	EmbeddingModelChanged bool
}

// SelectableModels are the model IDs the user may pick from.
type SelectableModels struct {
	Chat      []string
	Embedding []string
}

// defaultModelSettings is every setting's built-in default.
func defaultModelSettings() domainconfig.ModelSettings {
	defaults := domainllm.DefaultModelResolver{}
	return domainconfig.ModelSettings{
		Study:               defaults.ModelFor(domainllm.TaskStudy),
		ConversationSummary: defaults.ModelFor(domainllm.TaskConversationSummary),
		QueryEnrichment:     defaults.ModelFor(domainllm.TaskQueryEnrichment),
		NoteFormatting:      defaults.ModelFor(domainllm.TaskNoteFormatting),
		Embedding:           defaults.EmbeddingModel(),
	}
}

// loadConfig reads the config; a missing file is an empty config.
func (s *Service) loadConfig() (domainconfig.Config, error) {
	cfg, err := s.config.Load()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return domainconfig.Config{}, nil
		}
		return domainconfig.Config{}, fmt.Errorf("settings: loading config: %w", err)
	}
	return cfg, nil
}

// GetModelSettings returns the stored choices and the defaults.
func (s *Service) GetModelSettings() (ModelSettingsView, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return ModelSettingsView{}, err
	}
	return ModelSettingsView{Current: cfg.Models, Defaults: defaultModelSettings()}, nil
}

// ApplyStored publishes the stored choices to the resolver. Called once at
// startup so the first LLM call already honours them.
func (s *Service) ApplyStored() error {
	cfg, err := s.loadConfig()
	if err != nil {
		return err
	}
	s.apply(cfg.Models)
	return nil
}

func (s *Service) apply(models domainconfig.ModelSettings) {
	s.resolver.SetOverrides(map[domainllm.TaskType]string{
		domainllm.TaskStudy:               models.Study,
		domainllm.TaskConversationSummary: models.ConversationSummary,
		domainllm.TaskQueryEnrichment:     models.QueryEnrichment,
		domainllm.TaskNoteFormatting:      models.NoteFormatting,
	}, models.Embedding)
}

// SaveModelSettings validates the chosen IDs against the catalog, persists
// them next to the existing config (the OpenRouter key is preserved) and
// then publishes them to the resolver. Nothing is persisted or published if
// any ID is unknown or the catalog cannot be fetched. Empty means default
// and never needs the catalog, so resetting works offline.
func (s *Service) SaveModelSettings(ctx context.Context, models domainconfig.ModelSettings) (SaveResult, error) {
	models = trimmed(models)
	if err := s.validate(ctx, models); err != nil {
		return SaveResult{}, err
	}

	cfg, err := s.loadConfig()
	if err != nil {
		return SaveResult{}, err
	}
	changed := effectiveEmbedding(cfg.Models.Embedding) != effectiveEmbedding(models.Embedding)
	cfg.Models = models
	if err := s.config.Save(cfg); err != nil {
		return SaveResult{}, fmt.Errorf("settings: saving model settings: %w", err)
	}

	s.apply(models)
	return SaveResult{EmbeddingModelChanged: changed}, nil
}

func effectiveEmbedding(chosen string) string {
	if chosen == "" {
		return domainllm.EmbeddingModel
	}
	return chosen
}

func trimmed(models domainconfig.ModelSettings) domainconfig.ModelSettings {
	return domainconfig.ModelSettings{
		Study:               strings.TrimSpace(models.Study),
		ConversationSummary: strings.TrimSpace(models.ConversationSummary),
		QueryEnrichment:     strings.TrimSpace(models.QueryEnrichment),
		NoteFormatting:      strings.TrimSpace(models.NoteFormatting),
		Embedding:           strings.TrimSpace(models.Embedding),
	}
}

// validate checks every non-empty choice against its catalog. A catalog is
// fetched only when something needs it.
func (s *Service) validate(ctx context.Context, models domainconfig.ModelSettings) error {
	chat := []string{models.Study, models.ConversationSummary, models.QueryEnrichment, models.NoteFormatting}
	if slices.ContainsFunc(chat, func(id string) bool { return id != "" }) {
		listed, err := s.catalog.ListModels(ctx)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrCatalogUnavailable, err)
		}
		if err := requireListed(chat, listed); err != nil {
			return err
		}
	}
	if models.Embedding != "" {
		listed, err := s.catalog.ListEmbeddingModels(ctx)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrCatalogUnavailable, err)
		}
		return requireListed([]string{models.Embedding}, listed)
	}
	return nil
}

func requireListed(chosen []string, listed []domainllm.ModelInfo) error {
	known := make(map[string]bool, len(listed))
	for _, m := range listed {
		known[m.ID] = true
	}
	for _, id := range chosen {
		if id != "" && !known[id] {
			return fmt.Errorf("%w: %q", ErrUnknownModel, id)
		}
	}
	return nil
}

// ListSelectableModels returns the sorted IDs of the chat and embedding
// models the user may choose.
func (s *Service) ListSelectableModels(ctx context.Context) (SelectableModels, error) {
	chat, err := s.catalog.ListModels(ctx)
	if err != nil {
		return SelectableModels{}, fmt.Errorf("%w: %w", ErrCatalogUnavailable, err)
	}
	embedding, err := s.catalog.ListEmbeddingModels(ctx)
	if err != nil {
		return SelectableModels{}, fmt.Errorf("%w: %w", ErrCatalogUnavailable, err)
	}
	return SelectableModels{Chat: sortedIDs(chat), Embedding: sortedIDs(embedding)}, nil
}

func sortedIDs(models []domainllm.ModelInfo) []string {
	ids := make([]string, 0, len(models))
	for _, m := range models {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	slices.Sort(ids)
	return ids
}
