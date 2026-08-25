package knowledge

import (
	"context"
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

// runWithinTx makes the mocked Transactor behave like the real one: it
// just invokes fn immediately against ctx, so the repo mocks set up
// underneath faithfully observe every call Approve/Deprecate/UpdateItem/
// DeleteItem make inside their transactional read-modify-write.
func runWithinTx(tx *txmocks.MockTransactor) {
	tx.EXPECT().WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
}

// passingIndexGuard returns an IndexGuard mock that always allows the
// mutation — the default for every test that isn't specifically about the
// guard rejecting one.
func passingIndexGuard(t *testing.T) *txmocks.MockIndexGuard {
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(nil)
	guard.EXPECT().EndMutation()
	return guard
}

// expectSuccessfulIndexing wires llm/chunks/store/tx mocks so every call
// indexKnowledgeItem makes succeeds, `times` times over — used by tests
// that only care indexing happened, not what content got embedded.
func expectSuccessfulIndexing(
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
	runWithinTx(tx)
}

func TestReconcileContext_deadlineIsBoundedAroundFiveSeconds(t *testing.T) {
	// When building a reconciliation context
	before := time.Now()
	ctx, cancel := reconcileContext()
	defer cancel()

	// Then it is still open and its deadline is bounded close to 5s out —
	// a mutated timeout (e.g. 0 or a wildly different duration) would fail
	// either the immediate-cancellation check or the bound below
	require.NoError(t, ctx.Err())
	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	assert.WithinDuration(t, before.Add(5*time.Second), deadline, 500*time.Millisecond)
}
