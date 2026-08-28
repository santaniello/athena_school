package desktop

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
	"time"

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

// passingDesktopIndexGuard returns an IndexGuard mock that always allows
// the mutation, for desktop tests exercising SaveExtractedKnowledge/
// SaveAndApproveExtractedKnowledge (which now index each saved item).
func passingDesktopIndexGuard(t *testing.T) *txmocks.MockIndexGuard {
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(nil)
	guard.EXPECT().EndMutation()
	return guard
}

// expectDesktopSuccessfulIndexing wires llm/chunks/store/tx mocks so every
// call indexKnowledgeItem makes succeeds, `times` times over.
func expectDesktopSuccessfulIndexing(
	ctx context.Context,
	llm *llmmocks.MockProvider,
	chunks *knowledgemocks.MockChunkRepository,
	store *knowledgemocks.MockVectorStore,
	tx *txmocks.MockTransactor,
	times int,
) {
	llm.EXPECT().Embeddings(ctx, mock.MatchedBy(func(req domainllm.EmbeddingRequest) bool { return req.Input != "" })).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Times(times)
	chunks.EXPECT().DeleteByItemID(ctx, mock.MatchedBy(func(id string) bool { return id != "" })).
		Return(nil, nil).Times(times)
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool { return len(cs) == 1 })).
		Return(nil).Times(times)
	store.EXPECT().Remove(mock.Anything, []string(nil)).Return(nil).Times(times)
	store.EXPECT().Add(mock.Anything, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool { return len(cs) == 1 })).
		Return(nil).Times(times)
	tx.EXPECT().WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
}

func TestApp_ExtractKnowledge_returnsFullCandidateAndTruncationState(t *testing.T) {
	// Given a knowledge service returning a full candidate from the LLM
	ctx := context.Background()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	configs := configmocks.NewMockStore(t)
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{ID: "session-1", Topic: "Go"}, nil).Once()
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{{ID: "message-1", Role: domainstudy.RoleUser, Content: "Explain channels"}}, nil).Once()
	configs.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 8}, nil).Once()
	llm.EXPECT().Chat(ctx, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		return req.SessionID == "session-1" &&
			req.Task == domainllm.TaskKnowledgeExtraction &&
			len(req.Messages) == 1 &&
			req.Messages[0].Role == "system" &&
			strings.Contains(req.Messages[0].Content, "[message:message-1] User:\nExplain channels")
	})).Return(domainllm.ChatResponse{Content: `{"items":[{"concept":"Channels","definition":"Typed conduits.","properties":["typed"],"trade_offs":["coordination"],"related_concepts":["goroutines"],"evidence":[{"message_id":"message-1","quote":"Explain channels"}]}]}`}, nil).Once()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().FindByNormalizedConcept(ctx, "Go", "channels").Return(nil, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(0).Once()
	service := applicationknowledge.NewService(repository, sessions, messages, llm, configs, knowledgemocks.NewMockChunkRepository(t), nil, store, nil, domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)

	// When extracting through the desktop adapter
	result, err := app.ExtractKnowledge("session-1", false)

	// Then the wrapper and every candidate field survive the translation
	require.NoError(t, err)
	assert.False(t, result.Truncated)
	assert.NotEmpty(t, result.BatchID)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "Channels", result.Items[0].Concept)
	assert.Equal(t, []string{"typed"}, result.Items[0].Properties)
	assert.Equal(t, []string{"coordination"}, result.Items[0].TradeOffs)
	assert.Equal(t, []string{"goroutines"}, result.Items[0].RelatedConcepts)
	assert.Empty(t, result.Items[0].Duplicates)
	assert.False(t, result.Items[0].SemanticCheckUnavailable)
}

