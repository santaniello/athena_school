package knowledge

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainconfig "github.com/santaniello/athena/internal/domain/config"
	configmocks "github.com/santaniello/athena/internal/domain/config/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestExtractFromSession_returnsNoCandidatesWithoutCallingLLMWhenHistoryIsEmpty(t *testing.T) {
	// Given a study session with no messages
	ctx := context.Background()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{ID: "session-1", Topic: "Go"}, nil).Once()
	messages.EXPECT().ListBySession(ctx, "session-1").Return(nil, nil).Once()
	service := NewService(
		knowledgemocks.NewMockRepository(t),
		sessions,
		messages,
		llmmocks.NewMockProvider(t),
		configmocks.NewMockStore(t),
		knowledgemocks.NewMockChunkRepository(t),
		nil,
		nil,
		nil,
		domainknowledge.RetrievalThresholds{},
		nil,
	)

	// When extracting knowledge
	batch, truncated, err := service.ExtractFromSession(ctx, "session-1", false)

	// Then no candidates or truncation are returned, and the LLM mock receives no call
	require.NoError(t, err)
	require.Empty(t, batch.ID)
	require.Nil(t, batch.Items)
	require.False(t, truncated)
}

func TestExtractFromSession_returnsValidatedServerStampedCandidates(t *testing.T) {
	// Given a session transcript and a fenced LLM response containing one valid
	// and one invalid candidate
	ctx := context.Background()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	configs := configmocks.NewMockStore(t)
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{ID: "session-1", Topic: "Distributed systems"}, nil).Once()
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-user", Role: domainstudy.RoleUser, Content: " Explain CAP. "},
		{ID: "message-assistant", Role: domainstudy.RoleAssistant, Content: "CAP describes trade-offs."},
	}, nil).Once()
	configs.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 8}, nil).Once()
	llm.EXPECT().Chat(ctx, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		if req.SessionID != "session-1" || req.Task != domainllm.TaskKnowledgeExtraction || len(req.Messages) != 1 {
			return false
		}
		prompt := req.Messages[0]
		return prompt.Role == "system" &&
			strings.Contains(prompt.Content, "at most 8 items") &&
			strings.Contains(prompt.Content, `{"items":[`) &&
			strings.Contains(prompt.Content, "[message:message-user] User:\nExplain CAP.") &&
			strings.Contains(prompt.Content, "[message:message-assistant] Assistant:\nCAP describes trade-offs.")
	})).Return(domainllm.ChatResponse{Content: "```json\n" + `{"items":[{"topic":"hostile","concept":" CAP theorem ","definition":" A self-contained definition. ","properties":[" partition tolerance "," "],"trade_offs":[" consistency vs availability "],"related_concepts":[" PACELC "],"evidence":[{"message_id":"message-assistant","quote":"CAP describes trade-offs."}]},{"concept":"invalid"}]}` + "\n```"}, nil).Once()
	service := NewService(knowledgemocks.NewMockRepository(t), sessions, messages, llm, configs, knowledgemocks.NewMockChunkRepository(t), nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil)

	// When extracting knowledge
	batch, truncated, err := service.ExtractFromSession(ctx, "session-1", false)

	// Then only the valid item is returned with normalized client fields and
	// server-owned fields independent of the LLM payload
	require.NoError(t, err)
	assert.False(t, truncated)
	assert.NotEmpty(t, batch.ID)
	items := batch.Items
	require.Len(t, items, 1)
	item := items[0]
	assert.NotEmpty(t, item.ID)
	assert.Equal(t, "Distributed systems", item.Topic)
	assert.Equal(t, "CAP theorem", item.Concept)
	assert.Equal(t, "A self-contained definition.", item.Definition)
	assert.Equal(t, []string{"partition tolerance"}, item.Properties)
	assert.Equal(t, []string{"consistency vs availability"}, item.TradeOffs)
	assert.Equal(t, []string{"PACELC"}, item.RelatedConcepts)
	assert.Equal(t, domainknowledge.SourceAthena, item.Source)
	assert.Equal(t, domainknowledge.StatusDraft, item.Status)
	assert.WithinDuration(t, time.Now().UTC(), item.CreatedAt, time.Second)
	assert.Equal(t, item.CreatedAt, item.UpdatedAt)
	receipt, found := service.receipts.Get(batch.ID, item.ID)
	require.True(t, found)
	assert.Equal(t, "session-1", receipt.SessionID)
	assert.Equal(t, "Distributed systems", receipt.SourceLabel)
	assert.Equal(t, []domainknowledge.EvidenceRef{{MessageID: "message-assistant", Quote: "CAP describes trade-offs."}}, receipt.EvidenceRefs)
}

