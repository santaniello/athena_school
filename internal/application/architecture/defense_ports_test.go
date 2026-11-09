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

func testDefenseSubmission() domainexercise.Submission {
	return domainexercise.Submission{ContentType: "drawio-xml", Content: wrapFile(sampleCells)}
}

func testSummary(t *testing.T) DiagramSummary {
	t.Helper()
	summary, err := SummarizeDiagram(testDefenseSubmission().Content)
	require.NoError(t, err)
	return summary
}

func TestDefenseQuestionGenerator_GenerateQuestion_asksTheModelOnceAndReturnsTheTrimmedQuestion(t *testing.T) {
	// Given a provider that answers and a Portuguese assistant
	provider := llmmocks.NewMockProvider(t)
	ctx := context.Background()
	input := testQuestionInput(1, 1, nil)
	input.Submission = testDefenseSubmission()
	provider.EXPECT().Chat(ctx, domainllm.ChatRequest{
		Task: domainllm.TaskArchitectureLab, Messages: buildQuestionPrompt(input, testSummary(t), "pt"),
	}).Return(domainllm.ChatResponse{Content: "  E se o cache cair?\n"}, nil).Once()

	// When generating the question
	question, err := NewDefenseQuestionGenerator(provider, fakeLanguage{code: "pt"}).GenerateQuestion(ctx, input)

	// Then the model's trimmed text is returned
	require.NoError(t, err)
	assert.Equal(t, "E se o cache cair?", question)
}

func TestDefenseQuestionGenerator_GenerateQuestion_failsBeforeTheModelWhenTheDiagramCannotBeRead(t *testing.T) {
	// Given a submission that is not a draw.io diagram
	provider := llmmocks.NewMockProvider(t)
	input := testQuestionInput(1, 1, nil)
	input.Submission = domainexercise.Submission{ContentType: "image/png", Content: "x"}

	// When generating the question
	_, err := NewDefenseQuestionGenerator(provider, fakeLanguage{code: "en"}).GenerateQuestion(context.Background(), input)

	// Then it is an invalid diagram and the model was not called
	assert.ErrorIs(t, err, ErrInvalidDiagram)
}

func TestDefenseQuestionGenerator_GenerateQuestion_reportsAnUnreadableDiagram(t *testing.T) {
	// Given a submission with the right type but broken XML
	provider := llmmocks.NewMockProvider(t)
	input := testQuestionInput(1, 1, nil)
	input.Submission = domainexercise.Submission{ContentType: "drawio-xml", Content: "<mxfile"}

	// When generating the question
	_, err := NewDefenseQuestionGenerator(provider, fakeLanguage{code: "en"}).GenerateQuestion(context.Background(), input)

	// Then it is an invalid diagram
	assert.ErrorIs(t, err, ErrInvalidDiagram)
}

func TestDefenseQuestionGenerator_GenerateQuestion_returnsProfileAndProviderErrors(t *testing.T) {
	profileErr := errors.New("no profile")
	providerErr := errors.New("model down")
	input := testQuestionInput(1, 1, nil)
	input.Submission = testDefenseSubmission()

	// Given a profile that cannot be read
	_, err := NewDefenseQuestionGenerator(llmmocks.NewMockProvider(t), fakeLanguage{err: profileErr}).GenerateQuestion(context.Background(), input)

	// Then the error is returned
	assert.ErrorIs(t, err, profileErr)

	// Given a provider that fails
	provider := llmmocks.NewMockProvider(t)
	provider.EXPECT().Chat(context.Background(), domainllm.ChatRequest{
		Task: domainllm.TaskArchitectureLab, Messages: buildQuestionPrompt(input, testSummary(t), "en"),
	}).Return(domainllm.ChatResponse{}, providerErr).Once()
	_, err = NewDefenseQuestionGenerator(provider, fakeLanguage{code: "en"}).GenerateQuestion(context.Background(), input)

	// Then that error is returned, with no retry
	assert.ErrorIs(t, err, providerErr)
}

const validUnderstanding = `{
  "summary": "Knows resilience, not scaling.",
  "strengths": ["Fallback"], "gaps": ["Scaling"], "recommendations": ["Study sharding"],
  "criteria": [
    {"criterionKey": "resilience", "level": 3, "justification": "Fallback named.", "evidence": "We fall back", "insufficientEvidence": false},
    {"criterionKey": "scalability", "level": 0, "justification": "Did not know.", "insufficientEvidence": false}
  ]
}`

func testUnderstandingInputWithDiagram() domainexercise.UnderstandingInput {
	input := testUnderstandingInput()
	input.Submission = testDefenseSubmission()
	return input
}

