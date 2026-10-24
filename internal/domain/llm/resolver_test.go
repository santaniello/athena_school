package llm

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultModelResolver_ModelFor_returnsTierDefault(t *testing.T) {
	// Given the default resolver

	// When resolving a cheap-tier and a medium-tier task
	resolver := DefaultModelResolver{}

	// Then each resolves to its tier model
	assert.Equal(t, "openai/gpt-4o-mini", resolver.ModelFor(TaskConversationSummary))
	assert.Equal(t, "anthropic/claude-sonnet-4.5", resolver.ModelFor(TaskStudy))
}

func TestDefaultModelResolver_ModelFor_unknownTaskDefaultsToMedium(t *testing.T) {
	// Given a task type missing from the tier table

	// When resolving it
	model := DefaultModelResolver{}.ModelFor(TaskType("something_future"))

	// Then it falls back to the medium tier model
	assert.Equal(t, "anthropic/claude-sonnet-4.5", model)
}

func TestDefaultModelResolver_EmbeddingModel_returnsDefault(t *testing.T) {
	// Given the default resolver

	// When asking for the embedding model
	model := DefaultModelResolver{}.EmbeddingModel()

	// Then it is the built-in embedding model
	assert.Equal(t, EmbeddingModel, model)
}

func TestOverridableResolver_withoutOverrides_behavesLikeDefault(t *testing.T) {
	// Given a resolver nothing was ever set on
	resolver := NewOverridableResolver()

	// When resolving tasks and the embedding model
	// Then every answer equals the default resolver's
	assert.Equal(t, "anthropic/claude-sonnet-4.5", resolver.ModelFor(TaskStudy))
	assert.Equal(t, "openai/gpt-4o-mini", resolver.ModelFor(TaskQueryEnrichment))
	assert.Equal(t, EmbeddingModel, resolver.EmbeddingModel())
}

func TestOverridableResolver_ModelFor_usesOverrideForThatTaskOnly(t *testing.T) {
	// Given an override for the study task
	resolver := NewOverridableResolver()
	resolver.SetOverrides(map[TaskType]string{TaskStudy: "openai/gpt-5"}, "")

	// When resolving the study task and another one
	// Then only the study task changes
	assert.Equal(t, "openai/gpt-5", resolver.ModelFor(TaskStudy))
	assert.Equal(t, "openai/gpt-4o-mini", resolver.ModelFor(TaskConversationSummary))
}

func TestOverridableResolver_ModelFor_emptyOverrideMeansDefault(t *testing.T) {
	// Given an override that is an empty string
	resolver := NewOverridableResolver()
	resolver.SetOverrides(map[TaskType]string{TaskStudy: ""}, "")

	// When resolving that task
	// Then it resolves to the tier default
	assert.Equal(t, "anthropic/claude-sonnet-4.5", resolver.ModelFor(TaskStudy))
}

func TestOverridableResolver_EmbeddingModel_usesOverrideAndEmptyMeansDefault(t *testing.T) {
	// Given an embedding override
	resolver := NewOverridableResolver()
	resolver.SetOverrides(nil, "openai/text-embedding-3-large")
	assert.Equal(t, "openai/text-embedding-3-large", resolver.EmbeddingModel())

	// When the override is cleared
	resolver.SetOverrides(nil, "")

	// Then the default is back
	assert.Equal(t, EmbeddingModel, resolver.EmbeddingModel())
}

func TestOverridableResolver_SetOverrides_doesNotAliasTheCallersMap(t *testing.T) {
	// Given overrides set from a map the caller keeps mutating
	overrides := map[TaskType]string{TaskStudy: "openai/gpt-5"}
	resolver := NewOverridableResolver()
	resolver.SetOverrides(overrides, "")

	// When the caller changes its map afterwards
	overrides[TaskStudy] = "x/y"

	// Then the resolver keeps the values it was given
	assert.Equal(t, "openai/gpt-5", resolver.ModelFor(TaskStudy))
}

func TestOverridableResolver_isSafeForConcurrentUse(t *testing.T) {
	// Given a resolver read while it is being updated (run with -race)
	resolver := NewOverridableResolver()
	var wg sync.WaitGroup

	// When readers and a writer run together
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = resolver.ModelFor(TaskStudy)
			_ = resolver.EmbeddingModel()
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		resolver.SetOverrides(map[TaskType]string{TaskStudy: "a/b"}, "c/d")
	}()
	wg.Wait()

	// Then it ends in a consistent state
	assert.Equal(t, "a/b", resolver.ModelFor(TaskStudy))
}

func TestFixedEmbeddingModel_returnsItself(t *testing.T) {
	// Given a fixed embedding model value
	source := FixedEmbeddingModel("some/embedding")

	// When asking for the embedding model
	// Then it is the value itself
	assert.Equal(t, "some/embedding", source.EmbeddingModel())
}