func TestApp_ExtractKnowledge_returnsEmptyResultForMalformedLLMResponse(t *testing.T) {
	// Given a service whose LLM returns malformed JSON
	ctx := context.Background()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	configs := configmocks.NewMockStore(t)
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{ID: "session-1", Topic: "Go"}, nil).Once()
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{{ID: "message-1", Role: domainstudy.RoleUser, Content: "Explain channels"}}, nil).Once()
	configs.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 8}, nil).Once()
	llm.EXPECT().Chat(ctx, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		return req.SessionID == "session-1" &&
			req.Task == domainllm.TaskKnowledgeExtraction &&
			len(req.Messages) == 1 &&
			req.Messages[0].Role == "system" &&
			strings.Contains(req.Messages[0].Content, "[message:message-1] User:\nExplain channels")
	})).Return(domainllm.ChatResponse{Content: "not json"}, nil).Once()
	service := applicationknowledge.NewService(knowledgemocks.NewMockRepository(t), sessions, messages, llm, configs, knowledgemocks.NewMockChunkRepository(t), nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)

	// When extracting through the desktop adapter
	result, err := app.ExtractKnowledge("session-1", false)

	// Then the malformed response is swallowed as an empty result
	require.NoError(t, err)
	assert.Empty(t, result.Items)
	assert.False(t, result.Truncated)
}

func TestApp_SaveExtractedKnowledge_preservesFullInputAndReturnsSavedIndices(t *testing.T) {
	// Given a knowledge service backed by a repository and a real extraction
	// batch — the backend receipt, not the client input, is authoritative
	// for provenance
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().FindByNormalizedConcept(ctx, "Go", "channels").Return(nil, nil).Twice()
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "Channels" && assert.ObjectsAreEqual([]string{"typed"}, item.Properties)
	})).Return(nil).Once()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{ID: "session-1", Topic: "Go"}, nil).Once()
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{{ID: "message-1", Role: domainstudy.RoleUser, Content: "Explain channels"}}, nil).Twice()
	configs := configmocks.NewMockStore(t)
	configs.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 8}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(ctx, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		return req.SessionID == "session-1" && req.Task == domainllm.TaskKnowledgeExtraction
	})).Return(domainllm.ChatResponse{Content: `{"items":[{"concept":"Channels","definition":"Typed conduits.","properties":["typed"],"evidence":[{"message_id":"message-1","quote":"Explain channels"}]}]}`}, nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(0).Once()
	tx := txmocks.NewMockTransactor(t)
	guard := passingDesktopIndexGuard(t)
	expectDesktopSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.MatchedBy(func(e domainknowledge.Evidence) bool {
		return e.ID != "" && e.OriginType == domainknowledge.OriginSessionMessage &&
			e.OriginID != "" && e.SourceLabel != "" && e.Excerpt != "" && !e.CreatedAt.IsZero()
	})).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	evidenceRepo.EXPECT().LinkToItem(ctx, mock.MatchedBy(func(link domainknowledge.ItemEvidence) bool {
		return link.EvidenceID == "evidence-1" && link.ItemID != ""
	})).Return(nil).Once()
	service := applicationknowledge.NewService(repository, sessions, messages, llm, configs, chunks, tx, store, guard, domainknowledge.RetrievalThresholds{}, evidenceRepo, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)
	extracted, err := app.ExtractKnowledge("session-1", false)
	require.NoError(t, err)
	require.Len(t, extracted.Items, 1)

	// When saving the extracted candidate through the desktop adapter,
	// carrying its batch ID
	result := app.SaveExtractedKnowledge(extracted.BatchID, []KnowledgeItemInput{{
		ID: extracted.Items[0].ID, Topic: "Go", Concept: "Channels", Definition: "Typed conduits.", Properties: []string{"typed"},
	}})

	// Then it is persisted and its exact input index is returned
	assert.Equal(t, []int{0}, result.SavedIndices)
	assert.Empty(t, result.Error)
}

