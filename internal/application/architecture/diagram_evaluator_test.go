package architecture

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
)

func testEvaluatorInput() domainexercise.EvaluatorInput {
	rubric := testEvaluationRubric()
	rubric.Criteria[0].ID, rubric.Criteria[1].ID = "c-scal", "c-res"
	return domainexercise.EvaluatorInput{
		Challenge:  testEvaluationChallenge(),
		Rubric:     rubric,
		Submission: domainexercise.Submission{ContentType: "drawio-xml", Content: wrapFile(sampleCells)},
	}
}

const validEvaluation = `{
  "summary": "A solid start.",
  "strengths": ["Clear API"], "gaps": ["No cache failover"], "recommendations": ["Add replicas"],
  "criteria": [
    {"criterionKey": "scalability", "level": 3, "justification": "Cache shown.", "evidence": "API -> Database", "insufficientEvidence": false},
    {"criterionKey": "resilience", "level": 0, "justification": "Nothing written.", "insufficientEvidence": true}
  ]
}`

func expectEvaluationCall(t *testing.T, provider *llmmocks.MockProvider, input domainexercise.EvaluatorInput, language string) *llmmocks.MockProvider_Chat_Call {
	t.Helper()
	summary, err := SummarizeDiagram(input.Submission.Content)
	require.NoError(t, err)
	return provider.EXPECT().Chat(context.Background(), domainllm.ChatRequest{
		Task: domainllm.TaskArchitectureLab,
		Messages: []domainllm.Message{
			{Role: "user", Content: BuildEvaluationPrompt(input.Challenge, input.Rubric, summary, language)},
		},
	})
}

func TestDiagramEvaluator_Evaluate_mapsTheModelAnswerToEvaluatorOutput(t *testing.T) {
	// Given a provider that answers with a valid evaluation and a Portuguese assistant
	provider := llmmocks.NewMockProvider(t)
	input := testEvaluatorInput()
	expectEvaluationCall(t, provider, input, "pt").Return(domainllm.ChatResponse{Content: validEvaluation, Model: "vendor/model-x"}, nil).Once()

	// When evaluating
	output, err := NewDiagramEvaluator(provider, fakeLanguage{code: "pt"}).Evaluate(context.Background(), input)

	// Then the result maps criterion keys to ids, keeps the flag and records the exact model
	require.NoError(t, err)
	assert.Equal(t, domainexercise.EvaluatorOutput{
		Model: "vendor/model-x", Summary: "A solid start.",
		Strengths: []string{"Clear API"}, Gaps: []string{"No cache failover"}, Recommendations: []string{"Add replicas"},
		Results: []domainexercise.CriterionResult{
			{CriterionID: "c-scal", Level: 3, Justification: "Cache shown.", Evidence: "API -> Database"},
			{CriterionID: "c-res", Level: 0, Justification: "Nothing written.", InsufficientEvidence: true},
		},
	}, output)
}

func TestDiagramEvaluator_Evaluate_acceptsAnswerInAMarkdownFence(t *testing.T) {
	// Given a model that wraps its JSON in a code fence
	provider := llmmocks.NewMockProvider(t)
	input := testEvaluatorInput()
	expectEvaluationCall(t, provider, input, "en").Return(domainllm.ChatResponse{Content: "```json\n" + validEvaluation + "\n```"}, nil).Once()

	// When evaluating
	output, err := NewDiagramEvaluator(provider, fakeLanguage{code: "en"}).Evaluate(context.Background(), input)

	// Then the JSON is read
	require.NoError(t, err)
	assert.Len(t, output.Results, 2)
}

