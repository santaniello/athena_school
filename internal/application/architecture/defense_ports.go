package architecture

import (
	"context"
	"fmt"
	"strings"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// defenseTask routes the defense calls to the Architecture Lab model.
const defenseTask = domainllm.TaskArchitectureLab

// summarizeSubmission reads a submitted draw.io diagram into the summary the
// prompts use. A submission that is not a readable, non-empty draw.io diagram
// is ErrInvalidDiagram or ErrEmptyDiagram, before any model call.
func summarizeSubmission(submission domainexercise.Submission) (DiagramSummary, error) {
	if submission.ContentType != diagramContentType {
		return DiagramSummary{}, fmt.Errorf("%w: content type %q", ErrInvalidDiagram, submission.ContentType)
	}
	return SummarizeDiagram(submission.Content)
}

// DefenseQuestionGenerator implements domainexercise.QuestionGenerator with
// one model call per question, through the provider that records its usage.
type DefenseQuestionGenerator struct {
	llm      domainllm.Provider
	profiles ProfileReader
}

// NewDefenseQuestionGenerator creates a DefenseQuestionGenerator backed by the given ports.
func NewDefenseQuestionGenerator(llm domainllm.Provider, profiles ProfileReader) *DefenseQuestionGenerator {
	return &DefenseQuestionGenerator{llm: llm, profiles: profiles}
}

// GenerateQuestion asks the model to write the question for the criterion in
// the input, in the user's assistant language. It makes exactly one model call
// and never retries.
func (g *DefenseQuestionGenerator) GenerateQuestion(ctx context.Context, input domainexercise.QuestionInput) (string, error) {
	summary, err := summarizeSubmission(input.Submission)
	if err != nil {
		return "", err
	}
	profile, err := g.profiles.ReadProfile(ctx)
	if err != nil {
		return "", fmt.Errorf("architecture: reading profile: %w", err)
	}
	response, err := g.llm.Chat(ctx, domainllm.ChatRequest{
		Task:     defenseTask,
		Messages: buildQuestionPrompt(input, summary, profile.Language),
	})
	if err != nil {
		return "", fmt.Errorf("architecture: generating defense question: %w", err)
	}
	return strings.TrimSpace(response.Content), nil
}

// UnderstandingEvaluator implements domainexercise.UnderstandingEvaluator: one
// model call that judges every answer of the defense against the frozen rubric.
type UnderstandingEvaluator struct {
	llm      domainllm.Provider
	profiles ProfileReader
}

// NewUnderstandingEvaluator creates an UnderstandingEvaluator backed by the given ports.
func NewUnderstandingEvaluator(llm domainllm.Provider, profiles ProfileReader) *UnderstandingEvaluator {
	return &UnderstandingEvaluator{llm: llm, profiles: profiles}
}

// EvaluateUnderstanding makes exactly one model call. An answer that is not
// the expected JSON, or names a criterion the rubric does not have, wraps
// domainexercise.ErrInvalidEvaluation; whether it judges exactly the criteria
// that were asked is checked by the domain.
func (e *UnderstandingEvaluator) EvaluateUnderstanding(ctx context.Context, input domainexercise.UnderstandingInput) (domainexercise.EvaluatorOutput, error) {
	summary, err := summarizeSubmission(input.Submission)
	if err != nil {
		return domainexercise.EvaluatorOutput{}, err
	}
	profile, err := e.profiles.ReadProfile(ctx)
	if err != nil {
		return domainexercise.EvaluatorOutput{}, fmt.Errorf("architecture: reading profile: %w", err)
	}
	response, err := e.llm.Chat(ctx, domainllm.ChatRequest{
		Task: defenseTask,
		Messages: []domainllm.Message{
			{Role: "user", Content: BuildUnderstandingPrompt(input, summary, profile.Language)},
		},
	})
	if err != nil {
		return domainexercise.EvaluatorOutput{}, fmt.Errorf("architecture: evaluating understanding: %w", err)
	}
	output, err := parseEvaluation(response.Content, input.Rubric)
	if err != nil {
		return domainexercise.EvaluatorOutput{}, err
	}
	output.Model = response.Model
	return output, nil
}
