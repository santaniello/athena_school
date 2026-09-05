package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTierFor_returnsCheap_forOnboardingTask(t *testing.T) {
	// Given the onboarding task type

	// When resolving its tier
	tier := TierFor(TaskOnboarding)

	// Then it is cheap
	assert.Equal(t, TierCheap, tier)
}

func TestTierFor_returnsCheap_forKnowledgeExtractionTask(t *testing.T) {
	// Given the knowledge extraction task type

	// When resolving its tier
	tier := TierFor(TaskKnowledgeExtraction)

	// Then it is cheap
	assert.Equal(t, TierCheap, tier)
}

func TestTierFor_returnsCheap_forKnowledgeReconciliationTask(t *testing.T) {
	// Given the knowledge reconciliation task type

	// When resolving its tier
	tier := TierFor(TaskKnowledgeReconciliation)

	// Then it is cheap
	assert.Equal(t, TierCheap, tier)
}

func TestTierFor_returnsMedium_forStudyTask(t *testing.T) {
	// Given the study task type

	// When resolving its tier
	tier := TierFor(TaskStudy)

	// Then it is medium
	assert.Equal(t, TierMedium, tier)
}

func TestTierFor_returnsMedium_forChallengeFeedbackTask(t *testing.T) {
	// Given the challenge feedback task type

	// When resolving its tier
	tier := TierFor(TaskChallengeFeedback)

	// Then it is medium
	assert.Equal(t, TierMedium, tier)
}

func TestTierFor_returnsPremium_forInterviewEvaluationTask(t *testing.T) {
	// Given the interview evaluation task type

	// When resolving its tier
	tier := TierFor(TaskInterviewEvaluation)

	// Then it is premium
	assert.Equal(t, TierPremium, tier)
}

func TestTierFor_returnsPremium_forComplexReasoningTask(t *testing.T) {
	// Given the complex reasoning task type

	// When resolving its tier
	tier := TierFor(TaskComplexReasoning)

	// Then it is premium
	assert.Equal(t, TierPremium, tier)
}

func TestTierFor_returnsMedium_forUnrecognizedTaskType(t *testing.T) {
	// Given a task type that does not appear in the tier table
	unknown := TaskType("something_future_specs_add_later")

	// When resolving its tier
	tier := TierFor(unknown)

	// Then it defaults to medium rather than an empty/unrouted tier
	assert.Equal(t, TierMedium, tier)
}

func TestModelFor_returnsGPT4oMini_forACheapTierTask(t *testing.T) {
	// Given a task routed to the cheap tier

	// When resolving its model
	model := ModelFor(TaskOnboarding)

	// Then it is the cheap tier default
	assert.Equal(t, "openai/gpt-4o-mini", model)
}

func TestModelFor_returnsGPT4oMini_forKnowledgeReconciliationTask(t *testing.T) {
	// Given the knowledge reconciliation task, routed to the cheap tier

	// When resolving its model
	model := ModelFor(TaskKnowledgeReconciliation)

	// Then it is the cheap tier default
	assert.Equal(t, "openai/gpt-4o-mini", model)
}

func TestModelFor_returnsClaudeSonnet_forAMediumTierTask(t *testing.T) {
	// Given a task routed to the medium tier

	// When resolving its model
	model := ModelFor(TaskStudy)

	// Then it is the medium tier default
	assert.Equal(t, "anthropic/claude-sonnet-4.5", model)
}

func TestModelFor_returnsClaudeOpus_forAPremiumTierTask(t *testing.T) {
	// Given a task routed to the premium tier

	// When resolving its model
	model := ModelFor(TaskInterviewEvaluation)

	// Then it is the premium tier default
	assert.Equal(t, "anthropic/claude-opus-4.5", model)
}

func TestEmbeddingModel_isTextEmbedding3Small(t *testing.T) {
	// Embeddings do not go through the tier system — they always use the
	// same dedicated embedding model.
	assert.Equal(t, "openai/text-embedding-3-small", EmbeddingModel)
}

func TestFreeFallbackModel_isOpenRouterFree(t *testing.T) {
	// The free-model fallback always routes through OpenRouter's own
	// auto-router between currently available free models, rather than a
	// specific :free model that could be discontinued.
	assert.Equal(t, "openrouter/free", FreeFallbackModel)
}
