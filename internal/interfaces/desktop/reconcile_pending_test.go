package desktop

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	applicationknowledge "github.com/santaniello/athena/internal/application/knowledge"
	txmocks "github.com/santaniello/athena/internal/application/knowledge/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
)

func pendingDesktopProposal(id, action, targetItemID string, targetUpdatedAt time.Time) domainknowledge.ReconciliationProposal {
	return domainknowledge.ReconciliationProposal{
		ID: id, Action: action, Status: domainknowledge.ProposalPending,
		Candidate: domainknowledge.Item{
			Topic: "Distributed Systems", Concept: "Idempotency key",
			Definition: "A unique value a client attaches to a request so retries produce the same effect exactly once.",
			Source:     domainknowledge.SourceAthena, Status: domainknowledge.StatusDraft,
		},
		TargetItemID: targetItemID, TargetUpdatedAt: targetUpdatedAt,
		Reason: "classified reason", CreatedAt: time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC),
		EvidenceIDs: []string{"evidence-1"},
	}
}

func TestApp_ListPendingReconciliations_mapsStalenessAndTargetInfo(t *testing.T) {
	// Given one fresh and one stale pending proposal
	ctx := context.Background()
	freshUpdatedAt := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	proposals := []domainknowledge.ReconciliationProposal{
		pendingDesktopProposal("proposal-fresh", domainknowledge.ReconcileUpdate, "item-fresh", freshUpdatedAt),
		pendingDesktopProposal("proposal-stale", domainknowledge.ReconcileUpdate, "item-gone", freshUpdatedAt),
	}
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().ListByStatus(ctx, domainknowledge.ProposalPending).Return(proposals, nil).Once()
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().GetByID(ctx, "item-fresh").
		Return(domainknowledge.Item{ID: "item-fresh", Concept: "Eventual consistency", Status: domainknowledge.StatusApproved, UpdatedAt: freshUpdatedAt}, nil).Once()
	repo.EXPECT().GetByID(ctx, "item-gone").Return(domainknowledge.Item{}, domainknowledge.ErrItemNotFound).Once()
	service := applicationknowledge.NewService(repo, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, reconciliations, nil, 0, 0)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)

	// When listing pending reconciliations through the desktop adapter
	results, err := app.ListPendingReconciliations()

	// Then staleness and target info are mapped for each row
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.False(t, results[0].Stale)
	assert.Equal(t, "Eventual consistency", results[0].TargetConcept)
	assert.True(t, results[1].Stale)
}

func TestApp_ApplyPendingReconciliationCreate_createsANewDraftItem(t *testing.T) {
	// Given a pending create proposal
	ctx := context.Background()
	proposal := pendingDesktopProposal("proposal-1", domainknowledge.ReconcileCreate, "", time.Time{})
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().GetByID(ctx, "proposal-1").Return(proposal, nil).Once()
	reconciliations.EXPECT().UpdateStatus(ctx, "proposal-1", domainknowledge.ProposalApplied, "classified reason", mock.Anything).Return(nil).Once()
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().FindByNormalizedConcept(ctx, "Distributed Systems", "idempotency key").Return(nil, nil).Once()
	repo.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "Idempotency key" && item.Status == domainknowledge.StatusDraft
	})).Return(nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().LinkToItem(ctx, mock.Anything).Return(nil).Once()
	llm := llmmocks.NewMockProvider(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	tx := txmocks.NewMockTransactor(t)
	expectDesktopSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	service := applicationknowledge.NewService(repo, nil, nil, llm, nil, chunks, tx, store, passingDesktopIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo, reconciliations, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)

	// When applying it through the desktop adapter
	result, err := app.ApplyPendingReconciliationCreate("proposal-1", domainknowledge.StatusDraft)

	// Then a brand-new draft item is returned
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, domainknowledge.StatusDraft, result.Status)
}

func TestApp_RejectPendingReconciliationProposal_returnsErrorForAnAlreadyResolvedProposal(t *testing.T) {
	// Given a proposal that was already applied
	ctx := context.Background()
	proposal := pendingDesktopProposal("proposal-1", domainknowledge.ReconcileCreate, "", time.Time{})
	proposal.Status = domainknowledge.ProposalApplied
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().GetByID(ctx, "proposal-1").Return(proposal, nil).Once()
	service := applicationknowledge.NewService(nil, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, reconciliations, nil, 0, 0)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)

	// When rejecting it through the desktop adapter
	err := app.RejectPendingReconciliationProposal("proposal-1")

	// Then the error surfaces to the caller
	assert.ErrorIs(t, err, applicationknowledge.ErrReconciliationProposalNotPending)
}

func TestApp_CountPendingReconciliations_returnsTheRepositoryCount(t *testing.T) {
	// Given two pending proposals
	ctx := context.Background()
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().CountByStatus(ctx, domainknowledge.ProposalPending).Return(2, nil).Once()
	service := applicationknowledge.NewService(nil, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, reconciliations, nil, 0, 0)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, service, nil, nil, nil)
	app.Startup(ctx)

	// When counting them through the desktop adapter
	count, err := app.CountPendingReconciliations()

	// Then the repository's count is returned
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}
