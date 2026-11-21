package study

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func evidenceReport(assessment Assessment, evidence bool) StateReport {
	return StateReport{
		Concept:      "Race Condition",
		Status:       StatusDemonstrated,
		Strategy:     StrategyComprehensionCheck,
		UserEvidence: evidence,
		Assisted:     true,
		Assessment:   assessment,
		Description:  "  Explained  why a mutex\nfixes it  ",
	}
}

func TestAssessment_isValid_acceptsOnlyTheThreeValues(t *testing.T) {
	// Given the three assessments, an empty one and an unknown one
	// When checking validity
	// Then only the three documented values are valid
	assert.True(t, AssessmentCorrect.IsValid())
	assert.True(t, AssessmentPartiallyCorrect.IsValid())
	assert.True(t, AssessmentIncorrect.IsValid())
	assert.False(t, Assessment("").IsValid())
	assert.False(t, Assessment("correct").IsValid())
	assert.False(t, Assessment("MAYBE").IsValid())
}

func TestNewEvidence_withUserEvidence_buildsRowLinkedToBothMessages(t *testing.T) {
	// Given a report with the user's evidence and a valid assessment
	// When building the evidence
	evidence, ok := NewEvidence(evidenceReport(AssessmentCorrect, true), "session-1", "user-msg", "reply-msg", stateNow)

	// Then the row names the session, the normalized concept, both messages and the report's fields
	assert.True(t, ok)
	assert.Equal(t, "session-1", evidence.SessionID)
	assert.Equal(t, "race condition", evidence.Concept)
	assert.Equal(t, AssessmentCorrect, evidence.Assessment)
	assert.Equal(t, "Explained why a mutex fixes it", evidence.Description)
	assert.True(t, evidence.Assisted)
	assert.Equal(t, "user-msg", evidence.UserMessageID)
	assert.Equal(t, "reply-msg", evidence.ReplyMessageID)
	assert.Equal(t, stateNow, evidence.CreatedAt)
}

func TestNewEvidence_withoutUserEvidence_isRejected(t *testing.T) {
	// Given a report that says the user showed nothing
	// When building the evidence
	_, ok := NewEvidence(evidenceReport(AssessmentCorrect, false), "session-1", "user-msg", "reply-msg", stateNow)

	// Then no evidence is created
	assert.False(t, ok)
}

func TestNewEvidence_withMissingOrUnknownAssessment_isRejected(t *testing.T) {
	for _, assessment := range []Assessment{"", "GOOD"} {
		t.Run(string(assessment), func(t *testing.T) {
			// Given a report with the user's evidence but no usable assessment
			// When building the evidence
			_, ok := NewEvidence(evidenceReport(assessment, true), "session-1", "user-msg", "reply-msg", stateNow)

			// Then no evidence is created
			assert.False(t, ok)
		})
	}
}

func TestNewEvidence_withoutSupportingMessage_isRejected(t *testing.T) {
	cases := map[string][2]string{
		"blank user message":  {" ", "reply-msg"},
		"blank reply message": {"user-msg", ""},
	}
	for name, ids := range cases {
		t.Run(name, func(t *testing.T) {
			// Given a valid report but a missing message id
			// When building the evidence
			_, ok := NewEvidence(evidenceReport(AssessmentCorrect, true), "session-1", ids[0], ids[1], stateNow)

			// Then it is rejected: evidence always names the messages behind it
			assert.False(t, ok)
		})
	}
}

func TestNewEvidence_withBlankConcept_isRejected(t *testing.T) {
	// Given a report whose concept normalizes to nothing
	report := evidenceReport(AssessmentCorrect, true)
	report.Concept = "   "

	// When building the evidence
	_, ok := NewEvidence(report, "session-1", "user-msg", "reply-msg", stateNow)

	// Then it is rejected
	assert.False(t, ok)
}

func TestNewEvidence_longDescription_isCutOnRunes(t *testing.T) {
	// Given a description longer than the limit, made of multi-byte runes
	report := evidenceReport(AssessmentIncorrect, true)
	report.Description = strings.Repeat("é", MaxEvidenceDescriptionRunes+50)

	// When building the evidence
	evidence, ok := NewEvidence(report, "session-1", "user-msg", "reply-msg", stateNow)

	// Then it is cut to the limit, never in the middle of a rune
	assert.True(t, ok)
	assert.Equal(t, MaxEvidenceDescriptionRunes, len([]rune(evidence.Description)))
	assert.Equal(t, strings.Repeat("é", MaxEvidenceDescriptionRunes), evidence.Description)
}

func TestNewEvidence_missingDescription_isStillValid(t *testing.T) {
	// Given a valid report without a description
	report := evidenceReport(AssessmentPartiallyCorrect, true)
	report.Description = ""

	// When building the evidence
	evidence, ok := NewEvidence(report, "session-1", "user-msg", "reply-msg", stateNow)

	// Then the description is optional
	assert.True(t, ok)
	assert.Empty(t, evidence.Description)
}

func TestMaxEvidencePerConcept_isTen(t *testing.T) {
	// Given the documented cap
	// When reading it
	// Then it is ten proofs per concept
	assert.Equal(t, 10, MaxEvidencePerConcept)
}
