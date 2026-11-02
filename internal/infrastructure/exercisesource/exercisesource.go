// Package exercisesource adapts the study, profile and knowledge stores to the
// narrow ports challenge generation reads its context from, so the exercise
// package imports none of them. See
// specs/phases/phase-03-architecture-lab/06-01-challenge-generation.md.
package exercisesource

import (
	"context"
	"errors"
	"fmt"

	applicationexercise "github.com/santaniello/athena/internal/application/exercise"
	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// SessionTopics implements applicationexercise.SessionTopicReader.
type SessionTopics struct{ sessions domainstudy.SessionRepository }

// NewSessionTopics creates a SessionTopics reading from sessions.
func NewSessionTopics(sessions domainstudy.SessionRepository) *SessionTopics {
	return &SessionTopics{sessions: sessions}
}

// ReadSession returns the session's topic and goal, or
// applicationexercise.ErrSourceSessionNotFound.
func (r *SessionTopics) ReadSession(ctx context.Context, sessionID string) (domainexercise.SourceSession, error) {
	session, err := r.sessions.GetByID(ctx, sessionID)
	if errors.Is(err, domainstudy.ErrSessionNotFound) {
		return domainexercise.SourceSession{}, applicationexercise.ErrSourceSessionNotFound
	}
	if err != nil {
		return domainexercise.SourceSession{}, fmt.Errorf("exercisesource: reading session: %w", err)
	}
	return domainexercise.SourceSession{Topic: session.Topic, Goal: session.Goal}, nil
}

// Summaries implements applicationexercise.SummaryReader.
type Summaries struct {
	summaries domainstudy.ConversationSummaryRepository
}

// NewSummaries creates a Summaries reading from summaries.
func NewSummaries(summaries domainstudy.ConversationSummaryRepository) *Summaries {
	return &Summaries{summaries: summaries}
}

// ReadSummary returns the session's summary, or "" when it has none.
func (r *Summaries) ReadSummary(ctx context.Context, sessionID string) (string, error) {
	summary, err := r.summaries.Get(ctx, sessionID)
	if errors.Is(err, domainstudy.ErrConversationSummaryNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("exercisesource: reading summary: %w", err)
	}
	return summary.Content, nil
}

// Profiles implements applicationexercise.ProfileReader.
type Profiles struct{ profiles domainprofile.Store }

// NewProfiles creates a Profiles reading from profiles.
func NewProfiles(profiles domainprofile.Store) *Profiles { return &Profiles{profiles: profiles} }

// ReadProfile returns the focus area, experience level and assistant language.
func (r *Profiles) ReadProfile(context.Context) (domainexercise.GenerationProfile, error) {
	profile, err := r.profiles.Load()
	if err != nil {
		return domainexercise.GenerationProfile{}, fmt.Errorf("exercisesource: loading profile: %w", err)
	}
	return domainexercise.GenerationProfile{
		Area: profile.Area, ExperienceLevel: profile.ExperienceLevel, Language: profile.AssistantLanguage,
	}, nil
}

// Notes implements applicationexercise.NotesSearcher.
type Notes struct{ retriever domainknowledge.Retriever }

// NewNotes creates a Notes searching through retriever.
func NewNotes(retriever domainknowledge.Retriever) *Notes { return &Notes{retriever: retriever} }

// SearchNotes runs the session-scoped retrieval of spec 2.5 with the query and
// returns its chunks as excerpts. It fails, for instance, while the index is
// not ready.
func (r *Notes) SearchNotes(ctx context.Context, sessionID, query string) ([]domainexercise.NoteExcerpt, error) {
	result, err := r.retriever.Retrieve(ctx, sessionID, []string{query})
	if err != nil {
		return nil, fmt.Errorf("exercisesource: searching notes: %w", err)
	}
	excerpts := make([]domainexercise.NoteExcerpt, len(result.Chunks))
	for i, scored := range result.Chunks {
		excerpts[i] = domainexercise.NoteExcerpt{Source: scored.Chunk.FilePath, Content: scored.Chunk.Content}
	}
	return excerpts, nil
}
