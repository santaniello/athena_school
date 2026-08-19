package desktop

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	applicationknowledge "github.com/santaniello/athena/internal/application/knowledge"
	txmocks "github.com/santaniello/athena/internal/application/knowledge/mocks"
	domainconfig "github.com/santaniello/athena/internal/domain/config"
	configmocks "github.com/santaniello/athena/internal/domain/config/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestApp_ExtractKnowledge_returnsFullCandidateAndTruncationState(t *testing.T) {
	// Given a knowledge service returning a full candidate from the LLM
	ctx := context.Background()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	configs := configmocks.NewMockStore(t)
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{Topic: "Go"}, nil).Once()
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{{Role: domainstudy.RoleUser, Content: "Explain channels"}}, nil).Once()
	configs.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 8}, nil).Once()
	llm.EXPECT().Chat(ctx, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		return req.SessionID == "session-1" &&
			req.Task == domainllm.TaskKnowledgeExtraction &&
			len(req.Messages) == 1 &&
			req.Messages[0].Role == "system" &&
			strings.Contains(req.Messages[0].Content, "User: Explain channels")
	})).Return(domainllm.ChatResponse{Content: `{"items":[{"concept":"Channels","definition":"Typed conduits.","properties":["typed"],"trade_offs":["coordination"],"related_concepts":["goroutines"]}]}`}, nil).Once()
	service := applicationknowledge.NewService(knowledgemocks.NewMockRepository(t), sessions, messages, llm, configs, knowledgemocks.NewMockChunkRepository(t), nil)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil)
	app.Startup(ctx)

	// When extracting through the desktop adapter
	result, err := app.ExtractKnowledge("session-1", false)

	// Then the wrapper and every candidate field survive the translation
	require.NoError(t, err)
	assert.False(t, result.Truncated)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "Channels", result.Items[0].Concept)
	assert.Equal(t, []string{"typed"}, result.Items[0].Properties)
	assert.Equal(t, []string{"coordination"}, result.Items[0].TradeOffs)
	assert.Equal(t, []string{"goroutines"}, result.Items[0].RelatedConcepts)
}

func TestApp_ExtractKnowledge_returnsEmptyResultForMalformedLLMResponse(t *testing.T) {
	// Given a service whose LLM returns malformed JSON
	ctx := context.Background()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	configs := configmocks.NewMockStore(t)
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{Topic: "Go"}, nil).Once()
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{{Role: domainstudy.RoleUser, Content: "Explain channels"}}, nil).Once()
	configs.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 8}, nil).Once()
	llm.EXPECT().Chat(ctx, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		return req.SessionID == "session-1" &&
			req.Task == domainllm.TaskKnowledgeExtraction &&
			len(req.Messages) == 1 &&
			req.Messages[0].Role == "system" &&
			strings.Contains(req.Messages[0].Content, "User: Explain channels")
	})).Return(domainllm.ChatResponse{Content: "not json"}, nil).Once()
	service := applicationknowledge.NewService(knowledgemocks.NewMockRepository(t), sessions, messages, llm, configs, knowledgemocks.NewMockChunkRepository(t), nil)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil)
	app.Startup(ctx)

	// When extracting through the desktop adapter
	result, err := app.ExtractKnowledge("session-1", false)

	// Then the malformed response is swallowed as an empty result
	require.NoError(t, err)
	assert.Empty(t, result.Items)
	assert.False(t, result.Truncated)
}

func TestApp_SaveExtractedKnowledge_preservesFullInputAndReturnsSavedIndices(t *testing.T) {
	// Given a knowledge service backed by a repository
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "Channels" && assert.ObjectsAreEqual([]string{"typed"}, item.Properties)
	})).Return(nil).Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil)
	app.Startup(ctx)

	// When saving a full desktop candidate
	result := app.SaveExtractedKnowledge([]KnowledgeItemInput{{
		Topic: "Go", Concept: "Channels", Definition: "Typed conduits.", Properties: []string{"typed"},
	}})

	// Then it is persisted and its exact input index is returned
	assert.Equal(t, []int{0}, result.SavedIndices)
	assert.Empty(t, result.Error)
}

