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
	configmocks "github.com/santaniello/athena/internal/domain/config/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

// reconciliationTarget is one existing item used across parseReconciliation tests.
func reconciliationTarget() domainknowledge.Item {
	return domainknowledge.Item{
		ID: "item-target", Topic: "Distributed Systems", Concept: "Eventual consistency",
		Definition: "Converges eventually.", Status: domainknowledge.StatusApproved,
		UpdatedAt: time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC),
	}
}

func TestParseReconciliation_rejectsMalformedOrUnresolvableResponses(t *testing.T) {
	targets := []domainknowledge.Item{reconciliationTarget()}
	tests := []struct {
		name string
		raw  string
	}{
		{name: "not json", raw: "not json at all"},
		{name: "unknown action", raw: `{"action":"merge","target_item_id":"item-target","reason":"why"}`},
		{name: "missing reason", raw: `{"action":"update","target_item_id":"item-target"}`},
		{name: "blank reason", raw: `{"action":"update","target_item_id":"item-target","reason":"   "}`},
		{name: "target required but missing", raw: `{"action":"update","reason":"why"}`},
		{name: "target outside shortlist", raw: `{"action":"update","target_item_id":"item-outsider","reason":"why"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When parsing a structurally invalid classifier response
			_, err := parseReconciliation(test.raw, targets)

			// Then it is rejected as malformed rather than guessed at
			assert.ErrorIs(t, err, ErrMalformedReconciliation)
		})
	}
}

func TestParseReconciliation_acceptsCreateWithoutATarget(t *testing.T) {
	// Given a response proposing create — no existing item matches
	targets := []domainknowledge.Item{reconciliationTarget()}

	// When parsing it
	classification, err := parseReconciliation(`{"action":"create","reason":"no existing match"}`, targets)

	// Then it carries no target at all
	require.NoError(t, err)
	assert.Equal(t, domainknowledge.ReconcileCreate, classification.Action)
	assert.Empty(t, classification.TargetItemID)
	assert.True(t, classification.TargetUpdatedAt.IsZero())
	assert.Equal(t, "no existing match", classification.Reason)
}

func TestParseReconciliation_resolvesTheTargetsUpdatedAtFromTheSuppliedShortlist(t *testing.T) {
	// Given a response proposing update against the one shortlisted item,
	// with a bounded diff of changed fields only
	target := reconciliationTarget()
	raw := `{"action":"update","target_item_id":"item-target","reason":"extends the definition",` +
		`"changes":{"definition":"Converges eventually, with no read-your-writes guarantee.","properties":["eventual convergence"]}}`

	// When parsing it
	classification, err := parseReconciliation(raw, []domainknowledge.Item{target})

	// Then the target's current UpdatedAt is captured as the staleness token,
	// and only the fields the response set are present in Changes
	require.NoError(t, err)
	assert.Equal(t, domainknowledge.ReconcileUpdate, classification.Action)
	assert.Equal(t, "item-target", classification.TargetItemID)
	assert.Equal(t, target.UpdatedAt, classification.TargetUpdatedAt)
	require.NotNil(t, classification.Changes.Definition)
	assert.Equal(t, "Converges eventually, with no read-your-writes guarantee.", *classification.Changes.Definition)
	assert.Equal(t, []string{"eventual convergence"}, classification.Changes.Properties)
	assert.Nil(t, classification.Changes.TradeOffs)
	assert.Nil(t, classification.Changes.RelatedConcepts)
}

func TestBuildReconciliationPrompt_rendersPropertiesTradeOffsAndRelatedConceptsWhenPresent(t *testing.T) {
	// Given a candidate and a target that both carry every optional field
	candidate := domainknowledge.Item{
		Concept: "Idempotency key", Definition: "Retries produce the same effect.",
		Properties: []string{"deterministic"}, TradeOffs: []string{"extra storage"}, RelatedConcepts: []string{"exactly-once delivery"},
	}
	target := domainknowledge.Item{
		ID: "item-target", Status: domainknowledge.StatusApproved,
		Concept: "Idempotency key", Definition: "Retries produce the same effect.",
		Properties: []string{"deterministic"}, TradeOffs: []string{"extra storage"}, RelatedConcepts: []string{"exactly-once delivery"},
	}

	// When building the comparison prompt
	prompt := buildReconciliationPrompt(candidate, []domainknowledge.Item{target})

	// Then every optional field renders for both the candidate and the target
	assert.Contains(t, prompt, "Properties: deterministic")
	assert.Contains(t, prompt, "Trade-offs: extra storage")
	assert.Contains(t, prompt, "Related concepts: exactly-once delivery")
	assert.Contains(t, prompt, "[id:item-target status:approved]")
}

func TestBuildReconciliationPrompt_omitsPropertiesTradeOffsAndRelatedConceptsWhenAbsent(t *testing.T) {
	// Given a candidate and a target with none of the optional fields set
	candidate := domainknowledge.Item{Concept: "Idempotency key", Definition: "Retries produce the same effect."}
	target := domainknowledge.Item{ID: "item-target", Status: domainknowledge.StatusApproved, Concept: "Idempotency key", Definition: "Retries produce the same effect."}

	// When building the comparison prompt
	prompt := buildReconciliationPrompt(candidate, []domainknowledge.Item{target})

	// Then none of the optional field labels render at all — not even empty
	assert.NotContains(t, prompt, "Properties:")
	assert.NotContains(t, prompt, "Trade-offs:")
	assert.NotContains(t, prompt, "Related concepts:")
}

func TestClassifyCandidates_skipsTheLLMWhenACandidateHasNoDuplicateShortlist(t *testing.T) {
	// Given a candidate with an empty duplicate shortlist
	ctx := context.Background()
	service := NewService(nil, nil, nil, llmmocks.NewMockProvider(t), nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, nil, nil, 0, 0)
	results := []ExtractionCandidate{{Item: domainknowledge.Item{ID: "candidate-1", Concept: "Idempotency key"}}}

	// When classifying it
	classifications := service.classifyCandidates(ctx, results)

	// Then it gets a deterministic create classification, and the suggestion
	// is attached to the result for the frontend, without any LLM call — the
	// mock has no .EXPECT() for Chat, so mockery fails the test if it is called
	require.Len(t, classifications, 1)
	assert.Equal(t, domainknowledge.ReconcileCreate, classifications[0].Action)
	require.NotNil(t, results[0].Reconciliation)
	assert.Equal(t, domainknowledge.ReconcileCreate, results[0].Reconciliation.Action)
	assert.False(t, results[0].ReconciliationFailed)
}

func TestClassifyCandidates_fallsBackToCreateAndFlagsFailureWhenTheComparisonCallFails(t *testing.T) {
	// Given a candidate with a duplicate shortlist, whose classification LLM
	// call fails
	ctx := context.Background()
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().GetByID(ctx, "item-target").Return(reconciliationTarget(), nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(ctx, mock.Anything).Return(domainllm.ChatResponse{}, errors.New("openrouter: unavailable")).Once()
	service := NewService(repo, nil, nil, llm, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	results := []ExtractionCandidate{{
		Item: domainknowledge.Item{ID: "candidate-1", Topic: "Distributed Systems", Concept: "Eventual consistency"},
		Duplicates: []domainknowledge.DuplicateMatch{
			{ItemID: "item-target", Concept: "Eventual consistency", Status: domainknowledge.StatusApproved, MatchType: domainknowledge.MatchExact, Score: 1},
		},
	}}

	// When classifying it
	classifications := service.classifyCandidates(ctx, results)

	// Then the candidate still gets a usable create classification, but the
	// result is flagged so the caller can tell the comparison itself failed
	require.Len(t, classifications, 1)
	assert.Equal(t, domainknowledge.ReconcileCreate, classifications[0].Action)
	assert.True(t, results[0].ReconciliationFailed)
	require.NotNil(t, results[0].Reconciliation)
	assert.Equal(t, domainknowledge.ReconcileCreate, results[0].Reconciliation.Action)
}

// reconciliationReceiptFixture seeds a backend receipt exactly as
// ExtractFromSession/classifyCandidates would have, without going through
// the LLM — Apply/Resolve/Acknowledge/SaveForReview tests only care about
// what they do with a receipt (and its classification) already on file.
func reconciliationReceiptFixture(
	service *Service, sessionID, sourceLabel, candidateID string,
	classification reconciliationClassification, refs ...domainknowledge.EvidenceRef,
) string {
	return service.receipts.Create(sessionID, sourceLabel, []parsedCandidate{
		{Item: domainknowledge.Item{ID: candidateID}, EvidenceRefs: refs},
	}, []reconciliationClassification{classification})
}

func reconciliationCandidateContent() domainknowledge.Item {
	return domainknowledge.Item{
		Topic: "Distributed Systems", Concept: "Idempotency key",
		Definition: "A unique value a client attaches to a request so retries produce the same effect exactly once.",
		Source:     domainknowledge.SourceAthena, Status: domainknowledge.StatusDraft,
	}
}

func TestApplyReconciliationCreate_persistsANewItemAndTheAuditProposalWithEvidence(t *testing.T) {
	// Given a classified create candidate backed by a valid receipt
	ctx := context.Background()
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().FindByNormalizedConcept(ctx, "Distributed Systems", "idempotency key").Return(nil, nil).Once()
	repo.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID != "candidate-1" && item.ID != "" && item.Topic == "Distributed Systems" &&
			item.Concept == "Idempotency key" && item.Status == domainknowledge.StatusDraft
	})).Return(nil).Once()
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "An idempotency key lets retries be safe."},
	}, nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.MatchedBy(func(e domainknowledge.Evidence) bool {
		return e.OriginID == "message-1" && e.Excerpt == "An idempotency key lets retries be safe."
	})).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().Save(ctx, mock.MatchedBy(func(p domainknowledge.ReconciliationProposal) bool {
		return p.Action == domainknowledge.ReconcileCreate && p.Status == domainknowledge.ProposalApplied &&
			p.TargetItemID == "" && p.Reason == "no existing match found in this topic"
	})).Return(nil).Once()
	reconciliations.EXPECT().LinkEvidence(ctx, mock.Anything, "evidence-1").Return(nil).Once()
	llm := llmmocks.NewMockProvider(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	tx := txmocks.NewMockTransactor(t)
	expectSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	service := NewService(repo, studymocks.NewMockSessionRepository(t), messages, llm, configmocks.NewMockStore(t), chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo, reconciliations, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	batchID := reconciliationReceiptFixture(service, "session-1", "Distributed Systems", "candidate-1",
		deterministicCreateClassification("no existing match found in this topic"),
		domainknowledge.EvidenceRef{MessageID: "message-1", Quote: "An idempotency key lets retries be safe."})

	// When applying create
	item, err := service.ApplyReconciliationCreate(ctx, batchID, "candidate-1", reconciliationCandidateContent(), domainknowledge.StatusDraft)

	// Then a brand-new item is persisted and the receipt is consumed
	require.NoError(t, err)
	assert.NotEmpty(t, item.ID)
	assert.NotEqual(t, "candidate-1", item.ID)
	_, found := service.receipts.Get(batchID, "candidate-1")
	assert.False(t, found)
}

func TestApplyReconciliationCreate_restoresTheReceiptWhenTheExactDuplicateRecheckFindsAMatch(t *testing.T) {
	// Given a create candidate whose exact match appeared between
	// classification and apply
	ctx := context.Background()
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().FindByNormalizedConcept(ctx, "Distributed Systems", "idempotency key").
		Return([]domainknowledge.Item{{ID: "item-existing", Concept: "Idempotency key", Status: domainknowledge.StatusApproved}}, nil).Once()
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "An idempotency key lets retries be safe."},
	}, nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repo, studymocks.NewMockSessionRepository(t), messages, llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), nil, tx, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, nil, nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	batchID := reconciliationReceiptFixture(service, "session-1", "Distributed Systems", "candidate-1",
		deterministicCreateClassification("no existing match found in this topic"),
		domainknowledge.EvidenceRef{MessageID: "message-1", Quote: "An idempotency key lets retries be safe."})

	// When applying create
	_, err := service.ApplyReconciliationCreate(ctx, batchID, "candidate-1", reconciliationCandidateContent(), domainknowledge.StatusDraft)

	// Then it fails and the receipt is restored for retry, not silently lost
	require.Error(t, err)
	_, found := service.receipts.Get(batchID, "candidate-1")
	assert.True(t, found)
}

func TestApplyReconciliationCreate_returnsIndexingFailureButKeepsTheDurableItem(t *testing.T) {
	// Given a create candidate whose item persists successfully but whose
	// post-commit embedding call fails
	ctx := context.Background()
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().FindByNormalizedConcept(ctx, "Distributed Systems", "idempotency key").Return(nil, nil).Once()
	repo.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "An idempotency key lets retries be safe."},
	}, nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.Anything).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	reconciliations.EXPECT().LinkEvidence(ctx, mock.Anything, "evidence-1").Return(nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, mock.Anything).Return(domainllm.EmbeddingResponse{}, errors.New("openrouter: unavailable")).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repo, studymocks.NewMockSessionRepository(t), messages, llm, configmocks.NewMockStore(t), nil, tx, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo, reconciliations, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	batchID := reconciliationReceiptFixture(service, "session-1", "Distributed Systems", "candidate-1",
		deterministicCreateClassification("no existing match found in this topic"),
		domainknowledge.EvidenceRef{MessageID: "message-1", Quote: "An idempotency key lets retries be safe."})

	// When applying create
	item, err := service.ApplyReconciliationCreate(ctx, batchID, "candidate-1", reconciliationCandidateContent(), domainknowledge.StatusDraft)

	// Then the durable write is not reported as a failure — the caller gets
	// the real, persisted item alongside a typed indexing failure, and the
	// receipt stays consumed rather than retried
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrIndexingFailed)
	assert.NotEmpty(t, item.ID)
	_, found := service.receipts.Get(batchID, "candidate-1")
	assert.False(t, found)
}

func TestApplyReconciliationCreate_restoresTheReceiptWhenLinkingEvidenceFails(t *testing.T) {
	// Given a create candidate whose item persists but whose evidence link fails
	ctx := context.Background()
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().FindByNormalizedConcept(ctx, "Distributed Systems", "idempotency key").Return(nil, nil).Once()
	repo.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "An idempotency key lets retries be safe."},
	}, nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.Anything).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	reconciliations.EXPECT().LinkEvidence(ctx, mock.Anything, "evidence-1").Return(errors.New("disk full")).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repo, studymocks.NewMockSessionRepository(t), messages, llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), nil, tx, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo, reconciliations, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	batchID := reconciliationReceiptFixture(service, "session-1", "Distributed Systems", "candidate-1",
		deterministicCreateClassification("no existing match found in this topic"),
		domainknowledge.EvidenceRef{MessageID: "message-1", Quote: "An idempotency key lets retries be safe."})

	// When applying create
	_, err := service.ApplyReconciliationCreate(ctx, batchID, "candidate-1", reconciliationCandidateContent(), domainknowledge.StatusDraft)

	// Then it fails and the receipt is restored for retry
	require.Error(t, err)
	_, found := service.receipts.Get(batchID, "candidate-1")
	assert.True(t, found)
}

func TestApplyReconciliationUpdate_changesOnlyTheReviewedFieldsAndKeepsIdentityAndLifecycle(t *testing.T) {
	// Given a classified update candidate targeting an existing approved item
	ctx := context.Background()
	target := domainknowledge.Item{
		ID: "item-target", Topic: "Distributed Systems", Concept: "Eventual consistency",
		Definition: "Converges eventually.", Properties: []string{"weak consistency"},
		TradeOffs: []string{"stale reads possible"}, RelatedConcepts: []string{"CAP theorem"},
		Source: domainknowledge.SourceAthena, Status: domainknowledge.StatusApproved,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC),
	}
	newDefinition := "Converges eventually, with no read-your-writes guarantee."
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().GetByID(ctx, "item-target").Return(target, nil).Once()
	repo.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID == "item-target" && item.Topic == "Distributed Systems" &&
			item.Source == domainknowledge.SourceAthena && item.Status == domainknowledge.StatusApproved &&
			item.CreatedAt.Equal(target.CreatedAt) && item.Definition == newDefinition &&
			// Properties/TradeOffs/RelatedConcepts were not in Changes, so
			// they all stay untouched
			len(item.Properties) == 1 && item.Properties[0] == "weak consistency" &&
			len(item.TradeOffs) == 1 && item.TradeOffs[0] == "stale reads possible" &&
			len(item.RelatedConcepts) == 1 && item.RelatedConcepts[0] == "CAP theorem" &&
			item.UpdatedAt.After(target.UpdatedAt)
	})).Return(nil).Once()
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "Reads may lag briefly after a write."},
	}, nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.Anything).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().Save(ctx, mock.MatchedBy(func(p domainknowledge.ReconciliationProposal) bool {
		return p.Action == domainknowledge.ReconcileUpdate && p.TargetItemID == "item-target" &&
			p.Status == domainknowledge.ProposalApplied
	})).Return(nil).Once()
	reconciliations.EXPECT().LinkEvidence(ctx, mock.Anything, "evidence-1").Return(nil).Once()
	llm := llmmocks.NewMockProvider(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	tx := txmocks.NewMockTransactor(t)
	expectSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	service := NewService(repo, studymocks.NewMockSessionRepository(t), messages, llm, configmocks.NewMockStore(t), chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo, reconciliations, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	classification := reconciliationClassification{
		Action: domainknowledge.ReconcileUpdate, TargetItemID: "item-target", TargetUpdatedAt: target.UpdatedAt,
		Reason: "extends the definition", Changes: domainknowledge.ItemChanges{Definition: &newDefinition},
	}
	batchID := reconciliationReceiptFixture(service, "session-1", "Distributed Systems", "candidate-1", classification,
		domainknowledge.EvidenceRef{MessageID: "message-1", Quote: "Reads may lag briefly after a write."})

	// When applying update
	item, err := service.ApplyReconciliationUpdate(ctx, batchID, "candidate-1", reconciliationCandidateContent())

	// Then the target's identity and lifecycle survive, only the reviewed
	// field changed
	require.NoError(t, err)
	assert.Equal(t, "item-target", item.ID)
	assert.Equal(t, newDefinition, item.Definition)
}

func TestApplyReconciliationUpdate_restoresTheReceiptWhenTheTargetChangedSinceClassification(t *testing.T) {
	// Given a classified update whose target was edited after classification
	// captured its UpdatedAt
	ctx := context.Background()
	classifiedAt := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	currentTarget := domainknowledge.Item{ID: "item-target", UpdatedAt: classifiedAt.Add(time.Hour)}
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().GetByID(ctx, "item-target").Return(currentTarget, nil).Once()
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "Reads may lag briefly after a write."},
	}, nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repo, studymocks.NewMockSessionRepository(t), messages, llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), nil, tx, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, nil, nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	classification := reconciliationClassification{
		Action: domainknowledge.ReconcileUpdate, TargetItemID: "item-target", TargetUpdatedAt: classifiedAt, Reason: "extends",
	}
	batchID := reconciliationReceiptFixture(service, "session-1", "Distributed Systems", "candidate-1", classification,
		domainknowledge.EvidenceRef{MessageID: "message-1", Quote: "Reads may lag briefly after a write."})

	// When applying update
	_, err := service.ApplyReconciliationUpdate(ctx, batchID, "candidate-1", reconciliationCandidateContent())

	// Then it is refused as stale and the receipt is restored for retry
	assert.ErrorIs(t, err, ErrReconciliationTargetStale)
	_, found := service.receipts.Get(batchID, "candidate-1")
	assert.True(t, found)
}

func TestApplyReconciliationRelate_createsANewDraftAndACanonicalRelationToTheTarget(t *testing.T) {
	// Given a classified relate candidate against an existing target
	ctx := context.Background()
	target := domainknowledge.Item{ID: "item-target", UpdatedAt: time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)}
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().GetByID(ctx, "item-target").Return(target, nil).Once()
	repo.EXPECT().FindByNormalizedConcept(ctx, "Distributed Systems", "idempotency key").Return(nil, nil).Once()
	var createdItemID string
	repo.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		createdItemID = item.ID
		return item.Status == domainknowledge.StatusDraft
	})).Return(nil).Once()
	relations := knowledgemocks.NewMockRelationRepository(t)
	relations.EXPECT().Save(ctx, mock.MatchedBy(func(r domainknowledge.Relation) bool {
		return r.Type == domainknowledge.RelationRelated &&
			(r.FromItemID == "item-target" || r.ToItemID == "item-target") &&
			(r.FromItemID == createdItemID || r.ToItemID == createdItemID)
	})).Return(nil).Once()
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "An idempotency key lets retries be safe."},
	}, nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.Anything).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().Save(ctx, mock.MatchedBy(func(p domainknowledge.ReconciliationProposal) bool {
		return p.Action == domainknowledge.ReconcileRelate && p.TargetItemID == "item-target"
	})).Return(nil).Once()
	reconciliations.EXPECT().LinkEvidence(ctx, mock.Anything, "evidence-1").Return(nil).Once()
	llm := llmmocks.NewMockProvider(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	tx := txmocks.NewMockTransactor(t)
	expectSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	service := NewService(repo, studymocks.NewMockSessionRepository(t), messages, llm, configmocks.NewMockStore(t), chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo, reconciliations, relations, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	classification := reconciliationClassification{
		Action: domainknowledge.ReconcileRelate, TargetItemID: "item-target", TargetUpdatedAt: target.UpdatedAt,
		Reason: "distinct but connected concept",
	}
	batchID := reconciliationReceiptFixture(service, "session-1", "Distributed Systems", "candidate-1", classification,
		domainknowledge.EvidenceRef{MessageID: "message-1", Quote: "An idempotency key lets retries be safe."})

	// When applying relate
	item, err := service.ApplyReconciliationRelate(ctx, batchID, "candidate-1", reconciliationCandidateContent())

	// Then a new draft item is created, distinct from the target
	require.NoError(t, err)
	assert.NotEqual(t, "item-target", item.ID)
	assert.Equal(t, domainknowledge.StatusDraft, item.Status)
}

func TestResolveReconciliationConflict_keepExistingAppliesNoItemMutation(t *testing.T) {
	// Given a classified conflict candidate
	ctx := context.Background()
	target := domainknowledge.Item{ID: "item-target", UpdatedAt: time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)}
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().GetByID(ctx, "item-target").Return(target, nil).Once()
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "The circuit opens after N consecutive failures."},
	}, nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.Anything).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().Save(ctx, mock.MatchedBy(func(p domainknowledge.ReconciliationProposal) bool {
		return p.Action == domainknowledge.ReconcileConflict && p.Status == domainknowledge.ProposalApplied &&
			p.Reason == "criteria disagree; resolved: kept existing item"
	})).Return(nil).Once()
	reconciliations.EXPECT().LinkEvidence(ctx, mock.Anything, "evidence-1").Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repo, studymocks.NewMockSessionRepository(t), messages, llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), nil, tx, nil, nil, domainknowledge.RetrievalThresholds{}, evidenceRepo, reconciliations, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	classification := reconciliationClassification{
		Action: domainknowledge.ReconcileConflict, TargetItemID: "item-target", TargetUpdatedAt: target.UpdatedAt,
		Reason: "criteria disagree",
	}
	batchID := reconciliationReceiptFixture(service, "session-1", "Distributed Systems", "candidate-1", classification,
		domainknowledge.EvidenceRef{MessageID: "message-1", Quote: "The circuit opens after N consecutive failures."})

	// When resolving the conflict by keeping the existing item
	item, err := service.ResolveReconciliationConflict(ctx, batchID, "candidate-1", reconciliationCandidateContent(), ConflictKeepExisting)

	// Then no item is created or changed — repo has no .EXPECT() for
	// Save/Update, so mockery fails the test if either was called
	require.NoError(t, err)
	assert.Empty(t, item.ID)
}

func TestResolveReconciliationConflict_rejectsAnUnknownResolution(t *testing.T) {
	// Given a classified conflict candidate
	ctx := context.Background()
	service := NewService(nil, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, nil, nil, 0, 0)
	batchID := reconciliationReceiptFixture(service, "session-1", "Distributed Systems", "candidate-1",
		reconciliationClassification{Action: domainknowledge.ReconcileConflict, TargetItemID: "item-target", Reason: "criteria disagree"})

	// When resolving it with an unrecognized outcome
	_, err := service.ResolveReconciliationConflict(ctx, batchID, "candidate-1", reconciliationCandidateContent(), "discard_everything")

	// Then it is rejected without touching the receipt's claim state
	assert.ErrorIs(t, err, ErrReconciliationResolutionInvalid)
}

func TestAcknowledgeReconciliationNoChange_persistsTheAuditRecordWithoutTouchingAnyItem(t *testing.T) {
	// Given a classified no_change candidate
	ctx := context.Background()
	target := domainknowledge.Item{ID: "item-target", UpdatedAt: time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)}
	repo := knowledgemocks.NewMockRepository(t)
	repo.EXPECT().GetByID(ctx, "item-target").Return(target, nil).Once()
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "Delivery may happen more than once."},
	}, nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.Anything).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().Save(ctx, mock.MatchedBy(func(p domainknowledge.ReconciliationProposal) bool {
		return p.Action == domainknowledge.ReconcileNoChange && p.Status == domainknowledge.ProposalApplied
	})).Return(nil).Once()
	reconciliations.EXPECT().LinkEvidence(ctx, mock.Anything, "evidence-1").Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repo, studymocks.NewMockSessionRepository(t), messages, llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), nil, tx, nil, nil, domainknowledge.RetrievalThresholds{}, evidenceRepo, reconciliations, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	classification := reconciliationClassification{
		Action: domainknowledge.ReconcileNoChange, TargetItemID: "item-target", TargetUpdatedAt: target.UpdatedAt,
		Reason: "already captured",
	}
	batchID := reconciliationReceiptFixture(service, "session-1", "Distributed Systems", "candidate-1", classification,
		domainknowledge.EvidenceRef{MessageID: "message-1", Quote: "Delivery may happen more than once."})

	// When acknowledging no_change
	err := service.AcknowledgeReconciliationNoChange(ctx, batchID, "candidate-1", reconciliationCandidateContent())

	// Then the receipt is consumed — repo has no .EXPECT() for Save/Update
	require.NoError(t, err)
	_, found := service.receipts.Get(batchID, "candidate-1")
	assert.False(t, found)
}

func TestSaveReconciliationForReview_persistsAPendingProposalWithoutCheckingStaleness(t *testing.T) {
	// Given a classified update candidate whose target has since changed —
	// staleness would block Apply, but must not block a save for later review
	ctx := context.Background()
	classifiedAt := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "Reads may lag briefly after a write."},
	}, nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.Anything).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	reconciliations := knowledgemocks.NewMockReconciliationRepository(t)
	reconciliations.EXPECT().Save(ctx, mock.MatchedBy(func(p domainknowledge.ReconciliationProposal) bool {
		return p.Status == domainknowledge.ProposalPending && p.Action == domainknowledge.ReconcileUpdate
	})).Return(nil).Once()
	reconciliations.EXPECT().LinkEvidence(ctx, mock.Anything, "evidence-1").Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	// repo has no .EXPECT() for GetByID — proving no staleness re-check happens
	service := NewService(knowledgemocks.NewMockRepository(t), studymocks.NewMockSessionRepository(t), messages, llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), nil, tx, nil, nil, domainknowledge.RetrievalThresholds{}, evidenceRepo, reconciliations, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	classification := reconciliationClassification{
		Action: domainknowledge.ReconcileUpdate, TargetItemID: "item-target", TargetUpdatedAt: classifiedAt, Reason: "extends",
	}
	batchID := reconciliationReceiptFixture(service, "session-1", "Distributed Systems", "candidate-1", classification,
		domainknowledge.EvidenceRef{MessageID: "message-1", Quote: "Reads may lag briefly after a write."})

	// When saving it for review
	err := service.SaveReconciliationForReview(ctx, batchID, "candidate-1", reconciliationCandidateContent())

	// Then it succeeds, unconditionally
	require.NoError(t, err)
}

func TestApplyReconciliationUpdate_restoresTheReceiptWhenEvidenceIsNoLongerValid(t *testing.T) {
	// Given a classified update whose only evidence quote no longer appears
	// in its source message
	ctx := context.Background()
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "Completely rewritten content."},
	}, nil).Once()
	service := NewService(knowledgemocks.NewMockRepository(t), studymocks.NewMockSessionRepository(t), messages, llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), nil, nil, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, nil, nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	classification := reconciliationClassification{
		Action: domainknowledge.ReconcileUpdate, TargetItemID: "item-target", Reason: "extends",
	}
	batchID := reconciliationReceiptFixture(service, "session-1", "Distributed Systems", "candidate-1", classification,
		domainknowledge.EvidenceRef{MessageID: "message-1", Quote: "Reads may lag briefly after a write."})

	// When applying update
	_, err := service.ApplyReconciliationUpdate(ctx, batchID, "candidate-1", reconciliationCandidateContent())

	// Then it is refused, and the receipt is restored for retry
	assert.ErrorIs(t, err, ErrReconciliationEvidenceInvalid)
	_, found := service.receipts.Get(batchID, "candidate-1")
	assert.True(t, found)
}

func TestApplyReconciliationCreate_returnsNotFoundForAnAlreadyDecidedCandidate(t *testing.T) {
	// Given a batch with no receipt for the candidate — already decided, or
	// never classified
	ctx := context.Background()
	service := NewService(nil, nil, nil, nil, nil, nil, nil, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, nil, nil, nil, 0, 0)

	// When applying create for it anyway
	_, err := service.ApplyReconciliationCreate(ctx, "missing-batch", "candidate-1", reconciliationCandidateContent(), domainknowledge.StatusDraft)

	// Then it is rejected without panicking
	assert.ErrorIs(t, err, ErrReconciliationCandidateNotFound)
}
