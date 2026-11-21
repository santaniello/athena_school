package study

import (
	"context"
	"strings"
	"time"
)

// Assessment is how the tutor judged the user's latest message about a
// concept. It is the model's own judgment: see
// specs/phases/phase-04-tutor/04-evidence-and-session-summary.md.
type Assessment string

// The three assessments.
const (
	AssessmentCorrect          Assessment = "CORRECT"
	AssessmentPartiallyCorrect Assessment = "PARTIALLY_CORRECT"
	AssessmentIncorrect        Assessment = "INCORRECT"
)

// IsValid reports whether a is one of the three assessments.
func (a Assessment) IsValid() bool {
	switch a {
	case AssessmentCorrect, AssessmentPartiallyCorrect, AssessmentIncorrect:
		return true
	}
	return false
}

const (
	// MaxEvidencePerConcept bounds the proofs kept for one concept: the most
	// recent ones. With MaxConceptsPerSession it bounds a session's rows.
	MaxEvidencePerConcept = 10
	// MaxEvidenceDescriptionRunes caps the model's one-line description.
	MaxEvidenceDescriptionRunes = 200
)

// Evidence is one proof, written for a turn in which the user's own message
// showed something about a concept. It always names the user's message and
// the reply that justify it.
type Evidence struct {
	SessionID      string
	Concept        string // normalized, see NormalizeConceptName
	Assessment     Assessment
	Description    string
	Assisted       bool
	UserMessageID  string
	ReplyMessageID string
	CreatedAt      time.Time
}

// NewEvidence builds the evidence a report carries. The bool is false when
// the report supports none: no user evidence, no valid assessment, a blank
// concept, or a missing message id. Evidence without a message behind it is
// never created.
func NewEvidence(report StateReport, sessionID, userMessageID, replyMessageID string, now time.Time) (Evidence, bool) {
	concept := NormalizeConceptName(report.Concept)
	if !report.UserEvidence || !report.Assessment.IsValid() || concept == "" {
		return Evidence{}, false
	}
	if strings.TrimSpace(userMessageID) == "" || strings.TrimSpace(replyMessageID) == "" {
		return Evidence{}, false
	}
	return Evidence{
		SessionID:      sessionID,
		Concept:        concept,
		Assessment:     report.Assessment,
		Description:    normalizeDescription(report.Description),
		Assisted:       report.Assisted,
		UserMessageID:  userMessageID,
		ReplyMessageID: replyMessageID,
		CreatedAt:      now,
	}, true
}

// normalizeDescription collapses whitespace and cuts to
// MaxEvidenceDescriptionRunes runes.
func normalizeDescription(description string) string {
	runes := []rune(strings.Join(strings.Fields(description), " "))
	return strings.TrimSpace(string(runes[:min(len(runes), MaxEvidenceDescriptionRunes)]))
}

// EvidenceRepository persists the Evidence of a session.
type EvidenceRepository interface {
	// Add stores one proof. Its concept row and both messages must exist.
	Add(ctx context.Context, evidence Evidence) error
	// TrimOldest keeps only the keep most recent proofs of the concept.
	TrimOldest(ctx context.Context, sessionID, concept string, keep int) error
	// ListBySession returns the session's proofs, oldest first.
	ListBySession(ctx context.Context, sessionID string) ([]Evidence, error)
}