func TestApp_SaveExtractedKnowledge_returnsExactIndicesAlongsidePartialFailure(t *testing.T) {
	// Given an invalid input followed by one save and one repository failure
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "saved"
	})).Return(nil).Once()
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "failed"
	})).Return(assert.AnError).Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil)
	app.Startup(ctx)

	// When saving through the desktop adapter
	result := app.SaveExtractedKnowledge([]KnowledgeItemInput{
		{Topic: "Go", Concept: "invalid", Definition: ""},
		{Topic: "Go", Concept: "saved", Definition: "persisted"},
		{Topic: "Go", Concept: "failed", Definition: "not persisted"},
	})

	// Then the resolved result carries both the precise success and the failure
	assert.Equal(t, []int{1}, result.SavedIndices)
	assert.Contains(t, result.Error, assert.AnError.Error())
}

func TestApp_SaveAndApproveExtractedKnowledge_persistsDirectlyAsApproved(t *testing.T) {
	// Given a knowledge service backed by a repository
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "Channels" && item.Status == domainknowledge.StatusApproved
	})).Return(nil).Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil)
	app.Startup(ctx)

	// When saving and approving a full desktop candidate
	result := app.SaveAndApproveExtractedKnowledge([]KnowledgeItemInput{{
		Topic: "Go", Concept: "Channels", Definition: "Typed conduits.",
	}})

	// Then it is persisted directly as approved and its exact input index is returned
	assert.Equal(t, []int{0}, result.SavedIndices)
	assert.Empty(t, result.Error)
}

func TestApp_ListKnowledgeItems_returnsItemsForTopicAndStatus(t *testing.T) {
	// Given a repository with one matching item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().List(ctx, domainknowledge.Filter{Topic: "Go", Status: domainknowledge.StatusApproved}).
		Return([]domainknowledge.Item{{ID: "item-1", Topic: "Go", Concept: "Channels", Status: domainknowledge.StatusApproved}}, nil).Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil)
	app.Startup(ctx)

	// When listing through the desktop adapter
	results, err := app.ListKnowledgeItems("Go", domainknowledge.StatusApproved)

	// Then the item is returned as a KnowledgeItemResult
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Channels", results[0].Concept)
}

func TestApp_ListKnowledgeTopics_returnsTopics(t *testing.T) {
	// Given a repository with two topics
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().ListTopics(ctx).Return([]string{"Go", "Kubernetes"}, nil).Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil)
	app.Startup(ctx)

	// When listing topics through the desktop adapter
	topics, err := app.ListKnowledgeTopics()

	// Then they are returned as-is
	require.NoError(t, err)
	assert.Equal(t, []string{"Go", "Kubernetes"}, topics)
}

func TestApp_ApproveKnowledgeItem_returnsTheUpdatedItem(t *testing.T) {
	// Given a draft item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.", Status: domainknowledge.StatusDraft,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Status == domainknowledge.StatusApproved
	})).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	tx.EXPECT().WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), tx)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil)
	app.Startup(ctx)

	// When approving through the desktop adapter
	result, err := app.ApproveKnowledgeItem("item-1")

	// Then the returned item carries the new status
	require.NoError(t, err)
	assert.Equal(t, domainknowledge.StatusApproved, result.Status)
}

func TestApp_DeprecateKnowledgeItem_returnsTheUpdatedItem(t *testing.T) {
	// Given an approved item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.", Status: domainknowledge.StatusApproved,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Status == domainknowledge.StatusDeprecated
	})).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	tx.EXPECT().WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), tx)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil)
	app.Startup(ctx)

	// When deprecating through the desktop adapter
	result, err := app.DeprecateKnowledgeItem("item-1")

	// Then the returned item carries the new status
	require.NoError(t, err)
	assert.Equal(t, domainknowledge.StatusDeprecated, result.Status)
}

func TestApp_UpdateKnowledgeItem_persistsEditableFields_andReturnsTheUpdatedItem(t *testing.T) {
	// Given an existing item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Old", Definition: "Old def.", Status: domainknowledge.StatusApproved,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "New" && item.Status == domainknowledge.StatusApproved
	})).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	tx.EXPECT().WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), tx)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil)
	app.Startup(ctx)

	// When updating through the desktop adapter
	result, err := app.UpdateKnowledgeItem("item-1", KnowledgeItemInput{
		Topic: "Go", Concept: "New", Definition: "New def.",
	})

	// Then the edited fields persist
	require.NoError(t, err)
	assert.Equal(t, "New", result.Concept)
}

func TestApp_DeleteKnowledgeItem_deletesTheItemAndItsChunks(t *testing.T) {
	// Given an item with chunks
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil).Once()
	repository.EXPECT().Delete(ctx, "item-1").Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	tx.EXPECT().WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), chunks, tx)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil)
	app.Startup(ctx)

	// When deleting through the desktop adapter
	err := app.DeleteKnowledgeItem("item-1")

	// Then it succeeds
	require.NoError(t, err)
}