func TestApp_SaveExtractedKnowledge_returnsExactIndicesAlongsidePartialFailure(t *testing.T) {
	// Given an input with no matching receipt, followed by one save and one
	// repository failure among two candidates from a real extraction batch
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().FindByNormalizedConcept(ctx, "Go", "saved").Return(nil, nil).Twice()
	repository.EXPECT().FindByNormalizedConcept(ctx, "Go", "failed").Return(nil, nil).Twice()
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "saved"
	})).Return(nil).Once()
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "failed"
	})).Return(assert.AnError).Once()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{ID: "session-1", Topic: "Go"}, nil).Once()
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{{ID: "message-1", Role: domainstudy.RoleUser, Content: "Explain channels"}}, nil).Twice()
	configs := configmocks.NewMockStore(t)
	configs.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 8}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(ctx, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		return req.SessionID == "session-1" && req.Task == domainllm.TaskKnowledgeExtraction
	})).Return(domainllm.ChatResponse{Content: `{"items":[` +
		`{"concept":"saved","definition":"persisted","evidence":[{"message_id":"message-1","quote":"Explain channels"}]},` +
		`{"concept":"failed","definition":"not persisted","evidence":[{"message_id":"message-1","quote":"Explain channels"}]}` +
		`]}`}, nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(0).Times(2)
	tx := txmocks.NewMockTransactor(t)
	guard := passingDesktopIndexGuard(t)
	expectDesktopSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.MatchedBy(func(e domainknowledge.Evidence) bool {
		return e.ID != "" && e.OriginType == domainknowledge.OriginSessionMessage &&
			e.OriginID != "" && e.SourceLabel != "" && e.Excerpt != "" && !e.CreatedAt.IsZero()
	})).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	evidenceRepo.EXPECT().LinkToItem(ctx, mock.MatchedBy(func(link domainknowledge.ItemEvidence) bool {
		return link.EvidenceID == "evidence-1" && link.ItemID != ""
	})).Return(nil).Once()
	service := applicationknowledge.NewService(repository, sessions, messages, llm, configs, chunks, tx, store, guard, domainknowledge.RetrievalThresholds{}, evidenceRepo, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)
	extracted, err := app.ExtractKnowledge("session-1", false)
	require.NoError(t, err)
	require.Len(t, extracted.Items, 2)

	// When saving through the desktop adapter, with one input carrying an ID
	// that matches no backend receipt at all
	result := app.SaveExtractedKnowledge(extracted.BatchID, []KnowledgeItemInput{
		{ID: "not-a-real-candidate", Topic: "Go", Concept: "invalid", Definition: ""},
		{ID: extracted.Items[0].ID, Topic: "Go", Concept: "saved", Definition: "persisted"},
		{ID: extracted.Items[1].ID, Topic: "Go", Concept: "failed", Definition: "not persisted"},
	})

	// Then the resolved result carries both the precise success and the failure
	assert.Equal(t, []int{1}, result.SavedIndices)
	assert.Contains(t, result.Error, assert.AnError.Error())
}

func TestApp_SaveAndApproveExtractedKnowledge_persistsDirectlyAsApproved(t *testing.T) {
	// Given a knowledge service backed by a repository and a real extraction batch
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().FindByNormalizedConcept(ctx, "Go", "channels").Return(nil, nil).Twice()
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "Channels" && item.Status == domainknowledge.StatusApproved
	})).Return(nil).Once()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{ID: "session-1", Topic: "Go"}, nil).Once()
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{{ID: "message-1", Role: domainstudy.RoleUser, Content: "Explain channels"}}, nil).Twice()
	configs := configmocks.NewMockStore(t)
	configs.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 8}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(ctx, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		return req.SessionID == "session-1" && req.Task == domainllm.TaskKnowledgeExtraction
	})).Return(domainllm.ChatResponse{Content: `{"items":[{"concept":"Channels","definition":"Typed conduits.","evidence":[{"message_id":"message-1","quote":"Explain channels"}]}]}`}, nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(0).Once()
	tx := txmocks.NewMockTransactor(t)
	guard := passingDesktopIndexGuard(t)
	expectDesktopSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.MatchedBy(func(e domainknowledge.Evidence) bool {
		return e.ID != "" && e.OriginType == domainknowledge.OriginSessionMessage &&
			e.OriginID != "" && e.SourceLabel != "" && e.Excerpt != "" && !e.CreatedAt.IsZero()
	})).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	evidenceRepo.EXPECT().LinkToItem(ctx, mock.MatchedBy(func(link domainknowledge.ItemEvidence) bool {
		return link.EvidenceID == "evidence-1" && link.ItemID != ""
	})).Return(nil).Once()
	service := applicationknowledge.NewService(repository, sessions, messages, llm, configs, chunks, tx, store, guard, domainknowledge.RetrievalThresholds{}, evidenceRepo, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)
	extracted, err := app.ExtractKnowledge("session-1", false)
	require.NoError(t, err)
	require.Len(t, extracted.Items, 1)

	// When saving and approving the extracted candidate
	result := app.SaveAndApproveExtractedKnowledge(extracted.BatchID, []KnowledgeItemInput{{
		ID: extracted.Items[0].ID, Topic: "Go", Concept: "Channels", Definition: "Typed conduits.",
	}})

	// Then it is persisted directly as approved and its exact input index is returned
	assert.Equal(t, []int{0}, result.SavedIndices)
	assert.Empty(t, result.Error)
}