func TestExtractFromSession_requiresConfirmationBeforeCallingLLMForTruncatedTranscript(t *testing.T) {
	// Given a transcript whose two whole messages do not fit together
	ctx := context.Background()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	configs := configmocks.NewMockStore(t)
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{ID: "session-1", Topic: "Go"}, nil).Twice()
	history := []domainstudy.Message{
		{ID: "message-old", Role: domainstudy.RoleUser, Content: strings.Repeat("o", 13000)},
		{ID: "message-new", Role: domainstudy.RoleAssistant, Content: strings.Repeat("n", 13000)},
	}
	messages.EXPECT().ListBySession(ctx, "session-1").Return(history, nil).Twice()
	configs.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 8}, nil).Twice()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(ctx, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		prompt := req.Messages[0].Content
		return !strings.Contains(prompt, strings.Repeat("o", 100)) &&
			strings.Contains(prompt, strings.Repeat("n", 100))
	})).Return(domainllm.ChatResponse{Content: `{"items":[]}`}, nil).Once()
	service := NewService(knowledgemocks.NewMockRepository(t), sessions, messages, llm, configs, knowledgemocks.NewMockChunkRepository(t), nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil)

	// When extracting before and after the user confirms truncation
	firstBatch, firstTruncated, firstErr := service.ExtractFromSession(ctx, "session-1", false)
	secondBatch, secondTruncated, secondErr := service.ExtractFromSession(ctx, "session-1", true)

	// Then the first call returns only the confirmation signal, and the second
	// makes the sole LLM call using only the most recent whole message
	require.NoError(t, firstErr)
	assert.Nil(t, firstBatch.Items)
	assert.True(t, firstTruncated)
	require.NoError(t, secondErr)
	assert.Empty(t, secondBatch.Items)
	assert.True(t, secondTruncated)

	// And a rendered message exactly at the limit is not incorrectly truncated
	exactPrefix := "[message:message-exact] User:\n"
	exactContent := strings.Repeat("x", maxTranscriptChars-len([]rune(exactPrefix)))
	exactTranscript, exactMessages, exactTruncated := renderTranscript(
		[]domainstudy.Message{{ID: "message-exact", Role: domainstudy.RoleUser, Content: exactContent}},
		maxTranscriptChars,
	)
	assert.False(t, exactTruncated)
	assert.Len(t, []rune(exactTranscript), maxTranscriptChars)
	require.Len(t, exactMessages, 1)
}

func TestExtractFromSession_rejectsEvidenceFromMessageOutsideCappedTranscript(t *testing.T) {
	// Given a truncated Study Session whose oldest Message is not sent to the LLM
	ctx := context.Background()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	configs := configmocks.NewMockStore(t)
	llm := llmmocks.NewMockProvider(t)
	history := []domainstudy.Message{
		{ID: "message-outside", Role: domainstudy.RoleUser, Content: "outside quote " + strings.Repeat("o", 13000)},
		{ID: "message-included", Role: domainstudy.RoleAssistant, Content: "included quote " + strings.Repeat("n", 13000)},
	}
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{ID: "session-1", Topic: "Go"}, nil).Once()
	messages.EXPECT().ListBySession(ctx, "session-1").Return(history, nil).Once()
	configs.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 8}, nil).Once()
	llm.EXPECT().Chat(ctx, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		prompt := req.Messages[0].Content
		return !strings.Contains(prompt, "outside quote") && strings.Contains(prompt, "included quote")
	})).Return(domainllm.ChatResponse{Content: `{"items":[{"concept":"Outside","definition":"Not sent.","evidence":[{"message_id":"message-outside","quote":"outside quote"}]}]}`}, nil).Once()
	service := NewService(knowledgemocks.NewMockRepository(t), sessions, messages, llm, configs, knowledgemocks.NewMockChunkRepository(t), nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil)

	// When extracting after the user confirms transcript truncation
	batch, truncated, err := service.ExtractFromSession(ctx, "session-1", true)

	// Then the candidate cannot cite the real but excluded Message
	require.NoError(t, err)
	assert.True(t, truncated)
	assert.Empty(t, batch.Items)
	assert.Empty(t, batch.ID)
}

