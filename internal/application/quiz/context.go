package quiz

import (
	"context"
	"fmt"

	domainquiz "github.com/santaniello/athena/internal/domain/quiz"
)

// Bounds on the note excerpts offered to the model.
const (
	maxNoteChunks = 8
	maxNoteChars  = 6000
)

// generationContext is everything the model is told about the session before
// it writes a quiz.
type generationContext struct {
	Topic   string
	Goal    string
	Summary string // "" when the session has none
	Profile domainquiz.GenerationProfile
	Notes   []domainquiz.NoteExcerpt // bounded; empty when there are none
}

// generationContextBuilder gathers a generationContext through small ports.
type generationContextBuilder struct {
	summaries SummaryReader
	profiles  ProfileReader
	notes     NotesSearcher
}

// build collects the context of a session. A missing summary or missing notes
// are skipped, and a notes search that fails (for instance, an index that is
// not ready) never fails the build.
func (b generationContextBuilder) build(ctx context.Context, sessionID string, session domainquiz.SourceSession) (generationContext, error) {
	summary, err := b.summaries.ReadSummary(ctx, sessionID)
	if err != nil {
		return generationContext{}, fmt.Errorf("quiz: reading session summary: %w", err)
	}
	profile, err := b.profiles.ReadProfile(ctx)
	if err != nil {
		return generationContext{}, fmt.Errorf("quiz: reading profile: %w", err)
	}
	var notes []domainquiz.NoteExcerpt
	if found, err := b.notes.SearchNotes(ctx, sessionID, session.Topic); err == nil {
		notes = boundNotes(found)
	}
	return generationContext{Topic: session.Topic, Goal: session.Goal, Summary: summary, Profile: profile, Notes: notes}, nil
}

// boundNotes keeps the first maxNoteChunks excerpts and cuts the last one that
// still fits so the total stays within maxNoteChars characters.
func boundNotes(found []domainquiz.NoteExcerpt) []domainquiz.NoteExcerpt {
	var kept []domainquiz.NoteExcerpt
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