func TestApp_DiscardExtraction_leavesTheBatchUnsavable(t *testing.T) {
	// Given a real extraction batch with one pending candidate
	ctx := context.Background()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{ID: "session-1", Topic: "Go"}, nil).Once()
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{{ID: "message-1", Role: domainstudy.RoleUser, Content: "Explain channels"}}, nil).Once()
	configs := configmocks.NewMockStore(t)
	configs.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 8}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(ctx, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		return req.SessionID == "session-1" && req.Task == domainllm.TaskKnowledgeExtraction
	})).Return(domainllm.ChatResponse{Content: `{"items":[{"concept":"Channels","definition":"Typed conduits.","evidence":[{"message_id":"message-1","quote":"Explain channels"}]}]}`}, nil).Once()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().FindByNormalizedConcept(ctx, "Go", "channels").Return(nil, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(0).Once()
	guard := passingDesktopIndexGuard(t)
	service := applicationknowledge.NewService(repository, sessions, messages, llm, configs, knowledgemocks.NewMockChunkRepository(t), nil, store, guard, domainknowledge.RetrievalThresholds{}, knowledgemocks.NewMockEvidenceRepository(t), domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)
	extracted, err := app.ExtractKnowledge("session-1", false)
	require.NoError(t, err)
	require.Len(t, extracted.Items, 1)

	// When discarding the batch and then trying to save from it
	app.DiscardExtraction(extracted.BatchID)
	result := app.SaveExtractedKnowledge(extracted.BatchID, []KnowledgeItemInput{{
		ID: extracted.Items[0].ID, Topic: "Go", Concept: "Channels", Definition: "Typed conduits.",
	}})

	// Then nothing is saved and the repository is never touched
	assert.Empty(t, result.SavedIndices)
	repository.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

func TestApp_ListKnowledgeItems_returnsItemsForTopicAndStatus(t *testing.T) {
	// Given a repository with one matching item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().List(ctx, domainknowledge.Filter{Topic: "Go", Status: domainknowledge.StatusApproved}).
		Return([]domainknowledge.Item{{ID: "item-1", Topic: "Go", Concept: "Channels", Status: domainknowledge.StatusApproved}}, nil).Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)

	// When listing through the desktop adapter
	results, err := app.ListKnowledgeItems("Go", domainknowledge.StatusApproved)

	// Then the item is returned as a KnowledgeItemResult
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Channels", results[0].Concept)
}

