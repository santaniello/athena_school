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

// recordTutorState applies the reply's state report to the session's concepts,
// after the reply is safely persisted. A bad marker, or a failing repository,
// is logged without any message content and never fails the turn. A reply that
// answers no user message (the opening turn) has no evidence to report, so its
// userEvidence is forced to false.
func (s *Service) recordTutorState(
	ctx context.Context, sessionID string, report *domainstudy.StateReport, markerErr error, repliesToUser bool,
) {
	if markerErr != nil {
		log.Printf("study: tutor state of session %s: %v", sessionID, markerErr)
	}
	if report == nil || s.concepts == nil {
		return
	}
	if !repliesToUser {
		report.UserEvidence = false
	}
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		return s.applyConceptReport(ctx, sessionID, *report)
	})
	if err != nil {
		log.Printf("study: storing tutor state of session %s: %v", sessionID, err)
	}
}

// applyConceptReport upserts the reported concept and, when that leaves more
// than domainstudy.MaxConceptsPerSession, evicts one other concept. It runs in
// one transaction, so the session never keeps more than the limit.
func (s *Service) applyConceptReport(ctx context.Context, sessionID string, report domainstudy.StateReport) error {
	stored, err := s.concepts.ListBySession(ctx, sessionID)
	if err != nil {
		return err
	}
	name := domainstudy.NormalizeConceptName(report.Concept)
	var current *domainstudy.ConceptState
	for i := range stored {
		if stored[i].Name == name {
			current = &stored[i]
			break
		}
	}

	next, changed := domainstudy.ApplyReport(current, report, time.Now().UTC())
	if !changed {
		return nil
	}
	next.SessionID = sessionID
	if err := s.concepts.Upsert(ctx, next); err != nil {
		return err
	}

	total := len(stored)
	if current == nil {
		total++
	}
	if total > domainstudy.MaxConceptsPerSession {
		return s.concepts.DeleteOldest(ctx, sessionID, next.Name)
	}
	return nil
}