func TestExtractFromSession_returnsErrorWithoutCallingLLMWhenNoCompleteMessageFits(t *testing.T) {
	// Given a non-empty session whose newest message alone exceeds the transcript limit
	ctx := context.Background()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	configs := configmocks.NewMockStore(t)
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{ID: "session-1", Topic: "Go"}, nil).Twice()
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{{
		ID:      "message-1",
		Role:    domainstudy.RoleUser,
		Content: strings.Repeat("x", maxTranscriptChars),
	}}, nil).Twice()
	configs.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 8}, nil).Twice()
	service := NewService(
		knowledgemocks.NewMockRepository(t),
		sessions,
		messages,
		llmmocks.NewMockProvider(t),
		configs,
		knowledgemocks.NewMockChunkRepository(t),
		nil,
		nil,
		nil,
		domainknowledge.RetrievalThresholds{},
		nil,
	)

	// When extraction is invoked before and after truncation is confirmed
	firstBatch, firstTruncated, firstErr := service.ExtractFromSession(ctx, "session-1", false)
	batch, truncated, err := service.ExtractFromSession(ctx, "session-1", true)

	// Then confirmation is requested first, followed by an explicit error without an LLM call
	require.NoError(t, firstErr)
	assert.Nil(t, firstBatch.Items)
	assert.True(t, firstTruncated)
	assert.ErrorIs(t, err, ErrTranscriptTooLarge)
	assert.EqualError(t, err, "no complete transcript message fits within the extraction limit")
	assert.Nil(t, batch.Items)
	assert.True(t, truncated)
}

func TestExtractFromSession_returnsMalformedExtractionForUnparseablePayload(t *testing.T) {
	// Given a non-empty session and an unparseable LLM response
	ctx := context.Background()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	configs := configmocks.NewMockStore(t)
	llm := llmmocks.NewMockProvider(t)
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{Topic: "Go"}, nil).Once()
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{{ID: "message-1", Role: domainstudy.RoleUser, Content: "Explain channels"}}, nil).Once()
	configs.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 8}, nil).Once()
	llm.EXPECT().Chat(ctx, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		return req.SessionID == "session-1" &&
			req.Task == domainllm.TaskKnowledgeExtraction &&
			len(req.Messages) == 1 &&
			req.Messages[0].Role == "system" &&
			strings.Contains(req.Messages[0].Content, "[message:message-1] User:\nExplain channels")
	})).Return(domainllm.ChatResponse{Content: "not json"}, nil).Once()
	service := NewService(knowledgemocks.NewMockRepository(t), sessions, messages, llm, configs, knowledgemocks.NewMockChunkRepository(t), nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil)

	// When extracting knowledge
	batch, truncated, err := service.ExtractFromSession(ctx, "session-1", false)

	// Then the application reports malformed extraction without candidates
	assert.ErrorIs(t, err, ErrMalformedExtraction)
	assert.Nil(t, batch.Items)
	assert.Empty(t, batch.ID)
	assert.False(t, truncated)
}

func TestDiscardExtraction_removesEveryPendingReceiptInTheBatch(t *testing.T) {
	// Given a batch with two pending candidate receipts
	service := NewService(nil, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil)
	batchID := service.receipts.Create("session-1", "Go", []parsedCandidate{
		{Item: domainknowledge.Item{ID: "candidate-1"}},
		{Item: domainknowledge.Item{ID: "candidate-2"}},
	})

	// When discarding the batch
	service.DiscardExtraction(batchID)

	// Then neither candidate's receipt is available any longer
	_, firstFound := service.receipts.Get(batchID, "candidate-1")
	_, secondFound := service.receipts.Get(batchID, "candidate-2")
	assert.False(t, firstFound)
	assert.False(t, secondFound)
}