func TestApp_CountDraftKnowledgeItems_returnsRepositoryDraftCount(t *testing.T) {
	// Given a repository with two draft items
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().CountByStatus(ctx, domainknowledge.StatusDraft).Return(2, nil).Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)

	// When counting drafts through the desktop adapter
	count, err := app.CountDraftKnowledgeItems()

	// Then the repository's count is returned as-is
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestApp_ListKnowledgeTopics_returnsTopics(t *testing.T) {
	// Given a repository with two topics
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().ListTopics(ctx).Return([]string{"Go", "Kubernetes"}, nil).Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)

	// When listing topics through the desktop adapter
	topics, err := app.ListKnowledgeTopics()

	// Then they are returned as-is
	require.NoError(t, err)
	assert.Equal(t, []string{"Go", "Kubernetes"}, topics)
}

func TestApp_ListKnowledgeItemEvidence_returnsPersistedSnapshotsForTheItem(t *testing.T) {
	// Given an item with a persisted Evidence snapshot
	ctx := context.Background()
	createdAt := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().ListByItem(ctx, "item-1").Return([]domainknowledge.Evidence{
		{ID: "evidence-1", OriginType: domainknowledge.OriginSessionMessage, OriginID: "message-1", SourceLabel: "Distributed systems", Excerpt: "CAP describes trade-offs.", CreatedAt: createdAt},
	}, nil).Once()
	service := applicationknowledge.NewService(knowledgemocks.NewMockRepository(t), studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil, nil, nil, domainknowledge.RetrievalThresholds{}, evidenceRepo, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)

	// When listing that item's evidence through the desktop adapter
	results, err := app.ListKnowledgeItemEvidence("item-1")

	// Then the immutable snapshot is translated in order
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, domainknowledge.OriginSessionMessage, results[0].OriginType)
	assert.Equal(t, "Distributed systems", results[0].SourceLabel)
	assert.Equal(t, "CAP describes trade-offs.", results[0].Excerpt)
	assert.Equal(t, createdAt.Format(time.RFC3339Nano), results[0].CreatedAt)
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
	tx.EXPECT().WithinTx(ctx, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	existingChunks := []domainknowledge.Chunk{{ID: "chunk-1", ItemID: "item-1"}}
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Go", domainknowledge.StatusApproved,
		mock.MatchedBy(func(ts time.Time) bool { return !ts.IsZero() })).Return(existingChunks, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Add(mock.Anything, existingChunks).Return(nil).Once()
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(nil).Once()
	guard.EXPECT().EndMutation().Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), chunks, tx, store, guard, domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
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
	tx.EXPECT().WithinTx(ctx, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	existingChunks := []domainknowledge.Chunk{{ID: "chunk-1", ItemID: "item-1"}}
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Go", domainknowledge.StatusDeprecated,
		mock.MatchedBy(func(ts time.Time) bool { return !ts.IsZero() })).Return(existingChunks, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Add(mock.Anything, existingChunks).Return(nil).Once()
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(nil).Once()
	guard.EXPECT().EndMutation().Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), chunks, tx, store, guard, domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
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
	tx.EXPECT().WithinTx(ctx, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil, nil).Times(2)
	wantContent := "New\n\nNew def."
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: wantContent}).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1" && cs[0].Content == wantContent
	})).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string(nil)).Return(nil).Times(2)
	store.EXPECT().Add(mock.Anything, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1"
	})).Return(nil).Once()
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(nil).Once()
	guard.EXPECT().EndMutation().Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llm, configmocks.NewMockStore(t), chunks, tx, store, guard, domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
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
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil, nil).Once()
	repository.EXPECT().Delete(ctx, "item-1").Return(nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().DeleteUnreferenced(ctx).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	tx.EXPECT().WithinTx(ctx, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, ([]string)(nil)).Return(nil).Once()
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(nil).Once()
	guard.EXPECT().EndMutation().Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), chunks, tx, store, guard, domainknowledge.RetrievalThresholds{}, evidenceRepo, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)

	// When deleting through the desktop adapter
	err := app.DeleteKnowledgeItem("item-1")

	// Then it succeeds
	require.NoError(t, err)
}

