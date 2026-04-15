package study_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fsantaniello/athena_school/internal/platform/llm"
	"github.com/fsantaniello/athena_school/internal/study"
	"github.com/fsantaniello/athena_school/internal/study/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// setupMockForOneRound registers three Chat expectations on the mock:
// explanation, question, and evaluation. Returns each canned response.
func setupMockForOneRound(m *mocks.LLMProvider, explanation, question, evaluation string) {
	explainMatcher := mock.MatchedBy(func(req llm.ChatRequest) bool {
		return len(req.Messages) >= 1 &&
			req.Messages[0].Role == "user" &&
			strings.Contains(req.Messages[0].Content, "Explain the topic")
	})
	questionMatcher := mock.MatchedBy(func(req llm.ChatRequest) bool {
		return len(req.Messages) >= 2 &&
			req.Messages[len(req.Messages)-1].Role == "user" &&
			strings.Contains(req.Messages[len(req.Messages)-1].Content, "ask the user one specific question")
	})
	evalMatcher := mock.MatchedBy(func(req llm.ChatRequest) bool {
		return len(req.Messages) >= 3 &&
			req.Messages[len(req.Messages)-1].Role == "user" &&
			strings.Contains(req.Messages[len(req.Messages)-1].Content, "Evaluate the answer")
	})

	m.On("Chat", context.Background(), explainMatcher).Return(llm.ChatResponse{Content: explanation}, nil).Once()
	m.On("Chat", context.Background(), questionMatcher).Return(llm.ChatResponse{Content: question}, nil).Once()
	m.On("Chat", context.Background(), evalMatcher).Return(llm.ChatResponse{Content: evaluation}, nil).Once()
}

func Test_GivenTopicAndSubtopic_WhenSessionRun_ThenHeaderContainsBoth(t *testing.T) {
	// Given: a session with topic and subtopic; mock set up for one round
	mockProvider := mocks.NewLLMProvider(t)
	setupMockForOneRound(mockProvider, "explanation text", "What is LRU?", "good answer")

	input := strings.NewReader("my answer\n\nn\n")
	var out strings.Builder
	session := study.NewSession("system-design", "caching", "llama3", mockProvider, input, &out)

	// When: running the session
	err := session.Run(context.Background())

	// Then: the output header contains both topic and subtopic
	require.NoError(t, err)
	assert.Contains(t, out.String(), "system-design")
	assert.Contains(t, out.String(), "caching")
}

func Test_GivenTopicOnly_WhenSessionRun_ThenHeaderContainsTopicWithoutSubtopicSeparator(t *testing.T) {
	// Given: a session with only a topic and no subtopic
	mockProvider := mocks.NewLLMProvider(t)
	setupMockForOneRound(mockProvider, "explanation", "question?", "evaluation")

	input := strings.NewReader("answer\n\nn\n")
	var out strings.Builder
	session := study.NewSession("concurrency", "", "llama3", mockProvider, input, &out)

	// When: running the session
	err := session.Run(context.Background())

	// Then: output contains the topic but not the subtopic separator
	require.NoError(t, err)
	assert.Contains(t, out.String(), "concurrency")
	assert.NotContains(t, out.String(), "›")
}

func Test_GivenValidTopic_WhenSessionRun_ThenLLMCalledThreeTimes(t *testing.T) {
	// Given: a mock set up for exactly three Chat calls
	mockProvider := mocks.NewLLMProvider(t)
	setupMockForOneRound(mockProvider, "explanation", "question?", "evaluation")

	input := strings.NewReader("answer\n\nn\n")
	var out strings.Builder
	session := study.NewSession("caching", "", "llama3", mockProvider, input, &out)

	// When: running the session
	err := session.Run(context.Background())

	// Then: Chat was called exactly three times
	require.NoError(t, err)
	mockProvider.AssertNumberOfCalls(t, "Chat", 3)
}

func Test_GivenValidTopic_WhenSessionRun_ThenFirstCallUsesExplanationPrompt(t *testing.T) {
	// Given: a mock that captures the first Chat call
	var capturedFirstReq llm.ChatRequest
	mockProvider := mocks.NewLLMProvider(t)

	// First call has exactly 1 message (the explanation prompt)
	firstCallMatcher := mock.MatchedBy(func(req llm.ChatRequest) bool {
		if len(req.Messages) == 1 {
			capturedFirstReq = req
			return true
		}
		return false
	})
	// Subsequent calls have more than 1 message (history has been accumulated)
	otherMatcher := mock.MatchedBy(func(req llm.ChatRequest) bool {
		return len(req.Messages) > 1
	})

	mockProvider.On("Chat", context.Background(), firstCallMatcher).Return(llm.ChatResponse{Content: "Explanation content"}, nil).Once()
	mockProvider.On("Chat", context.Background(), otherMatcher).Return(llm.ChatResponse{Content: "question or eval"}, nil).Times(2)

	input := strings.NewReader("answer\n\nn\n")
	var out strings.Builder
	session := study.NewSession("caching", "", "llama3", mockProvider, input, &out)

	// When: running the session
	err := session.Run(context.Background())

	// Then: the first Chat call contains the explanation prompt with role "user"
	require.NoError(t, err)
	require.NotEmpty(t, capturedFirstReq.Messages)
	assert.Equal(t, "user", capturedFirstReq.Messages[0].Role)
	assert.Contains(t, capturedFirstReq.Messages[0].Content, "Explain the topic")
}