func TestUnderstandingEvaluator_EvaluateUnderstanding_mapsTheModelAnswerToEvaluatorOutput(t *testing.T) {
	// Given a provider that answers with a valid evaluation
	provider := llmmocks.NewMockProvider(t)
	ctx := context.Background()
	input := testUnderstandingInputWithDiagram()
	provider.EXPECT().Chat(ctx, domainllm.ChatRequest{
		Task:     domainllm.TaskArchitectureLab,
		Messages: []domainllm.Message{{Role: "user", Content: BuildUnderstandingPrompt(input, testSummary(t), "pt")}},
	}).Return(domainllm.ChatResponse{Content: validUnderstanding, Model: "vendor/model-x"}, nil).Once()

	// When evaluating the understanding
	output, err := NewUnderstandingEvaluator(provider, fakeLanguage{code: "pt"}).EvaluateUnderstanding(ctx, input)

	// Then criterion keys map to ids and the exact model is recorded
	require.NoError(t, err)
	assert.Equal(t, domainexercise.EvaluatorOutput{
		Model: "vendor/model-x", Summary: "Knows resilience, not scaling.",
		Strengths: []string{"Fallback"}, Gaps: []string{"Scaling"}, Recommendations: []string{"Study sharding"},
		Results: []domainexercise.CriterionResult{
			{CriterionID: "c-res", Level: 3, Justification: "Fallback named.", Evidence: "We fall back"},
			{CriterionID: "c-scal", Level: 0, Justification: "Did not know."},
		},
	}, output)
}

func TestUnderstandingEvaluator_EvaluateUnderstanding_anInvalidAnswerIsAnInvalidEvaluation(t *testing.T) {
	for name, content := range map[string]string{
		"not json":          "I think they did well.",
		"unknown criterion": `{"summary":"s","strengths":[],"gaps":[],"recommendations":[],"criteria":[{"criterionKey":"speed","level":1,"justification":"j","insufficientEvidence":false}]}`,
		"no criteria":       `{"summary":"s","strengths":[],"gaps":[],"recommendations":[],"criteria":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			// Given a model answer the engine cannot use
			provider := llmmocks.NewMockProvider(t)
			input := testUnderstandingInputWithDiagram()
			provider.EXPECT().Chat(context.Background(), domainllm.ChatRequest{
				Task:     domainllm.TaskArchitectureLab,
				Messages: []domainllm.Message{{Role: "user", Content: BuildUnderstandingPrompt(input, testSummary(t), "en")}},
			}).Return(domainllm.ChatResponse{Content: content}, nil).Once()

			// When evaluating
			_, err := NewUnderstandingEvaluator(provider, fakeLanguage{code: "en"}).EvaluateUnderstanding(context.Background(), input)

			// Then it is an invalid evaluation
			assert.ErrorIs(t, err, domainexercise.ErrInvalidEvaluation)
		})
	}
}

func TestUnderstandingEvaluator_EvaluateUnderstanding_failsBeforeTheModelWhenTheDiagramCannotBeRead(t *testing.T) {
	// Given a submission that is not a draw.io diagram
	input := testUnderstandingInput()
	input.Submission = domainexercise.Submission{ContentType: "image/png", Content: "x"}

	// When evaluating
	_, err := NewUnderstandingEvaluator(llmmocks.NewMockProvider(t), fakeLanguage{code: "en"}).EvaluateUnderstanding(context.Background(), input)

	// Then it is an invalid diagram and the model was not called
	assert.ErrorIs(t, err, ErrInvalidDiagram)
}

func TestUnderstandingEvaluator_EvaluateUnderstanding_returnsProfileAndProviderErrors(t *testing.T) {
	profileErr := errors.New("no profile")
	providerErr := errors.New("model down")
	input := testUnderstandingInputWithDiagram()

	// Given a profile that cannot be read
	_, err := NewUnderstandingEvaluator(llmmocks.NewMockProvider(t), fakeLanguage{err: profileErr}).EvaluateUnderstanding(context.Background(), input)

	// Then the error is returned
	assert.ErrorIs(t, err, profileErr)

	// Given a provider that fails
	provider := llmmocks.NewMockProvider(t)
	provider.EXPECT().Chat(context.Background(), domainllm.ChatRequest{
		Task:     domainllm.TaskArchitectureLab,
		Messages: []domainllm.Message{{Role: "user", Content: BuildUnderstandingPrompt(input, testSummary(t), "en")}},
	}).Return(domainllm.ChatResponse{}, providerErr).Once()
	_, err = NewUnderstandingEvaluator(provider, fakeLanguage{code: "en"}).EvaluateUnderstanding(context.Background(), input)

	// Then that error is returned
	assert.ErrorIs(t, err, providerErr)
}