// captureLog redirects the standard logger's output into a buffer for the
// duration of the test, restoring it on cleanup.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestApp_ApproveKnowledgeItem_reportsSuccess_whenPostCommitReconciliationFails(t *testing.T) {
	// Given an item whose approval persists but whose store reconciliation fails
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.", Status: domainknowledge.StatusDraft,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	tx.EXPECT().WithinTx(ctx, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	existingChunks := []domainknowledge.Chunk{{ID: "chunk-1", ItemID: "item-1"}}
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Go", domainknowledge.StatusApproved,
		mock.MatchedBy(func(ts time.Time) bool { return !ts.IsZero() })).Return(existingChunks, nil).Once()
	boom := errors.New("store exploded")
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Add(mock.Anything, existingChunks).Return(boom).Once()
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(nil).Once()
	guard.EXPECT().EndMutation().Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), chunks, tx, store, guard, domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)
	logs := captureLog(t)

	// When approving through the desktop adapter
	result, err := app.ApproveKnowledgeItem("item-1")

	// Then the durable transition is reported as successful, and the
	// technical failure is logged rather than swallowed
	require.NoError(t, err)
	assert.Equal(t, domainknowledge.StatusApproved, result.Status)
	assert.Contains(t, logs.String(), boom.Error())
}

func TestApp_DeprecateKnowledgeItem_reportsSuccess_whenPostCommitReconciliationFails(t *testing.T) {
	// Given an item whose deprecation persists but whose store reconciliation fails
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.", Status: domainknowledge.StatusApproved,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	tx.EXPECT().WithinTx(ctx, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	existingChunks := []domainknowledge.Chunk{{ID: "chunk-1", ItemID: "item-1"}}
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Go", domainknowledge.StatusDeprecated,
		mock.MatchedBy(func(ts time.Time) bool { return !ts.IsZero() })).Return(existingChunks, nil).Once()
	boom := errors.New("store exploded")
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Add(mock.Anything, existingChunks).Return(boom).Once()
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(nil).Once()
	guard.EXPECT().EndMutation().Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), chunks, tx, store, guard, domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)
	logs := captureLog(t)

	// When deprecating through the desktop adapter
	result, err := app.DeprecateKnowledgeItem("item-1")

	// Then the durable transition is reported as successful
	require.NoError(t, err)
	assert.Equal(t, domainknowledge.StatusDeprecated, result.Status)
	assert.Contains(t, logs.String(), boom.Error())
}

func TestApp_UpdateKnowledgeItem_reportsSuccess_whenPostCommitReconciliationFails(t *testing.T) {
	// Given a topic-only edit (metadata-only path) whose store reconciliation fails
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.", Status: domainknowledge.StatusApproved,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	tx.EXPECT().WithinTx(ctx, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	existingChunks := []domainknowledge.Chunk{{ID: "chunk-1", ItemID: "item-1"}}
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Distributed systems", domainknowledge.StatusApproved,
		mock.MatchedBy(func(ts time.Time) bool { return !ts.IsZero() })).Return(existingChunks, nil).Once()
	boom := errors.New("store exploded")
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Add(mock.Anything, existingChunks).Return(boom).Once()
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(nil).Once()
	guard.EXPECT().EndMutation().Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), chunks, tx, store, guard, domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)
	logs := captureLog(t)

	// When updating through the desktop adapter, changing only the topic
	result, err := app.UpdateKnowledgeItem("item-1", KnowledgeItemInput{
		Topic: "Distributed systems", Concept: "Channels", Definition: "Typed conduits.",
	})

	// Then the durable update is reported as successful
	require.NoError(t, err)
	assert.Equal(t, "Distributed systems", result.Topic)
	assert.Contains(t, logs.String(), boom.Error())
}

func TestApp_DeleteKnowledgeItem_reportsSuccess_whenPostCommitReconciliationFails(t *testing.T) {
	// Given an item whose delete persists but whose store reconciliation fails
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return([]string{"chunk-1"}, nil).Once()
	repository.EXPECT().Delete(ctx, "item-1").Return(nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().DeleteUnreferenced(ctx).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	tx.EXPECT().WithinTx(ctx, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	boom := errors.New("store exploded")
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string{"chunk-1"}).Return(boom).Once()
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(nil).Once()
	guard.EXPECT().EndMutation().Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), chunks, tx, store, guard, domainknowledge.RetrievalThresholds{}, evidenceRepo, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)
	logs := captureLog(t)

	// When deleting through the desktop adapter
	err := app.DeleteKnowledgeItem("item-1")

	// Then the durable delete is reported as successful
	require.NoError(t, err)
	assert.Contains(t, logs.String(), boom.Error())
}