func Test_GivenValidTopic_WhenSessionRun_ThenSecondCallIncludesExplanationInHistory(t *testing.T) {
	// Given: a mock where the first call returns "Explanation content";
	// the second call should receive the assistant reply in its history.
	mockProvider := mocks.NewLLMProvider(t)

	var capturedSecondReq llm.ChatRequest
	callCount := 0

	mockProvider.On("Chat", context.Background(), mock.MatchedBy(func(req llm.ChatRequest) bool {
		callCount++
		if callCount == 2 {
			capturedSecondReq = req
		}
		return true
	})).Return(llm.ChatResponse{Content: "Explanation content"}, nil).Once()

	mockProvider.On("Chat", context.Background(), mock.MatchedBy(func(req llm.ChatRequest) bool {
		callCount++
		if callCount == 2 {
			capturedSecondReq = req
		}
		return true
	})).Return(llm.ChatResponse{Content: "What is LRU?"}, nil).Once()

	mockProvider.On("Chat", context.Background(), mock.MatchedBy(func(_ llm.ChatRequest) bool { return true })).
		Return(llm.ChatResponse{Content: "evaluation text"}, nil).Once()

	input := strings.NewReader("my answer\n\nn\n")
	var out strings.Builder
	session := study.NewSession("caching", "", "llama3", mockProvider, input, &out)

	// When: running the session
	err := session.Run(context.Background())

	// Then: the second Chat call's history contains an assistant message with the explanation
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(capturedSecondReq.Messages), 2)
	hasAssistantReply := false
	for _, msg := range capturedSecondReq.Messages {
		if msg.Role == "assistant" && strings.Contains(msg.Content, "Explanation content") {
			hasAssistantReply = true
			break
		}
	}
	assert.True(t, hasAssistantReply, "second call should include the assistant's explanation in history")
}

func Test_GivenValidTopic_WhenSessionRun_ThenQuestionIsPrinted(t *testing.T) {
	// Given: a mock where the second call returns a specific question text
	mockProvider := mocks.NewLLMProvider(t)
	setupMockForOneRound(mockProvider, "Explanation text", "What are cache invalidation strategies?", "good job")

	input := strings.NewReader("my answer here\n\nn\n")
	var out strings.Builder
	session := study.NewSession("caching", "", "llama3", mockProvider, input, &out)

	// When: running the session
	err := session.Run(context.Background())

	// Then: the output contains the question marker and the question text
	require.NoError(t, err)
	assert.Contains(t, out.String(), "❓ Question:")
	assert.Contains(t, out.String(), "What are cache invalidation strategies?")
}

func Test_GivenValidTopic_WhenSessionRun_ThenThirdCallIncludesUserAnswer(t *testing.T) {
	// Given: a mock; the user will type a specific answer
	mockProvider := mocks.NewLLMProvider(t)

	var capturedThirdReq llm.ChatRequest
	callCount := 0

	captureOnThird := mock.MatchedBy(func(req llm.ChatRequest) bool {
		callCount++
		if callCount == 3 {
			capturedThirdReq = req
		}
		return true
	})

	mockProvider.On("Chat", context.Background(), captureOnThird).
		Return(llm.ChatResponse{Content: "response"}, nil).Times(3)

	input := strings.NewReader("LRU evicts the least recently used item\n\nn\n")
	var out strings.Builder
	session := study.NewSession("caching", "", "llama3", mockProvider, input, &out)

	// When: running the session
	err := session.Run(context.Background())

	// Then: the third Chat call's messages contain the user's answer
	require.NoError(t, err)
	require.NotEmpty(t, capturedThirdReq.Messages)
	lastMsg := capturedThirdReq.Messages[len(capturedThirdReq.Messages)-1]
	assert.Contains(t, lastMsg.Content, "LRU evicts the least recently used item")
}

