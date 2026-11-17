// Package quizsource adapts the study, profile and knowledge stores to the
// narrow ports quiz generation reads its context from, so the quiz packages
// import none of them. See
// specs/phases/phase-03-architecture-lab/21-interactive-quiz-core.md.
package quizsource

import (
	"context"
	"errors"
	"fmt"

	applicationquiz "github.com/santaniello/athena/internal/application/quiz"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	domainquiz "github.com/santaniello/athena/internal/domain/quiz"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Sessions implements applicationquiz.SessionReader.
type Sessions struct{ sessions domainstudy.SessionRepository }

// NewSessions creates a Sessions reading from sessions.
func NewSessions(sessions domainstudy.SessionRepository) *Sessions {
	return &Sessions{sessions: sessions}
}

// ReadSession returns the session's topic and goal, or
// applicationquiz.ErrSourceSessionNotFound.
func (r *Sessions) ReadSession(ctx context.Context, sessionID string) (domainquiz.SourceSession, error) {
	session, err := r.sessions.GetByID(ctx, sessionID)
	if errors.Is(err, domainstudy.ErrSessionNotFound) {
		return domainquiz.SourceSession{}, applicationquiz.ErrSourceSessionNotFound
	}
	if err != nil {
		return domainquiz.SourceSession{}, fmt.Errorf("quizsource: reading session: %w", err)
	}
	return domainquiz.SourceSession{Topic: session.Topic, Goal: session.Goal}, nil
}

// Summaries implements applicationquiz.SummaryReader.
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
		return "", fmt.Errorf("quizsource: reading summary: %w", err)
	}
	return summary.Content, nil
}

// Profiles implements applicationquiz.ProfileReader.
type Profiles struct{ profiles domainprofile.Store }

// NewProfiles creates a Profiles reading from profiles.
func NewProfiles(profiles domainprofile.Store) *Profiles { return &Profiles{profiles: profiles} }

// ReadProfile returns the focus area, experience level and assistant language.
func (r *Profiles) ReadProfile(context.Context) (domainquiz.GenerationProfile, error) {
	profile, err := r.profiles.Load()
	if err != nil {
		return domainquiz.GenerationProfile{}, fmt.Errorf("quizsource: loading profile: %w", err)
	}
	return domainquiz.GenerationProfile{
		Area: profile.Area, ExperienceLevel: profile.ExperienceLevel, Language: profile.AssistantLanguage,
	}, nil
}

// Notes implements applicationquiz.NotesSearcher.
type Notes struct{ retriever domainknowledge.Retriever }

// NewNotes creates a Notes searching through retriever.
func NewNotes(retriever domainknowledge.Retriever) *Notes { return &Notes{retriever: retriever} }

// SearchNotes runs the session-scoped retrieval of spec 2.5 with the query and
// returns its chunks as excerpts. It fails, for instance, while the index is
// not ready.
func (r *Notes) SearchNotes(ctx context.Context, sessionID, query string) ([]domainquiz.NoteExcerpt, error) {
	result, err := r.retriever.Retrieve(ctx, sessionID, []string{query})
	if err != nil {
		return nil, fmt.Errorf("quizsource: searching notes: %w", err)
	}
	excerpts := make([]domainquiz.NoteExcerpt, len(result.Chunks))
	for i, scored := range result.Chunks {
		excerpts[i] = domainquiz.NoteExcerpt{Source: scored.Chunk.FilePath, Content: scored.Chunk.Content}
	}
	return excerpts, nil
}

// offerService is what OfferGate needs from the quiz offer service.
type offerService interface {
	GetPending(ctx context.Context, sessionID string) (domainquiz.Offer, bool, error)
	CreateOffer(ctx context.Context, sessionID, messageID string) (domainquiz.Offer, error)
}

// OfferGate adapts the quiz offer service to the shape the study chat's
// composed offer gate (package studyoffers) expects, so the chat package
// imports no quiz code. See
// specs/phases/phase-03-architecture-lab/23-interactive-quiz-chat-offer.md.
type OfferGate struct{ offers offerService }

// NewOfferGate creates an OfferGate backed by offers.
func NewOfferGate(offers offerService) *OfferGate { return &OfferGate{offers: offers} }

// HasPending reports whether the session has a pending quiz offer.
func (g *OfferGate) HasPending(ctx context.Context, sessionID string) (bool, error) {
	_, found, err := g.offers.GetPending(ctx, sessionID)
	return found, err
}

// CreateOffer stores a pending quiz offer under the assistant message.
func (g *OfferGate) CreateOffer(ctx context.Context, sessionID, messageID string) error {
	_, err := g.offers.CreateOffer(ctx, sessionID, messageID)
	return err
}