// capturedReindexEvents records every ingest:* event emitted through
// App.emit during a ReindexKnowledgeItems test.
type capturedReindexEvents struct {
	progress []ReindexProgressResult
	done     *ReindexSummaryResult
	errors   []string
}

func newTestReindexApp(t *testing.T, service *applicationknowledge.Service) (*App, *capturedReindexEvents) {
	t.Helper()
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(context.Background())

	captured := &capturedReindexEvents{}
	app.emit = func(_ context.Context, eventName string, data ...interface{}) {
		switch eventName {
		case eventIngestProgress:
			p := data[0].(ReindexProgressResult)
			captured.progress = append(captured.progress, p)
		case eventIngestDone:
			s := data[0].(ReindexSummaryResult)
			captured.done = &s
		case eventIngestError:
			captured.errors = append(captured.errors, data[0].(string))
		}
	}
	return app, captured
}

func TestApp_CountUnindexedKnowledgeItems_returnsRepositoryCount(t *testing.T) {
	// Given a repository reporting 4 unindexed items
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().CountUnindexed(ctx, domainllm.EmbeddingModel).Return(4, nil).Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)

	// When counting through the desktop adapter
	count, err := app.CountUnindexedKnowledgeItems()

	// Then the repository's count is returned as-is
	require.NoError(t, err)
	assert.Equal(t, 4, count)
}

func TestApp_ReindexKnowledgeItems_emitsProgressThenDone_onSuccess(t *testing.T) {
	// Given one unindexed item that embeds successfully
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().ListUnindexed(ctx, domainllm.EmbeddingModel).Return([]domainknowledge.Item{
		{ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.", Status: domainknowledge.StatusApproved},
	}, nil).Once()
	wantContent := "Channels\n\nTyped conduits."
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: wantContent}).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1" && cs[0].Content == wantContent
	})).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string(nil)).Return(nil).Once()
	store.EXPECT().Add(mock.Anything, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1"
	})).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	tx.EXPECT().WithinTx(ctx, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(nil).Once()
	guard.EXPECT().EndMutation().Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llm, configmocks.NewMockStore(t), chunks, tx, store, guard, domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app, captured := newTestReindexApp(t, service)

	// When reindexing through the desktop adapter
	err := app.ReindexKnowledgeItems()

	// Then progress is emitted for the one item, followed by a done summary
	require.NoError(t, err)
	require.Len(t, captured.progress, 1)
	assert.Equal(t, 1, captured.progress[0].ItemsTotal)
	assert.Equal(t, "Go", captured.progress[0].CurrentTopic)
	require.NotNil(t, captured.done)
	assert.Equal(t, 1, captured.done.ItemsIndexed)
	assert.Empty(t, captured.errors)
}

func TestApp_ReindexKnowledgeItems_emitsError_whenTheRunFails(t *testing.T) {
	// Given a repository whose listing fails
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	listErr := errors.New("database locked")
	repository.EXPECT().ListUnindexed(ctx, domainllm.EmbeddingModel).Return(nil, listErr).Once()
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(nil).Once()
	guard.EXPECT().EndMutation().Once()
	service := applicationknowledge.NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil, nil, guard, domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app, captured := newTestReindexApp(t, service)

	// When reindexing through the desktop adapter
	err := app.ReindexKnowledgeItems()

	// Then the error is emitted and returned, with no done summary
	require.Error(t, err)
	require.Len(t, captured.errors, 1)
	assert.Contains(t, captured.errors[0], listErr.Error())
	assert.Nil(t, captured.done)
}