func Test_GivenValidTopic_WhenSessionRun_ThenEvaluationIsPrinted(t *testing.T) {
	// Given: a mock where the third call returns evaluation text
	mockProvider := mocks.NewLLMProvider(t)
	setupMockForOneRound(mockProvider, "Explanation", "Question?", "✅ Strengths:\n- Good point\n⭐ Score: 8/10")

	input := strings.NewReader("answer\n\nn\n")
	var out strings.Builder
	session := study.NewSession("caching", "", "llama3", mockProvider, input, &out)

	// When: running the session
	err := session.Run(context.Background())

	// Then: the output contains the evaluation content
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Strengths")
	assert.Contains(t, out.String(), "Score")
}

func Test_GivenLLMError_WhenSessionRun_ThenRunReturnsError(t *testing.T) {
	// Given: a mock that returns an error on the first Chat call
	mockProvider := mocks.NewLLMProvider(t)
	mockProvider.On("Chat", context.Background(), mock.MatchedBy(func(_ llm.ChatRequest) bool { return true })).
		Return(llm.ChatResponse{}, errors.New("LLM unavailable")).Once()

	input := strings.NewReader("")
	var out strings.Builder
	session := study.NewSession("caching", "", "llama3", mockProvider, input, &out)

	// When: running the session
	err := session.Run(context.Background())

	// Then: Run returns an error containing the LLM error message
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLM unavailable")
}

func Test_GivenUserTypesExit_WhenReadingAnswer_ThenRunEndsWithoutError(t *testing.T) {
	// Given: a mock set up for explanation and question (not evaluation, since user exits);
	// user types "exit" as their answer
	mockProvider := mocks.NewLLMProvider(t)

	explainMatcher := mock.MatchedBy(func(req llm.ChatRequest) bool {
		return strings.Contains(req.Messages[0].Content, "Explain the topic")
	})
	questionMatcher := mock.MatchedBy(func(req llm.ChatRequest) bool {
		last := req.Messages[len(req.Messages)-1]
		return last.Role == "user" && strings.Contains(last.Content, "ask the user one specific question")
	})

	mockProvider.On("Chat", context.Background(), explainMatcher).Return(llm.ChatResponse{Content: "explanation"}, nil).Once()
	mockProvider.On("Chat", context.Background(), questionMatcher).Return(llm.ChatResponse{Content: "question?"}, nil).Once()

	input := strings.NewReader("exit\n")
	var out strings.Builder
	session := study.NewSession("caching", "", "llama3", mockProvider, input, &out)

	// When: running the session
	err := session.Run(context.Background())

	// Then: Run completes gracefully without error; evaluation is not called
	require.NoError(t, err)
	mockProvider.AssertNumberOfCalls(t, "Chat", 2)
}

func Test_GivenUserTypesDot_WhenReadingAnswer_ThenSessionContinuesToEvaluation(t *testing.T) {
	// Given: a mock set up for three calls; user submits with "."
	mockProvider := mocks.NewLLMProvider(t)
	setupMockForOneRound(mockProvider, "explanation", "question?", "evaluation")

	input := strings.NewReader("partial answer\n.\nn\n")
	var out strings.Builder
	session := study.NewSession("caching", "", "llama3", mockProvider, input, &out)

	// When: running the session
	err := session.Run(context.Background())

	// Then: all three Chat calls were made (evaluation happened)
	require.NoError(t, err)
	mockProvider.AssertNumberOfCalls(t, "Chat", 3)
}

func Test_GivenUserChoosesContinue_WhenRun_ThenRunsTwoFullRounds(t *testing.T) {
	// Given: a mock set up for six Chat calls (2 rounds × 3 calls each);
	// user types "y" after the first round then "n" after the second
	mockProvider := mocks.NewLLMProvider(t)
	setupMockForOneRound(mockProvider, "explanation1", "question1?", "evaluation1")
	setupMockForOneRound(mockProvider, "explanation2", "question2?", "evaluation2")

	input := strings.NewReader("answer1\n\ny\nanswer2\n\nn\n")
	var out strings.Builder
	session := study.NewSession("caching", "", "llama3", mockProvider, input, &out)

	// When: running the session
	err := session.Run(context.Background())

	// Then: Chat was called six times (two full rounds)
	require.NoError(t, err)
	mockProvider.AssertNumberOfCalls(t, "Chat", 6)
}

func Test_GivenUserChoosesNotToContinue_WhenRun_ThenExitsAfterOneRound(t *testing.T) {
	// Given: a mock set up for three calls; user types "n" at the continue prompt
	mockProvider := mocks.NewLLMProvider(t)
	setupMockForOneRound(mockProvider, "explanation", "question?", "evaluation")

	input := strings.NewReader("answer\n\nn\n")
	var out strings.Builder
	session := study.NewSession("caching", "", "llama3", mockProvider, input, &out)

	// When: running the session
	err := session.Run(context.Background())

	// Then: Chat was called exactly three times and session exited cleanly
	require.NoError(t, err)
	mockProvider.AssertNumberOfCalls(t, "Chat", 3)
}