func TestDiagramEvaluator_Evaluate_badModelOutputIsAnInvalidEvaluation(t *testing.T) {
	for name, answer := range map[string]string{
		"not json":              "I think it is good.",
		"no criteria":           `{"summary": "x"}`,
		"empty criteria":        `{"criteria": []}`,
		"unknown criterion":     `{"criteria": [{"criterionKey": "speed", "level": 1, "justification": "x"}]}`,
		"missing criterion key": `{"criteria": [{"level": 1, "justification": "x"}]}`,
		"missing level":         `{"criteria": [{"criterionKey": "scalability", "justification": "x"}]}`,
		"fractional level":      `{"criteria": [{"criterionKey": "scalability", "level": 2.5, "justification": "x"}]}`,
		"level as text":         `{"criteria": [{"criterionKey": "scalability", "level": "high", "justification": "x"}]}`,
		"trailing text":         validEvaluation + " thanks",
	} {
		t.Run(name, func(t *testing.T) {
			// Given a provider that answers with bad output
			provider := llmmocks.NewMockProvider(t)
			input := testEvaluatorInput()
			expectEvaluationCall(t, provider, input, "en").Return(domainllm.ChatResponse{Content: answer}, nil).Once()

			// When evaluating
			_, err := NewDiagramEvaluator(provider, fakeLanguage{code: "en"}).Evaluate(context.Background(), input)

			// Then the engine can treat it as an invalid evaluation
			assert.ErrorIs(t, err, domainexercise.ErrInvalidEvaluation)
		})
	}
}

func TestDiagramEvaluator_Evaluate_aProviderErrorIsReturned(t *testing.T) {
	// Given a failing provider, and a cancelled one
	for _, cause := range []error{errors.New("boom"), context.Canceled} {
		provider := llmmocks.NewMockProvider(t)
		input := testEvaluatorInput()
		expectEvaluationCall(t, provider, input, "en").Return(domainllm.ChatResponse{}, cause).Once()

		// When evaluating
		_, err := NewDiagramEvaluator(provider, fakeLanguage{code: "en"}).Evaluate(context.Background(), input)

		// Then the cause is in the chain and is not an invalid evaluation
		assert.ErrorIs(t, err, cause)
		assert.NotErrorIs(t, err, domainexercise.ErrInvalidEvaluation)
	}
}

func TestDiagramEvaluator_Evaluate_aProfileErrorStopsBeforeTheModel(t *testing.T) {
	// Given a profile that cannot be read and a provider that must not be called
	provider := llmmocks.NewMockProvider(t)
	cause := errors.New("no profile")

	// When evaluating
	_, err := NewDiagramEvaluator(provider, fakeLanguage{err: cause}).Evaluate(context.Background(), testEvaluatorInput())

	// Then the error is returned
	assert.ErrorIs(t, err, cause)
}

func TestDiagramEvaluator_Evaluate_aBadDiagramNeverReachesTheModel(t *testing.T) {
	cases := map[string]struct {
		contentType, content string
		want                 error
	}{
		"wrong content type": {"image/png", wrapFile(sampleCells), ErrInvalidDiagram},
		"malformed":          {"drawio-xml", "<mxfile", ErrInvalidDiagram},
		"empty":              {"drawio-xml", wrapFile(""), ErrEmptyDiagram},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Given a submission that cannot be summarized and a provider that must not be called
			provider := llmmocks.NewMockProvider(t)
			input := testEvaluatorInput()
			input.Submission.ContentType, input.Submission.Content = c.contentType, c.content

			// When evaluating
			_, err := NewDiagramEvaluator(provider, fakeLanguage{code: "en"}).Evaluate(context.Background(), input)

			// Then it fails with the matching sentinel
			assert.ErrorIs(t, err, c.want)
		})
	}
}

func TestDiagramEvaluator_diagramErrorsCarryAUserFacingReason(t *testing.T) {
	// Given the two diagram sentinels
	// When asking for the reason the attempt is stored with
	// Then each is short and distinct
	var invalid, empty interface{ FailureReason() string }
	require.ErrorAs(t, ErrInvalidDiagram, &invalid)
	require.ErrorAs(t, ErrEmptyDiagram, &empty)
	assert.Equal(t, "The diagram could not be read.", invalid.FailureReason())
	assert.Equal(t, "The diagram is empty.", empty.FailureReason())
}
