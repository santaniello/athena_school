package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	txmocks "github.com/santaniello/athena/internal/application/knowledge/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
)

func pendingProposal(id, action, targetItemID string, targetUpdatedAt time.Time) domainknowledge.ReconciliationProposal {
	return domainknowledge.ReconciliationProposal{
		ID: id, Action: action, Status: domainknowledge.ProposalPending,
		Candidate: reconciliationCandidateContent(), TargetItemID: targetItemID, TargetUpdatedAt: targetUpdatedAt,
		Reason: "classified reason", CreatedAt: time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC),
		EvidenceIDs: []string{"evidence-1"},
	}
}

func TestListPendingReconciliations_marksATargetlessAndAFreshProposalUsable_andAStaleOneStale(t *testing.T) {
	// Given three pending proposals: one with no target (create), one whose
	// target is unchanged, and one whose target was deleted since classification
	ctx := context.Background()
	freshUpdatedAt := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	proposals := []domainknowledge.ReconciliationProposal{
		pendingProposal("proposal-create", domainknowledge.ReconcileCreate, "", time.Time{}),
		pendingProposal("proposal-fresh", domainknowledge.ReconcileUpdate, "item-fresh", freshUpdatedAt),
		pendingProposal("proposal-stale", domainknowledge.ReconcileUpdate, "item-gone", freshUpdatedAt),
	}
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().ListByStatus(ctx, domainknowledge.ProposalPending).Return(proposals, nil).Once()
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().GetByID(ctx, "item-fresh").
		Return(domainknowledge.Item{ID: "item-fresh", Concept: "Eventual consistency", Status: domainknowledge.StatusApproved, UpdatedAt: freshUpdatedAt}, nil).Once()
	repo.EXPECT().GetByID(ctx, "item-gone").Return(domainknowledge.Item{}, domainknowledge.ErrItemNotFound).Once()
	service := NewService(repo, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, reconciliations, nil, 0, 0)

	// When listing pending reconciliations
	results, err := service.ListPendingReconciliations(ctx)

	// Then the create (no target) and the fresh one are usable, and only the
	// one whose target vanished is marked stale — the list itself never fails
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.False(t, results[0].Stale)
	assert.False(t, results[1].Stale)
	assert.Equal(t, "Eventual consistency", results[1].TargetConcept)
	assert.Equal(t, domainknowledge.StatusApproved, results[1].TargetStatus)
	assert.True(t, results[2].Stale)
}

func TestListPendingReconciliations_propagatesAGenuineErrorFromReloadingATarget(t *testing.T) {
	// Given a pending proposal whose target lookup fails for a reason other
	// than "the item no longer exists"
	ctx := context.Background()
	proposal := pendingProposal("proposal-1", domainknowledge.ReconcileUpdate, "item-target", time.Now().UTC())
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().ListByStatus(ctx, domainknowledge.ProposalPending).
		Return([]domainknowledge.ReconciliationProposal{proposal}, nil).Once()
	repo := knowledgemocks.NewMockRepository(t)
	dbErr := errors.New("database locked")
	repo.EXPECT().GetByID(ctx, "item-target").Return(domainknowledge.Item{}, dbErr).Once()
	service := NewService(repo, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, reconciliations, nil, 0, 0)

	// When listing pending reconciliations
	_, err := service.ListPendingReconciliations(ctx)

	// Then the real error surfaces — it is not mistaken for a stale target
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrReconciliationTargetStale)
}

