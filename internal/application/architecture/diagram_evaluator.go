package architecture

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// diagramContentType is the content type of a submitted draw.io diagram.
const diagramContentType = "drawio-xml"

// evaluationTask routes the evaluation call to the Architecture Lab model.
const evaluationTask = domainllm.TaskArchitectureLab

// DiagramEvaluator implements domainexercise.Evaluator for architecture
// challenges: it summarizes the submitted draw.io XML, asks the model for a
// level per rubric criterion in one call and returns them for the exercise
// engine to validate and score. See
// specs/phases/phase-03-architecture-lab/11-architecture-evaluator.md.
type DiagramEvaluator struct {
	llm      domainllm.Provider
	profiles ProfileReader
}

// NewDiagramEvaluator creates a DiagramEvaluator backed by the given ports.
func NewDiagramEvaluator(llm domainllm.Provider, profiles ProfileReader) *DiagramEvaluator {
	return &DiagramEvaluator{llm: llm, profiles: profiles}
}

// Evaluate makes exactly one model call. A submission that is not a readable,
// non-empty draw.io diagram fails with ErrInvalidDiagram or ErrEmptyDiagram
// before any call. An answer that is not the expected JSON, or names a
// criterion the rubric does not have, wraps domainexercise.ErrInvalidEvaluation.
func (e *DiagramEvaluator) Evaluate(ctx context.Context, input domainexercise.EvaluatorInput) (domainexercise.EvaluatorOutput, error) {
	summary, err := summarizeSubmission(input.Submission)
	if err != nil {
		return domainexercise.EvaluatorOutput{}, err
	}
	profile, err := e.profiles.ReadProfile(ctx)
	if err != nil {
		return domainexercise.EvaluatorOutput{}, fmt.Errorf("architecture: reading profile: %w", err)
	}
	response, err := e.llm.Chat(ctx, domainllm.ChatRequest{
		Task: evaluationTask,
		Messages: []domainllm.Message{
			{Role: "user", Content: BuildEvaluationPrompt(input.Challenge, input.Rubric, summary, starterBaseline(input.Challenge), profile.Language)},
		},
	})
	if err != nil {
		return domainexercise.EvaluatorOutput{}, fmt.Errorf("architecture: evaluating diagram: %w", err)
	}
	output, err := parseEvaluation(response.Content, input.Rubric)
	if err != nil {
		return domainexercise.EvaluatorOutput{}, err
	}
	output.Model = response.Model
	return output, nil
}

// starterBaseline summarizes the diagram a practice started from, or returns
// nil when there is none. A stored starter that cannot be read must not block
// the learner's evaluation, so it is treated as no baseline.
func starterBaseline(challenge domainexercise.Challenge) *DiagramSummary {
	if challenge.StarterDiagram == "" {
		return nil
	}
	summary, err := SummarizeDiagram(challenge.StarterDiagram)
	if err != nil {
		return nil
	}
	return &summary
}

type evaluationAnswer struct {
	Summary         *string           `json:"summary"`
	Strengths       *[]string         `json:"strengths"`
	Gaps            *[]string         `json:"gaps"`
	Recommendations *[]string         `json:"recommendations"`
	Criteria        []criterionAnswer `json:"criteria"`
}

type criterionAnswer struct {
	CriterionKey         string `json:"criterionKey"`
	Level                *int   `json:"level"`
	Justification        string `json:"justification"`
	Evidence             string `json:"evidence"`
	InsufficientEvidence *bool  `json:"insufficientEvidence"`
}

func invalidEvaluation(format string, args ...any) error {
	return fmt.Errorf("%w: %s", domainexercise.ErrInvalidEvaluation, fmt.Sprintf(format, args...))
}

// parseEvaluation reads the model's JSON into an EvaluatorOutput without a
// model id. It tolerates a Markdown code fence around the JSON and nothing else.
func parseEvaluation(content string, rubric domainexercise.RubricVersion) (domainexercise.EvaluatorOutput, error) {
	var answer evaluationAnswer
	if err := json.Unmarshal([]byte(stripCodeFence(content)), &answer); err != nil {
		return domainexercise.EvaluatorOutput{}, invalidEvaluation("the answer is not the expected JSON: %v", err)
	}
	if answer.Summary == nil || answer.Strengths == nil || answer.Gaps == nil || answer.Recommendations == nil {
		return domainexercise.EvaluatorOutput{}, invalidEvaluation("the answer is missing summary, strengths, gaps or recommendations")
	}
	if len(answer.Criteria) == 0 {
		return domainexercise.EvaluatorOutput{}, invalidEvaluation("the answer has no criteria")
	}
	ids := make(map[string]string, len(rubric.Criteria))
	for _, criterion := range rubric.Criteria {
		ids[criterion.Key] = criterion.ID
	}
	results := make([]domainexercise.CriterionResult, 0, len(answer.Criteria))
	for _, item := range answer.Criteria {
		id, known := ids[item.CriterionKey]
		if !known {
			return domainexercise.EvaluatorOutput{}, invalidEvaluation("unknown criterion %q", item.CriterionKey)
		}
		if item.Level == nil || item.InsufficientEvidence == nil {
			return domainexercise.EvaluatorOutput{}, invalidEvaluation("criterion %q has no level or insufficientEvidence", item.CriterionKey)
		}
		results = append(results, domainexercise.CriterionResult{
			CriterionID: id, Level: *item.Level, Justification: item.Justification,
			Evidence: item.Evidence, InsufficientEvidence: *item.InsufficientEvidence,
		})
	}
	return domainexercise.EvaluatorOutput{
		Summary: *answer.Summary, Strengths: *answer.Strengths, Gaps: *answer.Gaps,
		Recommendations: *answer.Recommendations, Results: results,
	}, nil
}

// stripCodeFence removes a surrounding ``` fence, with an optional language tag.
func stripCodeFence(content string) string {
	text := strings.TrimSpace(content)
	if !strings.HasPrefix(text, "```") {
		return text
	}
	if newline := strings.Index(text, "\n"); newline >= 0 {
		text = text[newline+1:]
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "```"))
}
