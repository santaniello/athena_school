package desktop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationknowledge "github.com/santaniello/athena/internal/application/knowledge"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// unreachableReconciliationApp builds an App backed by a knowledge service
// with every dependency nil — any of its methods that actually reaches the
// application layer panics on a nil-pointer dereference, so a test that
// completes without panicking proves the desktop binding's own input
// validation returned before ever calling the use case, keeping these
// Wails bindings thin adapters per AGENTS.md.
func unreachableReconciliationApp(t *testing.T) *App {
	t.Helper()
	service := applicationknowledge.NewService(nil, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, nil, nil, 0, 0)
	app := NewApp(nil, nil, nil, nil, nil, service, nil, nil, nil, nil)
	app.Startup(context.Background())
	return app
}

func TestApp_ApplyPendingReconciliationCreate_rejectsEmptyIDAndInvalidStatus(t *testing.T) {
	app := unreachableReconciliationApp(t)

	_, err := app.ApplyPendingReconciliationCreate("", domainknowledge.StatusDraft)
	assert.ErrorIs(t, err, ErrReconciliationProposalIDRequired)

	_, err = app.ApplyPendingReconciliationCreate("proposal-1", "published")
	assert.ErrorIs(t, err, ErrReconciliationStatusInvalid)
}

func TestApp_ApplyPendingReconciliationUpdate_rejectsEmptyID(t *testing.T) {
	app := unreachableReconciliationApp(t)

	_, err := app.ApplyPendingReconciliationUpdate("")
	assert.ErrorIs(t, err, ErrReconciliationProposalIDRequired)
}

func TestApp_ApplyPendingReconciliationRelate_rejectsEmptyID(t *testing.T) {
	app := unreachableReconciliationApp(t)

	_, err := app.ApplyPendingReconciliationRelate("")
	assert.ErrorIs(t, err, ErrReconciliationProposalIDRequired)
}

func TestApp_ResolvePendingReconciliationConflict_rejectsEmptyIDAndInvalidResolution(t *testing.T) {
	app := unreachableReconciliationApp(t)

	_, err := app.ResolvePendingReconciliationConflict("", applicationknowledge.ConflictKeepExisting)
	assert.ErrorIs(t, err, ErrReconciliationProposalIDRequired)

	_, err = app.ResolvePendingReconciliationConflict("proposal-1", "discard")
	assert.ErrorIs(t, err, ErrReconciliationResolutionInvalid)
}

func TestApp_AcknowledgePendingReconciliationNoChange_rejectsEmptyID(t *testing.T) {
	app := unreachableReconciliationApp(t)

	err := app.AcknowledgePendingReconciliationNoChange("")
	assert.ErrorIs(t, err, ErrReconciliationProposalIDRequired)
}

func TestApp_RejectPendingReconciliationProposal_rejectsEmptyID(t *testing.T) {
	app := unreachableReconciliationApp(t)

	err := app.RejectPendingReconciliationProposal("")
	require.ErrorIs(t, err, ErrReconciliationProposalIDRequired)
}
