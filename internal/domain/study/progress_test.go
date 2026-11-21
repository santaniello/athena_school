package study

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func progressConcept(name string, status ConceptStatus, age time.Duration) ConceptState {
	return ConceptState{
		SessionID: "session-1", Name: name, Status: status,
		AssistanceCount: 2, EvidenceCount: 3, UpdatedAt: stateNow.Add(-age),
	}
}

func progressNames(concepts []ProgressConcept) []string {
	names := make([]string, 0, len(concepts))
	for _, concept := range concepts {
		names = append(names, concept.Name)
	}
	return names
}

func TestBuildSessionProgress_groupsConceptsByStatus(t *testing.T) {
	// Given one concept in each stored status
	concepts := []ConceptState{
		progressConcept("explained", StatusIntroduced, 0),
		progressConcept("shown", StatusDemonstrated, 0),
		progressConcept("gap", StatusPartiallyUnderstood, 0),
		progressConcept("again", StatusNeedsReview, 0),
	}

	// When building the progress
	progress := BuildSessionProgress(concepts, nil)

	// Then each lands in its own group: demonstrated apart from only explained
	assert.Equal(t, []string{"shown"}, progressNames(progress.Demonstrated))
	assert.Equal(t, []string{"explained"}, progressNames(progress.ExplainedOnly))
	assert.Equal(t, []string{"gap"}, progressNames(progress.Difficulties))
	assert.Equal(t, []string{"again"}, progressNames(progress.NeedsReview))
}

func TestBuildSessionProgress_carriesTheConceptsCounters(t *testing.T) {
	// Given a demonstrated concept with counters
	// When building the progress
	progress := BuildSessionProgress([]ConceptState{progressConcept("shown", StatusDemonstrated, 0)}, nil)

	// Then its status, counters and update time are in the entry
	require.Len(t, progress.Demonstrated, 1)
	entry := progress.Demonstrated[0]
	assert.Equal(t, StatusDemonstrated, entry.Status)
	assert.Equal(t, 2, entry.AssistanceCount)
	assert.Equal(t, 3, entry.EvidenceCount)
	assert.Equal(t, stateNow, entry.UpdatedAt)
}

func TestBuildSessionProgress_ordersEachGroupByMostRecentUpdate(t *testing.T) {
	// Given three demonstrated concepts, two with the same update time
	concepts := []ConceptState{
		progressConcept("old", StatusDemonstrated, 2*time.Hour),
		progressConcept("b same", StatusDemonstrated, time.Hour),
		progressConcept("a same", StatusDemonstrated, time.Hour),
	}

	// When building the progress
	progress := BuildSessionProgress(concepts, nil)

	// Then the most recent come first, ties by name
	assert.Equal(t, []string{"a same", "b same", "old"}, progressNames(progress.Demonstrated))
}

func TestBuildSessionProgress_attachesEachConceptsProofs_newestFirst(t *testing.T) {
	// Given proofs of two concepts, out of order
	concepts := []ConceptState{
		progressConcept("mutex", StatusDemonstrated, 0),
		progressConcept("channel", StatusDemonstrated, time.Hour),
	}
	proof := func(concept, description string, age time.Duration) Evidence {
		return Evidence{SessionID: "session-1", Concept: concept, Description: description, CreatedAt: stateNow.Add(-age)}
	}
	evidence := []Evidence{
		proof("mutex", "first", 3*time.Hour),
		proof("channel", "other", time.Hour),
		proof("mutex", "third", time.Hour),
		proof("mutex", "second", 2*time.Hour),
	}

	// When building the progress
	progress := BuildSessionProgress(concepts, evidence)

	// Then each concept carries only its own proofs, the newest first
	require.Len(t, progress.Demonstrated, 2)
	mutex, channel := progress.Demonstrated[0], progress.Demonstrated[1]
	assert.Equal(t, "mutex", mutex.Name)
	assert.Equal(t, []string{"third", "second", "first"}, proofDescriptions(mutex.Evidence))
	assert.Equal(t, []string{"other"}, proofDescriptions(channel.Evidence))
}

func proofDescriptions(proofs []Evidence) []string {
	descriptions := make([]string, 0, len(proofs))
	for _, proof := range proofs {
		descriptions = append(descriptions, proof.Description)
	}
	return descriptions
}

func TestBuildSessionProgress_ignoresProofsOfUnknownConcepts(t *testing.T) {
	// Given a proof whose concept has no row
	evidence := []Evidence{{Concept: "ghost", Description: "orphan"}}

	// When building the progress
	progress := BuildSessionProgress([]ConceptState{progressConcept("mutex", StatusIntroduced, 0)}, evidence)

	// Then it is attached to nothing
	require.Len(t, progress.ExplainedOnly, 1)
	assert.Empty(t, progress.ExplainedOnly[0].Evidence)
}

func TestBuildSessionProgress_withNothing_returnsEmptyGroupsNotNil(t *testing.T) {
	// Given no concepts
	// When building the progress
	progress := BuildSessionProgress(nil, nil)

	// Then every group is an empty, non-nil slice (it is serialized to the UI as [])
	assert.NotNil(t, progress.Demonstrated)
	assert.NotNil(t, progress.ExplainedOnly)
	assert.NotNil(t, progress.Difficulties)
	assert.NotNil(t, progress.NeedsReview)
	assert.True(t, progress.IsEmpty())
}

func TestSessionProgress_IsEmpty_falseWithAnyConcept(t *testing.T) {
	// Given a progress with one concept
	progress := BuildSessionProgress([]ConceptState{progressConcept("x", StatusNeedsReview, 0)}, nil)

	// When asking whether it is empty
	// Then it is not
	assert.False(t, progress.IsEmpty())
}

func TestBuildSessionProgress_skipsNotAssessedConcepts(t *testing.T) {
	// Given a concept stored as NOT_ASSESSED (never written today, but the status exists)
	// When building the progress
	progress := BuildSessionProgress([]ConceptState{progressConcept("x", StatusNotAssessed, 0)}, nil)

	// Then it appears in no group
	assert.True(t, progress.IsEmpty())
}
