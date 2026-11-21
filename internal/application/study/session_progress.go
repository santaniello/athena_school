package study

import (
	"context"
	"fmt"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// GetSessionProgress returns what the tutor has observed in the session,
// grouped for the user (see domainstudy.BuildSessionProgress). It is derived
// from the stored concepts and proofs only: no LLM call. A service without the
// pedagogical state reports an empty progress.
func (s *Service) GetSessionProgress(ctx context.Context, sessionID string) (domainstudy.SessionProgress, error) {
	if s.concepts == nil {
		return domainstudy.BuildSessionProgress(nil, nil), nil
	}
	concepts, err := s.concepts.ListBySession(ctx, sessionID)
	if err != nil {
		return domainstudy.SessionProgress{}, fmt.Errorf("study: reading session progress: %w", err)
	}
	var proofs []domainstudy.Evidence
	if s.evidence != nil {
		if proofs, err = s.evidence.ListBySession(ctx, sessionID); err != nil {
			return domainstudy.SessionProgress{}, fmt.Errorf("study: reading session progress: %w", err)
		}
	}
	return domainstudy.BuildSessionProgress(concepts, proofs), nil
}
