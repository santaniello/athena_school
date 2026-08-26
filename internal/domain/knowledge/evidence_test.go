package knowledge

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvidence_Validate_acceptsCompleteSnapshotFromEachKnownOrigin(t *testing.T) {
	for _, originType := range []string{OriginSessionMessage, OriginKnowledgeChunk} {
		t.Run(originType, func(t *testing.T) {
			// Given a complete Evidence snapshot from a known origin
			evidence := Evidence{
				ID:          "evidence-1",
				OriginType:  originType,
				OriginID:    "origin-1",
				SourceLabel: "Concurrency",
				Excerpt:     "A channel coordinates communication.",
				CreatedAt:   time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC),
			}

			// When validating it
			err := evidence.Validate()

			// Then it is accepted
			require.NoError(t, err)
		})
	}
}

func TestEvidence_Validate_rejectsUnknownOriginAndMissingSnapshotFields(t *testing.T) {
	// Given Evidence values missing each required persistence invariant
	valid := Evidence{
		ID:          "evidence-1",
		OriginType:  OriginSessionMessage,
		OriginID:    "message-1",
		SourceLabel: "Concurrency",
		Excerpt:     "A channel coordinates communication.",
		CreatedAt:   time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC),
	}
	tests := []struct {
		name     string
		mutate   func(*Evidence)
		expected error
	}{
		{name: "id", mutate: func(e *Evidence) { e.ID = " " }, expected: ErrEvidenceIDRequired},
		{name: "origin type", mutate: func(e *Evidence) { e.OriginType = "web_page" }, expected: ErrEvidenceOriginTypeInvalid},
		{name: "origin id", mutate: func(e *Evidence) { e.OriginID = " " }, expected: ErrEvidenceOriginIDRequired},
		{name: "source label", mutate: func(e *Evidence) { e.SourceLabel = " " }, expected: ErrEvidenceSourceLabelRequired},
		{name: "excerpt", mutate: func(e *Evidence) { e.Excerpt = " " }, expected: ErrEvidenceExcerptRequired},
		{name: "created at", mutate: func(e *Evidence) { e.CreatedAt = time.Time{} }, expected: ErrEvidenceCreatedAtRequired},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)

			// When validating the invalid snapshot
			err := candidate.Validate()

			// Then the matching domain error is returned
			assert.ErrorIs(t, err, test.expected)
		})
	}
}

func TestEvidenceRef_IsSupportedBy_reportsVerbatimContainment(t *testing.T) {
	ref := EvidenceRef{MessageID: "message-1", Quote: "A channel coordinates communication."}

	// A message whose content contains the quote verbatim supports it,
	// including with surrounding text
	assert.True(t, ref.IsSupportedBy("Intro. A channel coordinates communication. Outro."))
	assert.True(t, ref.IsSupportedBy("A channel coordinates communication."))

	// A message that was edited to remove or alter the quote no longer
	// supports it
	assert.False(t, ref.IsSupportedBy("Completely rewritten content."))
	assert.False(t, ref.IsSupportedBy("A channel coordinates communication"))
	assert.False(t, ref.IsSupportedBy(""))
}
