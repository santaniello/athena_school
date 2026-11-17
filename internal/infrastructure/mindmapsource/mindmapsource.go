// Package mindmapsource adapts the study, profile and knowledge stores to the
// narrow ports mind map generation reads its context from, so the mind map
// packages import none of them. See
// specs/phases/phase-03-architecture-lab/24-mind-map.md.
package mindmapsource

import (
	"context"
	"errors"
	"fmt"

	applicationmindmap "github.com/santaniello/athena/internal/application/mindmap"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainmindmap "github.com/santaniello/athena/internal/domain/mindmap"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Sessions implements applicationmindmap.SessionReader.
type Sessions struct{ sessions domainstudy.SessionRepository }

// NewSessions creates a Sessions reading from sessions.
func NewSessions(sessions domainstudy.SessionRepository) *Sessions {
	return &Sessions{sessions: sessions}
}

// ReadSession returns the session's topic and goal, or
// applicationmindmap.ErrSourceSessionNotFound.
func (r *Sessions) ReadSession(ctx context.Context, sessionID string) (domainmindmap.SourceSession, error) {
	session, err := r.sessions.GetByID(ctx, sessionID)
	if errors.Is(err, domainstudy.ErrSessionNotFound) {
		return domainmindmap.SourceSession{}, applicationmindmap.ErrSourceSessionNotFound
	}
	if err != nil {
		return domainmindmap.SourceSession{}, fmt.Errorf("mindmapsource: reading session: %w", err)
	}
	return domainmindmap.SourceSession{Topic: session.Topic, Goal: session.Goal}, nil
}

// Summaries implements applicationmindmap.SummaryReader.
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
		return "", fmt.Errorf("mindmapsource: reading summary: %w", err)
	}
	return summary.Content, nil
}

// Profiles implements applicationmindmap.ProfileReader.
type Profiles struct{ profiles domainprofile.Store }

// NewProfiles creates a Profiles reading from profiles.
func NewProfiles(profiles domainprofile.Store) *Profiles { return &Profiles{profiles: profiles} }

// ReadProfile returns the focus area, experience level and assistant language.
func (r *Profiles) ReadProfile(context.Context) (domainmindmap.GenerationProfile, error) {
	profile, err := r.profiles.Load()
	if err != nil {
		return domainmindmap.GenerationProfile{}, fmt.Errorf("mindmapsource: loading profile: %w", err)
	}
	return domainmindmap.GenerationProfile{
		Area: profile.Area, ExperienceLevel: profile.ExperienceLevel, Language: profile.AssistantLanguage,
	}, nil
}

// Notes implements applicationmindmap.NotesSearcher.
type Notes struct{ retriever domainknowledge.Retriever }

// NewNotes creates a Notes searching through retriever.
func NewNotes(retriever domainknowledge.Retriever) *Notes { return &Notes{retriever: retriever} }

// SearchNotes runs the session-scoped retrieval of spec 2.5 with the query and
// returns its chunks as excerpts. It fails, for instance, while the index is
// not ready.
func (r *Notes) SearchNotes(ctx context.Context, sessionID, query string) ([]domainmindmap.NoteExcerpt, error) {
	result, err := r.retriever.Retrieve(ctx, sessionID, []string{query})
	if err != nil {
		return nil, fmt.Errorf("mindmapsource: searching notes: %w", err)
	}
	excerpts := make([]domainmindmap.NoteExcerpt, len(result.Chunks))
	for i, scored := range result.Chunks {
		excerpts[i] = domainmindmap.NoteExcerpt{Source: scored.Chunk.FilePath, Content: scored.Chunk.Content}
	}
	return excerpts, nil
}

// offerService is what OfferGate needs from the mind map offer service.
type offerService interface {
	GetPending(ctx context.Context, sessionID string) (domainmindmap.Offer, bool, error)
	CreateOffer(ctx context.Context, sessionID, messageID string) (domainmindmap.Offer, error)
}

// OfferGate adapts the mind map offer service to the shape the study chat's
// composed offer gate (package studyoffers) expects, so the chat package
// imports no mind map code. See
// specs/phases/phase-03-architecture-lab/24-mind-map.md.
type OfferGate struct{ offers offerService }

// NewOfferGate creates an OfferGate backed by offers.
func NewOfferGate(offers offerService) *OfferGate { return &OfferGate{offers: offers} }

// HasPending reports whether the session has a pending mind map offer.
func (g *OfferGate) HasPending(ctx context.Context, sessionID string) (bool, error) {
	_, found, err := g.offers.GetPending(ctx, sessionID)
	return found, err
}

// CreateOffer stores a pending mind map offer under the assistant message.
func (g *OfferGate) CreateOffer(ctx context.Context, sessionID, messageID string) error {
	_, err := g.offers.CreateOffer(ctx, sessionID, messageID)
	return err
}

// The adapters implement the ports the application declares.
var (
	_ applicationmindmap.SessionReader = (*Sessions)(nil)
	_ applicationmindmap.SummaryReader = (*Summaries)(nil)
	_ applicationmindmap.ProfileReader = (*Profiles)(nil)
	_ applicationmindmap.NotesSearcher = (*Notes)(nil)
)
