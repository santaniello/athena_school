package llm

import "context"

// ModelInfo is one entry of a provider's model catalog: an exact model ID
// and the context window it supports, in tokens.
type ModelInfo struct {
	ID            string
	ContextLength int
}

// ModelCatalog lists the models a provider currently offers. Implemented by
// the OpenRouter adapter; consumed by an application-level cache/resolver
// (see ModelContextResolver) rather than called directly on the hot path of
// a chat response.
//
// Chat models and embedding models are separate listings on OpenRouter, so
// each has its own method.
type ModelCatalog interface {
	ListModels(ctx context.Context) ([]ModelInfo, error)
	ListEmbeddingModels(ctx context.Context) ([]ModelInfo, error)
}

// ModelContextResolver resolves a model ID to its context window length,
// backed by an in-memory cache built from ModelCatalog. CachedContextLength
// is a memory-only lookup and never performs I/O, so the successful-response
// path can call it without risking a network wait. RefreshContextLength
// performs or joins a catalog refresh and then repeats the lookup.
type ModelContextResolver interface {
	CachedContextLength(modelID string) (int, bool)
	RefreshContextLength(ctx context.Context, modelID string) (int, error)
}
