package study

import (
	"context"
	"strings"
	"time"
)

// ConceptStatus is what the tutor has observed about one concept in a
// session. Only evidence from the user's own messages moves it up: see
// specs/phases/phase-04-tutor/02-pedagogical-state.md.
type ConceptStatus string

// The five concept statuses.
const (
	StatusNotAssessed         ConceptStatus = "NOT_ASSESSED"
	StatusIntroduced          ConceptStatus = "INTRODUCED"
	StatusPartiallyUnderstood ConceptStatus = "PARTIALLY_UNDERSTOOD"
	StatusDemonstrated        ConceptStatus = "DEMONSTRATED"
	StatusNeedsReview         ConceptStatus = "NEEDS_REVIEW"
)

// IsValid reports whether s is one of the five statuses.
func (s ConceptStatus) IsValid() bool {
	switch s {
	case StatusNotAssessed, StatusIntroduced, StatusPartiallyUnderstood, StatusDemonstrated, StatusNeedsReview:
		return true
	}
	return false
}

// TutorStrategy is the approach the tutor used in a reply: the six labels of
// the adaptive tutor prompt.
type TutorStrategy string

// The six strategies, as the adaptive tutor prompt names them.
const (
	StrategyDirectInstruction   TutorStrategy = "DIRECT_INSTRUCTION"
	StrategySocraticQuestioning TutorStrategy = "SOCRATIC_QUESTIONING"
	StrategyGuidedPractice      TutorStrategy = "GUIDED_PRACTICE"
	StrategyComprehensionCheck  TutorStrategy = "COMPREHENSION_CHECK"
	StrategyCorrectiveFeedback  TutorStrategy = "CORRECTIVE_FEEDBACK"
	StrategyConceptReview       TutorStrategy = "CONCEPT_REVIEW"
)

// IsValid reports whether s is one of the six strategies.
func (s TutorStrategy) IsValid() bool {
	switch s {
	case StrategyDirectInstruction, StrategySocraticQuestioning, StrategyGuidedPractice,
		StrategyComprehensionCheck, StrategyCorrectiveFeedback, StrategyConceptReview:
		return true
	}
	return false
}

const (
	// MaxConceptsPerSession bounds the rows kept for one session.
	MaxConceptsPerSession = 30
	// MaxConceptNameRunes caps a normalized concept name.
	MaxConceptNameRunes = 80
)

// ConceptState is the row kept for one concept of one session.
type ConceptState struct {
	SessionID       string
	Name            string // normalized, see NormalizeConceptName
	Status          ConceptStatus
	LastStrategy    TutorStrategy // may be empty: the tutor omitted it or used an unknown one
	AssistanceCount int
	EvidenceCount   int // turns with UserEvidence = true
	UpdatedAt       time.Time
}

// StateReport is what the tutor's hidden marker carries after a reply.
type StateReport struct {
	Concept      string
	Status       ConceptStatus
	Strategy     TutorStrategy
	UserEvidence bool // the user's latest message shows the behavior behind Status
	Assisted     bool // the tutor had to hint or explain to get there
}

// ConceptStateRepository persists the ConceptStates of a session.
type ConceptStateRepository interface {
	// Upsert inserts or replaces the row identified by (SessionID, Name).
	Upsert(ctx context.Context, state ConceptState) error
	// ListBySession returns the session's concepts, oldest update first.
	ListBySession(ctx context.Context, sessionID string) ([]ConceptState, error)
	// DeleteOldest removes one concept of the session, never the one named
	// keep: the oldest DEMONSTRATED one, else the oldest overall.
	DeleteOldest(ctx context.Context, sessionID, keep string) error
}

// NormalizeConceptName is the identity of a concept: trimmed, lower-cased,
// inner whitespace collapsed and cut to MaxConceptNameRunes runes. Accents
// are kept. A blank name stays blank.
func NormalizeConceptName(name string) string {
	collapsed := strings.ToLower(strings.Join(strings.Fields(name), " "))
	runes := []rune(collapsed)
	return strings.TrimSpace(string(runes[:min(len(runes), MaxConceptNameRunes)]))
}

// ApplyReport folds a report into the concept's current row (nil when the
// concept has none yet) and returns the row to store. The bool is false when
// the report changes nothing (a NOT_ASSESSED report). current is not mutated;
// the caller sets SessionID on a new row.
//
// Only evidence moves a status. Without UserEvidence a report can create the
// row at INTRODUCED but never changes an existing status. With it, the
// reported status applies, except that DEMONSTRATED on a NEEDS_REVIEW concept
// first steps to PARTIALLY_UNDERSTOOD. The counters and LastStrategy move
// either way.
func ApplyReport(current *ConceptState, report StateReport, now time.Time) (ConceptState, bool) {
	if report.Status == StatusNotAssessed {
		if current == nil {
			return ConceptState{}, false
		}
		return *current, false
	}

	var next ConceptState
	if current != nil {
		next = *current
		if report.UserEvidence {
			next.Status = reportedStatus(current.Status, report.Status)
		}
	} else {
		next.Name = NormalizeConceptName(report.Concept)
		next.Status = StatusIntroduced
		if report.UserEvidence {
			next.Status = report.Status
		}
	}

	if report.Assisted {
		next.AssistanceCount++
	}
	if report.UserEvidence {
		next.EvidenceCount++
	}
	if report.Strategy != "" {
		next.LastStrategy = report.Strategy
	}
	next.UpdatedAt = now
	return next, true
}

// reportedStatus is the status an existing concept takes when the user's
// evidence supports reported.
func reportedStatus(current, reported ConceptStatus) ConceptStatus {
	if current == StatusNeedsReview && reported == StatusDemonstrated {
		return StatusPartiallyUnderstood
	}
	return reported
}
