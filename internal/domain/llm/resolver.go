package llm

import "sync"

// EmbeddingModelSource reports the embedding model currently in effect.
// Consumers that only need the embedding model (the vector index loader)
// depend on this narrower port instead of the whole ModelResolver.
type EmbeddingModelSource interface {
	EmbeddingModel() string
}

// ModelResolver decides which model serves a chat task and which model
// produces embeddings. Implementations may change their answers between
// calls (see OverridableResolver), so callers resolve at call time rather
// than caching the result.
type ModelResolver interface {
	EmbeddingModelSource
	ModelFor(task TaskType) string
}

// DefaultModelResolver resolves every task through its tier and embeddings
// through EmbeddingModel — what the app did before models became
// configurable.
type DefaultModelResolver struct{}

// ModelFor returns the tier default for task.
func (DefaultModelResolver) ModelFor(task TaskType) string { return ModelFor(task) }

// EmbeddingModel returns the built-in embedding model.
func (DefaultModelResolver) EmbeddingModel() string { return EmbeddingModel }

// FixedEmbeddingModel is an EmbeddingModelSource that never changes.
type FixedEmbeddingModel string

// EmbeddingModel returns the fixed model ID.
func (m FixedEmbeddingModel) EmbeddingModel() string { return string(m) }

// OverridableResolver is a ModelResolver whose per-task and embedding models
// can be replaced at runtime. An absent or empty override means the default,
// so a fresh resolver behaves exactly like DefaultModelResolver. Safe for
// concurrent use: a settings save may land while a study turn is streaming.
type OverridableResolver struct {
	mu        sync.RWMutex
	overrides map[TaskType]string
	embedding string
}

// NewOverridableResolver creates a resolver with no overrides.
func NewOverridableResolver() *OverridableResolver {
	return &OverridableResolver{overrides: map[TaskType]string{}}
}

// SetOverrides replaces every override at once. The map is copied.
func (r *OverridableResolver) SetOverrides(overrides map[TaskType]string, embedding string) {
	copied := make(map[TaskType]string, len(overrides))
	for task, model := range overrides {
		copied[task] = model
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.overrides = copied
	r.embedding = embedding
}

// ModelFor returns the override for task, or its tier default when none is set.
func (r *OverridableResolver) ModelFor(task TaskType) string {
	r.mu.RLock()
	model := r.overrides[task]
	r.mu.RUnlock()
	if model != "" {
		return model
	}
	return ModelFor(task)
}

// EmbeddingModel returns the embedding override, or EmbeddingModel when none is set.
func (r *OverridableResolver) EmbeddingModel() string {
	r.mu.RLock()
	model := r.embedding
	r.mu.RUnlock()
	if model != "" {
		return model
	}
	return EmbeddingModel
}
