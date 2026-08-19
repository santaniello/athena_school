package knowledge

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransitionTo_succeeds_fromDraftToApproved(t *testing.T) {
	// Given a draft item
	item := Item{Status: StatusDraft}
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)

	// When transitioning to approved
	next, err := item.TransitionTo(StatusApproved, now)

	// Then it succeeds and stamps the new status and UpdatedAt
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, next.Status)
	assert.Equal(t, now, next.UpdatedAt)
}

func TestTransitionTo_succeeds_fromApprovedToDeprecated(t *testing.T) {
	// Given an approved item
	item := Item{Status: StatusApproved}
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)

	// When transitioning to deprecated
	next, err := item.TransitionTo(StatusDeprecated, now)

	// Then it succeeds and stamps the new status and UpdatedAt
	require.NoError(t, err)
	assert.Equal(t, StatusDeprecated, next.Status)
	assert.Equal(t, now, next.UpdatedAt)
}

func TestTransitionTo_returnsErrInvalidStatusTransition_fromDraftToDeprecated(t *testing.T) {
	// Given a draft item
	item := Item{Status: StatusDraft}

	// When transitioning directly to deprecated
	_, err := item.TransitionTo(StatusDeprecated, time.Now())

	// Then it is rejected as an invalid transition
	assert.ErrorIs(t, err, ErrInvalidStatusTransition)
}

func TestTransitionTo_returnsErrInvalidStatusTransition_fromDeprecatedToApproved(t *testing.T) {
	// Given a deprecated item
	item := Item{Status: StatusDeprecated}

	// When transitioning back to approved
	_, err := item.TransitionTo(StatusApproved, time.Now())

	// Then it is rejected as an invalid transition
	assert.ErrorIs(t, err, ErrInvalidStatusTransition)
}

func TestTransitionTo_returnsErrUnknownStatus_forAnUnrecognizedStatus(t *testing.T) {
	// Given a draft item
	item := Item{Status: StatusDraft}

	// When transitioning to a status that doesn't exist
	_, err := item.TransitionTo("archived", time.Now())

	// Then it is rejected as unknown
	assert.ErrorIs(t, err, ErrUnknownStatus)
}

func TestTransitionTo_leavesReceiverUnmodified(t *testing.T) {
	// Given a draft item
	item := Item{Status: StatusDraft}
	originalUpdatedAt := item.UpdatedAt

	// When transitioning to approved
	_, err := item.TransitionTo(StatusApproved, time.Now())

	// Then the original item is untouched
	require.NoError(t, err)
	assert.Equal(t, StatusDraft, item.Status)
	assert.Equal(t, originalUpdatedAt, item.UpdatedAt)
}

func TestItem_Validate_acceptsRequiredFields(t *testing.T) {
	// Given an item with every required field
	item := Item{Topic: "Distributed systems", Concept: "CAP theorem", Definition: "A consistency trade-off."}

	// When validating it
	err := item.Validate()

	// Then it is accepted
	require.NoError(t, err)
}

func TestItem_Validate_returnsTopicRequiredWhenBlank(t *testing.T) {
	// Given an item with a blank topic
	item := Item{Topic: "  ", Concept: "CAP theorem", Definition: "A consistency trade-off."}

	// When validating it
	err := item.Validate()

	// Then the topic error is returned
	assert.ErrorIs(t, err, ErrTopicRequired)
}

func TestItem_Validate_returnsConceptRequiredWhenBlank(t *testing.T) {
	// Given an item with a blank concept
	item := Item{Topic: "Distributed systems", Concept: "  ", Definition: "A consistency trade-off."}

	// When validating it
	err := item.Validate()

	// Then the concept error is returned
	assert.ErrorIs(t, err, ErrConceptRequired)
}

func TestItem_Validate_returnsDefinitionRequiredWhenBlank(t *testing.T) {
	// Given an item with a blank definition
	item := Item{Topic: "Distributed systems", Concept: "CAP theorem", Definition: "  "}

	// When validating it
	err := item.Validate()

	// Then the definition error is returned
	assert.ErrorIs(t, err, ErrDefinitionRequired)
}
