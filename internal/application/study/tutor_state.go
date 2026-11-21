package study

import (
	"context"
	"log"
	"time"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// WithConceptStates wires the tutor's pedagogical state (see
// tutor_state_marker.go). Without it the state block is still hidden and
// stripped from every reply, but nothing is stored.
func WithConceptStates(concepts domainstudy.ConceptStateRepository) Option {
	return func(s *Service) { s.concepts = concepts }
}

// WithConceptEvidence wires the proofs behind the tutor's state changes (see
// domainstudy.Evidence). Without it the state is stored as usual but no proof is.
func WithConceptEvidence(evidence domainstudy.EvidenceRepository) Option {
	return func(s *Service) { s.evidence = evidence }
}

// recordTutorState applies the reply's state report to the session's concepts,
// after the reply is safely persisted, then stores the proof it carries. A bad
// marker, or a failing repository, is logged without any message content and
// never fails the turn. userMessageID is the message the reply answers, empty
// for a reply that answers none (the opening turn): such a reply has no
// evidence to report, so its userEvidence is forced to false.
func (s *Service) recordTutorState(
	ctx context.Context, sessionID string, report *domainstudy.StateReport, markerErr error,
	userMessageID, replyMessageID string,
) {
	if markerErr != nil {
		log.Printf("study: tutor state of session %s: %v", sessionID, markerErr)
	}
	if report == nil || s.concepts == nil {
		return
	}
	if userMessageID == "" {
		report.UserEvidence = false
	}
	now := time.Now().UTC()
	stored := false
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		stored, err = s.applyConceptReport(ctx, sessionID, *report, now)
		return err
	})
	if err != nil {
		log.Printf("study: storing tutor state of session %s: %v", sessionID, err)
		return
	}
	if stored {
		s.recordEvidence(ctx, sessionID, *report, userMessageID, replyMessageID, now)
	}
}

// recordEvidence stores the proof the report carries, if any, and keeps the
// concept's most recent ones. It runs apart from the state write, so a failure
// here never undoes the concept's update, and is only logged.
func (s *Service) recordEvidence(
	ctx context.Context, sessionID string, report domainstudy.StateReport, userMessageID, replyMessageID string, now time.Time,
) {
	if s.evidence == nil {
		return
	}
	proof, ok := domainstudy.NewEvidence(report, sessionID, userMessageID, replyMessageID, now)
	if !ok {
		return
	}
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.evidence.Add(ctx, proof); err != nil {
			return err
		}
		return s.evidence.TrimOldest(ctx, sessionID, proof.Concept, domainstudy.MaxEvidencePerConcept)
	})
	if err != nil {
		log.Printf("study: storing tutor evidence of session %s: %v", sessionID, err)
	}
}

// applyConceptReport upserts the reported concept and, when that leaves more
// than domainstudy.MaxConceptsPerSession, evicts one other concept. It runs in
// one transaction, so the session never keeps more than the limit. The bool
// reports whether the concept's row was written.
func (s *Service) applyConceptReport(
	ctx context.Context, sessionID string, report domainstudy.StateReport, now time.Time,
) (bool, error) {
	stored, err := s.concepts.ListBySession(ctx, sessionID)
	if err != nil {
		return false, err
	}
	name := domainstudy.NormalizeConceptName(report.Concept)
	var current *domainstudy.ConceptState
	for i := range stored {
		if stored[i].Name == name {
			current = &stored[i]
			break
		}
	}

	next, changed := domainstudy.ApplyReport(current, report, now)
	if !changed {
		return false, nil
	}
	next.SessionID = sessionID
	if err := s.concepts.Upsert(ctx, next); err != nil {
		return false, err
	}

	total := len(stored)
	if current == nil {
		total++
	}
	if total > domainstudy.MaxConceptsPerSession {
		return true, s.concepts.DeleteOldest(ctx, sessionID, next.Name)
	}
	return true, nil
}
