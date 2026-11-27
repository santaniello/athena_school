package exercise

import (
	"context"
	"fmt"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// Bounds on the note excerpts offered to the model.
const (
	maxNoteChunks = 8
	maxNoteChars  = 6000
	// maxExistingTitles bounds the titles of earlier challenges offered to
	// the model.
	maxExistingTitles = 10
)

// generationContext is everything the model is told about the session before
// it writes a challenge.
type generationContext struct {
	Topic   string
	Goal    string
	Summary string // "" when the session has none
	Profile domainexercise.GenerationProfile
	Notes   []domainexercise.NoteExcerpt // bounded; empty when there are none
	// ExistingTitles are the titles of the session's earlier challenges of
	// the same kind, newest first, at most maxExistingTitles.
	ExistingTitles []string
}

// generationContextBuilder gathers a generationContext through small ports.
type generationContextBuilder struct {
	summaries SummaryReader
	profiles  ProfileReader
	notes     NotesSearcher
	titles    ExistingTitleLister
}

// build collects the context of a session for a challenge of the given kind.
// A missing summary or missing notes
// are skipped, and a notes search that fails (for instance, an index that is
// not ready) never fails the build.
func (b generationContextBuilder) build(ctx context.Context, sessionID string, session domainexercise.SourceSession, kind domainexercise.ChallengeKind) (generationContext, error) {
	summary, err := b.summaries.ReadSummary(ctx, sessionID)
	if err != nil {
		return generationContext{}, fmt.Errorf("exercise: reading session summary: %w", err)
	}
	profile, err := b.profiles.ReadProfile(ctx)
	if err != nil {
		return generationContext{}, fmt.Errorf("exercise: reading profile: %w", err)
	}
	var notes []domainexercise.NoteExcerpt
	if found, err := b.notes.SearchNotes(ctx, sessionID, session.Topic); err == nil {
		notes = boundNotes(found)
	}
	titles, err := b.titles.ListTitlesBySourceSession(ctx, sessionID, kind, maxExistingTitles)
	if err != nil {
		return generationContext{}, fmt.Errorf("exercise: listing existing challenge titles: %w", err)
	}
	return generationContext{
		Topic: session.Topic, Goal: session.Goal, Summary: summary, Profile: profile, Notes: notes, ExistingTitles: titles,
	}, nil
}

// boundNotes keeps the first maxNoteChunks excerpts and cuts the last one that
// still fits so the total stays within maxNoteChars characters.
func boundNotes(found []domainexercise.NoteExcerpt) []domainexercise.NoteExcerpt {
	var kept []domainexercise.NoteExcerpt
	remaining := maxNoteChars
	for _, excerpt := range found {
		if len(kept) == maxNoteChunks || remaining == 0 {
			break
		}
		runes := []rune(excerpt.Content)
		runes = runes[:min(len(runes), remaining)]
		excerpt.Content = string(runes)
		remaining -= len(runes)
		kept = append(kept, excerpt)
	}
	return kept
}
