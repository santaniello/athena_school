package llm

// TaskType identifies the kind of work a Chat/ChatStream call is for. It
// drives model routing via TierFor.
type TaskType string

// Task types recognized by TierFor. Future specs (challenge, interview,
// knowledge extraction) add their own use of these as they are built.
const (
	TaskOnboarding              TaskType = "onboarding"
	TaskKnowledgeExtraction     TaskType = "knowledge_extraction"
	TaskKnowledgeReconciliation TaskType = "knowledge_reconciliation"
	TaskStudy                   TaskType = "study"
	TaskChallengeFeedback       TaskType = "challenge_feedback"
	TaskInterviewEvaluation     TaskType = "interview_evaluation"
	TaskComplexReasoning        TaskType = "complex_reasoning"
)

// Tier is a cost/capability class of model.
type Tier string

// Tiers recognized by ModelFor, from cheapest to most capable.
const (
	TierCheap   Tier = "cheap"
	TierMedium  Tier = "medium"
	TierPremium Tier = "premium"
)

// EmbeddingModel is the fixed model used for Provider.Embeddings.
// Embeddings do not go through the tier system.
const EmbeddingModel = "openai/text-embedding-3-small"

// FreeFallbackModel is OpenRouter's own auto-router between currently
// available free models. Chat/ChatStream retry against it once when the
// account has run out of credits, regardless of the task's tier.
const FreeFallbackModel = "openrouter/free"

var tierModels = map[Tier]string{
	TierCheap:   "openai/gpt-4o-mini",
	TierMedium:  "anthropic/claude-sonnet-4.5",
	TierPremium: "anthropic/claude-opus-4.5",
}

// TierFor maps a task type to its model tier. An unrecognized task type
// defaults to TierMedium rather than leaving it unrouted.
func TierFor(task TaskType) Tier {
	switch task {
	case TaskOnboarding, TaskKnowledgeExtraction, TaskKnowledgeReconciliation:
		return TierCheap
	case TaskStudy, TaskChallengeFeedback:
		return TierMedium
	case TaskInterviewEvaluation, TaskComplexReasoning:
		return TierPremium
	default:
		return TierMedium
	}
}

// ModelFor returns the OpenRouter model ID to use for the given task.
func ModelFor(task TaskType) string {
	return tierModels[TierFor(task)]
}