func TestApplyPendingReconciliationCreate_persistsANewItemAndLinksAlreadyMaterializedEvidence(t *testing.T) {
	// Given a pending create proposal with evidence already saved
	ctx := context.Background()
	proposal := pendingProposal("proposal-1", domainknowledge.ReconcileCreate, "", time.Time{})
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().GetByID(ctx, "proposal-1").Return(proposal, nil).Once()
	reconciliations.EXPECT().UpdateStatus(ctx, "proposal-1", domainknowledge.ProposalApplied, "classified reason", mock.Anything).Return(nil).Once()
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().FindByNormalizedConcept(ctx, "Distributed Systems", "idempotency key").Return(nil, nil).Once()
	repo.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "Idempotency key" && item.Status == domainknowledge.StatusDraft
	})).Return(nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().LinkToItem(ctx, mock.MatchedBy(func(link domainknowledge.ItemEvidence) bool {
		return link.EvidenceID == "evidence-1" && link.ItemID != ""
	})).Return(nil).Once()
	llm := llmmocks.NewMockProvider(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	tx := txmocks.NewMockTransactor(t)
	expectSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	service := NewService(repo, nil, nil, llm, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo, reconciliations, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// When applying it, as a draft
	item, err := service.ApplyPendingReconciliationCreate(ctx, "proposal-1", domainknowledge.StatusDraft)

	// Then a brand-new item is created and the already-saved evidence is
	// linked to it directly, with no session reloaded and no message
	// revalidated
	require.NoError(t, err)
	assert.NotEmpty(t, item.ID)
}

func TestApplyPendingReconciliationCreate_restoresNothingButPropagatesTheErrorWhenLinkingEvidenceFails(t *testing.T) {
	// Given a pending create proposal whose item persists but whose
	// evidence link fails
	ctx := context.Background()
	proposal := pendingProposal("proposal-1", domainknowledge.ReconcileCreate, "", time.Time{})
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().GetByID(ctx, "proposal-1").Return(proposal, nil).Once()
	reconciliations.EXPECT().UpdateStatus(ctx, "proposal-1", domainknowledge.ProposalApplied, "classified reason", mock.Anything).Return(nil).Once()
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().FindByNormalizedConcept(ctx, "Distributed Systems", "idempotency key").Return(nil, nil).Once()
	repo.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().LinkToItem(ctx, mock.Anything).Return(errors.New("disk full")).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repo, nil, nil, nil, nil, nil, tx, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo, reconciliations, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// When applying it
	_, err := service.ApplyPendingReconciliationCreate(ctx, "proposal-1", domainknowledge.StatusDraft)

	// Then the failure propagates
	require.Error(t, err)
}

func TestApplyPendingReconciliationCreate_returnsIndexingFailureButKeepsTheDurableItem(t *testing.T) {
	// Given a pending create proposal whose item persists but whose
	// post-commit embedding call fails
	ctx := context.Background()
	proposal := pendingProposal("proposal-1", domainknowledge.ReconcileCreate, "", time.Time{})
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().GetByID(ctx, "proposal-1").Return(proposal, nil).Once()
	reconciliations.EXPECT().UpdateStatus(ctx, "proposal-1", domainknowledge.ProposalApplied, "classified reason", mock.Anything).Return(nil).Once()
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().FindByNormalizedConcept(ctx, "Distributed Systems", "idempotency key").Return(nil, nil).Once()
	repo.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().LinkToItem(ctx, mock.Anything).Return(nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, mock.Anything).Return(domainllm.EmbeddingResponse{}, errors.New("openrouter: unavailable")).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repo, nil, nil, llm, nil, nil, tx, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo, reconciliations, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// When applying it
	item, err := service.ApplyPendingReconciliationCreate(ctx, "proposal-1", domainknowledge.StatusDraft)

	// Then the durable write is not reported as a failure — the caller gets
	// the real, persisted item alongside a typed indexing failure
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrIndexingFailed)
	assert.NotEmpty(t, item.ID)
}

func TestApplyPendingReconciliationCreate_returnsNotPendingWhenAlreadyResolved(t *testing.T) {
	// Given a proposal that was already applied
	ctx := context.Background()
	proposal := pendingProposal("proposal-1", domainknowledge.ReconcileCreate, "", time.Time{})
	proposal.Status = domainknowledge.ProposalApplied
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().GetByID(ctx, "proposal-1").Return(proposal, nil).Once()
	service := NewService(nil, nil, nil, nil, nil, nil, nil, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, nil, reconciliations, nil, 0, 0)

	// When trying to apply it again
	_, err := service.ApplyPendingReconciliationCreate(ctx, "proposal-1", domainknowledge.StatusDraft)

	// Then it is refused — repo has no .EXPECT() for Save, proving nothing was created
	assert.ErrorIs(t, err, ErrReconciliationProposalNotPending)
}

func TestApplyPendingReconciliationUpdate_appliesChangesAndKeepsIdentity(t *testing.T) {
	// Given a pending update proposal against a still-fresh target
	ctx := context.Background()
	targetUpdatedAt := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	newDefinition := "Converges eventually, with no read-your-writes guarantee."
	proposal := pendingProposal("proposal-1", domainknowledge.ReconcileUpdate, "item-target", targetUpdatedAt)
	proposal.Changes = domainknowledge.ItemChanges{Definition: &newDefinition}
	target := domainknowledge.Item{
		ID: "item-target", Topic: "Distributed Systems", Concept: "Eventual consistency", Definition: "Converges eventually.",
		Source: domainknowledge.SourceAthena, Status: domainknowledge.StatusApproved,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), UpdatedAt: targetUpdatedAt,
	}
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().GetByID(ctx, "proposal-1").Return(proposal, nil).Once()
	reconciliations.EXPECT().UpdateStatus(ctx, "proposal-1", domainknowledge.ProposalApplied, "classified reason", mock.Anything).Return(nil).Once()
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().GetByID(ctx, "item-target").Return(target, nil).Once()
	repo.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID == "item-target" && item.CreatedAt.Equal(target.CreatedAt) && item.Definition == newDefinition
	})).Return(nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().LinkToItem(ctx, domainknowledge.ItemEvidence{ItemID: "item-target", EvidenceID: "evidence-1"}).Return(nil).Once()
	llm := llmmocks.NewMockProvider(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	tx := txmocks.NewMockTransactor(t)
	expectSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	service := NewService(repo, nil, nil, llm, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo, reconciliations, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// When applying it
	item, err := service.ApplyPendingReconciliationUpdate(ctx, "proposal-1")

	// Then the target's identity survives and only the reviewed field changed
	require.NoError(t, err)
	assert.Equal(t, "item-target", item.ID)
	assert.Equal(t, newDefinition, item.Definition)
}

func TestApplyPendingReconciliationUpdate_returnsStaleWhenTheTargetChangedSinceClassification(t *testing.T) {
	// Given a pending update proposal whose target was edited afterward
	ctx := context.Background()
	classifiedAt := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	proposal := pendingProposal("proposal-1", domainknowledge.ReconcileUpdate, "item-target", classifiedAt)
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().GetByID(ctx, "proposal-1").Return(proposal, nil).Once()
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().GetByID(ctx, "item-target").
		Return(domainknowledge.Item{ID: "item-target", UpdatedAt: classifiedAt.Add(time.Hour)}, nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repo, nil, nil, nil, nil, nil, tx, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, nil, reconciliations, nil, 0, 0)

	// When trying to apply it
	_, err := service.ApplyPendingReconciliationUpdate(ctx, "proposal-1")

	// Then it is refused as stale
	assert.ErrorIs(t, err, ErrReconciliationTargetStale)
}

func TestApplyPendingReconciliationRelate_createsADraftAndACanonicalRelation(t *testing.T) {
	// Given a pending relate proposal
	ctx := context.Background()
	targetUpdatedAt := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	proposal := pendingProposal("proposal-1", domainknowledge.ReconcileRelate, "item-target", targetUpdatedAt)
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().GetByID(ctx, "proposal-1").Return(proposal, nil).Once()
	reconciliations.EXPECT().UpdateStatus(ctx, "proposal-1", domainknowledge.ProposalApplied, "classified reason", mock.Anything).Return(nil).Once()
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().GetByID(ctx, "item-target").Return(domainknowledge.Item{ID: "item-target", UpdatedAt: targetUpdatedAt}, nil).Once()
	repo.EXPECT().FindByNormalizedConcept(ctx, "Distributed Systems", "idempotency key").Return(nil, nil).Once()
	var createdItemID string
	repo.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		createdItemID = item.ID
		return item.Status == domainknowledge.StatusDraft
	})).Return(nil).Once()
	relations := knowledgemocks.NewMockRelationRepository(t)
	relations.EXPECT().Save(ctx, mock.MatchedBy(func(r domainknowledge.Relation) bool {
		return r.Type == domainknowledge.RelationRelated &&
			(r.FromItemID == createdItemID || r.ToItemID == createdItemID)
	})).Return(nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().LinkToItem(ctx, mock.Anything).Return(nil).Once()
	llm := llmmocks.NewMockProvider(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	tx := txmocks.NewMockTransactor(t)
	expectSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	service := NewService(repo, nil, nil, llm, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo, reconciliations, relations, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// When applying it
	item, err := service.ApplyPendingReconciliationRelate(ctx, "proposal-1")

	// Then a new draft item is created, distinct from the target
	require.NoError(t, err)
	assert.NotEqual(t, "item-target", item.ID)
	assert.Equal(t, domainknowledge.StatusDraft, item.Status)
}

func TestResolvePendingReconciliationConflict_keepExistingAppliesNoItemMutation(t *testing.T) {
	// Given a pending conflict proposal
	ctx := context.Background()
	targetUpdatedAt := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	proposal := pendingProposal("proposal-1", domainknowledge.ReconcileConflict, "item-target", targetUpdatedAt)
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().GetByID(ctx, "proposal-1").Return(proposal, nil).Once()
	reconciliations.EXPECT().UpdateStatus(ctx, "proposal-1", domainknowledge.ProposalApplied, "classified reason; resolved: kept existing item", mock.Anything).Return(nil).Once()
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().GetByID(ctx, "item-target").Return(domainknowledge.Item{ID: "item-target", UpdatedAt: targetUpdatedAt}, nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repo, nil, nil, nil, nil, nil, tx, nil, nil, domainknowledge.RetrievalThresholds{}, nil, reconciliations, nil, 0, 0)

	// When resolving it by keeping the existing item
	item, err := service.ResolvePendingReconciliationConflict(ctx, "proposal-1", ConflictKeepExisting)

	// Then no item is created or changed
	require.NoError(t, err)
	assert.Empty(t, item.ID)
}

func TestResolvePendingReconciliationConflict_rejectsAnUnknownResolution(t *testing.T) {
	// Given a pending conflict proposal
	ctx := context.Background()
	proposal := pendingProposal("proposal-1", domainknowledge.ReconcileConflict, "item-target", time.Now().UTC())
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	service := NewService(nil, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, reconciliations, nil, 0, 0)
	_ = proposal

	// When resolving it with an unrecognized outcome
	_, err := service.ResolvePendingReconciliationConflict(ctx, "proposal-1", "discard_everything")

	// Then it is rejected without ever loading the proposal — repo has no
	// .EXPECT() for GetByID
	assert.ErrorIs(t, err, ErrReconciliationResolutionInvalid)
}

func TestAcknowledgePendingReconciliationNoChange_marksAppliedWithoutTouchingAnyItem(t *testing.T) {
	// Given a pending no_change proposal
	ctx := context.Background()
	targetUpdatedAt := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	proposal := pendingProposal("proposal-1", domainknowledge.ReconcileNoChange, "item-target", targetUpdatedAt)
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().GetByID(ctx, "proposal-1").Return(proposal, nil).Once()
	reconciliations.EXPECT().UpdateStatus(ctx, "proposal-1", domainknowledge.ProposalApplied, "classified reason", mock.Anything).Return(nil).Once()
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().GetByID(ctx, "item-target").Return(domainknowledge.Item{ID: "item-target", UpdatedAt: targetUpdatedAt}, nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repo, nil, nil, nil, nil, nil, tx, nil, nil, domainknowledge.RetrievalThresholds{}, nil, reconciliations, nil, 0, 0)

	// When acknowledging it
	err := service.AcknowledgePendingReconciliationNoChange(ctx, "proposal-1")

	// Then it succeeds without creating or changing any item
	require.NoError(t, err)
}

func TestRejectPendingReconciliationProposal_marksRejectedEvenWhenTheTargetChanged(t *testing.T) {
	// Given a pending proposal whose target was edited since classification —
	// rejecting must still work, since nothing gets applied to it
	ctx := context.Background()
	proposal := pendingProposal("proposal-1", domainknowledge.ReconcileUpdate, "item-target", time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().GetByID(ctx, "proposal-1").Return(proposal, nil).Once()
	reconciliations.EXPECT().UpdateStatus(ctx, "proposal-1", domainknowledge.ProposalRejected, "classified reason", mock.Anything).Return(nil).Once()
	// repo has no .EXPECT() for GetByID — proving no staleness check runs
	service := NewService(knowledgemocks.NewMockRepository(t), nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, reconciliations, nil, 0, 0)

	// When rejecting it
	err := service.RejectPendingReconciliationProposal(ctx, "proposal-1")

	// Then it succeeds
	require.NoError(t, err)
}

func TestRejectPendingReconciliationProposal_returnsNotPendingWhenAlreadyResolved(t *testing.T) {
	// Given a proposal that was already rejected
	ctx := context.Background()
	proposal := pendingProposal("proposal-1", domainknowledge.ReconcileCreate, "", time.Time{})
	proposal.Status = domainknowledge.ProposalRejected
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().GetByID(ctx, "proposal-1").Return(proposal, nil).Once()
	service := NewService(nil, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, reconciliations, nil, 0, 0)

	// When rejecting it again
	err := service.RejectPendingReconciliationProposal(ctx, "proposal-1")

	// Then it is refused
	assert.ErrorIs(t, err, ErrReconciliationProposalNotPending)
}

func TestCountPendingReconciliations_delegatesToTheRepository(t *testing.T) {
	// Given a repository reporting two pending proposals
	ctx := context.Background()
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().CountByStatus(ctx, domainknowledge.ProposalPending).Return(2, nil).Once()
	service := NewService(nil, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, reconciliations, nil, 0, 0)

	// When counting them
	count, err := service.CountPendingReconciliations(ctx)

	// Then the repository's count is returned as-is
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}
