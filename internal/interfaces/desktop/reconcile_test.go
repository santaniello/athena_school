package desktop

import (
	"context"
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

func reconciliationCandidateInput(id string) KnowledgeItemInput {
	return KnowledgeItemInput{
		ID: id, Topic: "Distributed Systems", Concept: "Idempotency key",
		Definition: "A unique value a client attaches to a request so retries produce the same effect exactly once.",
		Source:     domainknowledge.SourceAthena, Status: domainknowledge.StatusDraft,
	}
}

func TestApp_ApplyReconciliationCreate_createsANewDraftItem(t *testing.T) {
	// Given a real extraction batch whose one candidate has no duplicate
	// shortlist — classified as a deterministic create, no comparison call
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().FindByNormalizedConcept(ctx, "Distributed Systems", "idempotency key").Return(nil, nil).Twice()
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "Idempotency key" && item.Status == domainknowledge.StatusDraft
	})).Return(nil).Once()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{ID: "session-1", Topic: "Distributed Systems"}, nil).Once()
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "An idempotency key lets retries be safe."},
	}, nil).Twice()
	configs := configmocks.NewMockStore(t)
	configs.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 8}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(ctx, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		return req.Task == domainllm.TaskKnowledgeExtraction
	})).Return(domainllm.ChatResponse{
		Content: `{"items":[{"concept":"Idempotency key","definition":"A unique value a client attaches to a request so retries produce the same effect exactly once.","evidence":[{"message_id":"message-1","quote":"An idempotency key lets retries be safe."}]}]}`,
	}, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(0).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	tx := txmocks.NewMockTransactor(t)
	guard := passingDesktopIndexGuard(t)
	expectDesktopSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.Anything).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().Save(ctx, mock.MatchedBy(func(p domainknowledge.ReconciliationProposal) bool {
		return p.Action == domainknowledge.ReconcileCreate && p.Status == domainknowledge.ProposalApplied
	})).Return(nil).Once()
	reconciliations.EXPECT().LinkEvidence(ctx, mock.Anything, "evidence-1").Return(nil).Once()
	service := applicationknowledge.NewService(repository, sessions, messages, llm, configs, chunks, tx, store, guard, domainknowledge.RetrievalThresholds{}, evidenceRepo, reconciliations, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)
	extracted, err := app.ExtractKnowledge("session-1", false)
	require.NoError(t, err)
	require.Len(t, extracted.Items, 1)
	require.NotNil(t, extracted.Items[0].Reconciliation)
	assert.Equal(t, domainknowledge.ReconcileCreate, extracted.Items[0].Reconciliation.Action)

	// When applying create through the desktop adapter
	result, err := app.ApplyReconciliationCreate(extracted.BatchID, extracted.Items[0].ID, reconciliationCandidateInput(extracted.Items[0].ID), domainknowledge.StatusDraft)

	// Then a brand-new item is returned with a fresh, server-generated ID
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.NotEqual(t, extracted.Items[0].ID, result.ID)
	assert.Equal(t, domainknowledge.StatusDraft, result.Status)
}

func TestApp_SaveReconciliationForReview_succeedsWithoutPersistingAnyItem(t *testing.T) {
	// Given the same kind of real extraction batch
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().FindByNormalizedConcept(ctx, "Distributed Systems", "idempotency key").Return(nil, nil).Once()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{ID: "session-1", Topic: "Distributed Systems"}, nil).Once()
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "An idempotency key lets retries be safe."},
	}, nil).Twice()
	configs := configmocks.NewMockStore(t)
	configs.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 8}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(ctx, mock.Anything).Return(domainllm.ChatResponse{
		Content: `{"items":[{"concept":"Idempotency key","definition":"A unique value a client attaches to a request so retries produce the same effect exactly once.","evidence":[{"message_id":"message-1","quote":"An idempotency key lets retries be safe."}]}]}`,
	}, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(0).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.Anything).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().Save(ctx, mock.MatchedBy(func(p domainknowledge.ReconciliationProposal) bool {
		return p.Status == domainknowledge.ProposalPending
	})).Return(nil).Once()
	reconciliations.EXPECT().LinkEvidence(ctx, mock.Anything, "evidence-1").Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	tx.EXPECT().WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	// repository has no further .EXPECT() for Save/Update — proving no item
	// is persisted; guard is nil since SaveReconciliationForReview never
	// touches the IndexGuard at all — only ExtractKnowledge's own
	// classification runs here, and that doesn't index anything either
	service := applicationknowledge.NewService(repository, sessions, messages, llm, configs, knowledgemocks.NewMockChunkRepository(t), tx, store, nil, domainknowledge.RetrievalThresholds{}, evidenceRepo, reconciliations, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)
	extracted, err := app.ExtractKnowledge("session-1", false)
	require.NoError(t, err)
	require.Len(t, extracted.Items, 1)

	// When saving it for review through the desktop adapter
	err = app.SaveReconciliationForReview(extracted.BatchID, extracted.Items[0].ID, reconciliationCandidateInput(extracted.Items[0].ID))

	// Then it succeeds
	require.NoError(t, err)
}

func TestApp_ApplyReconciliationUpdate_returnsErrorForAnAlreadyDecidedCandidate(t *testing.T) {
	// Given a knowledge service with no receipt for the candidate at all
	ctx := context.Background()
	service := applicationknowledge.NewService(nil, nil, nil, nil, nil, nil, nil, nil, passingDesktopIndexGuard(t), domainknowledge.RetrievalThresholds{}, nil, nil, nil, 0, 0)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)

	// When applying update through the desktop adapter anyway
	_, err := app.ApplyReconciliationUpdate("missing-batch", "candidate-1", reconciliationCandidateInput("candidate-1"))

	// Then the error surfaces to the caller instead of panicking
	assert.ErrorIs(t, err, applicationknowledge.ErrReconciliationCandidateNotFound)
}
