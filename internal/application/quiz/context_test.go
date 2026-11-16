package quiz

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	domainquiz "github.com/santaniello/athena/internal/domain/quiz"
)

func excerpt(content string) domainquiz.NoteExcerpt {
	return domainquiz.NoteExcerpt{Source: "n.md", Content: content}
}

func TestBoundNotes_keepsAtMostEightExcerpts(t *testing.T) {
	// Given ten short excerpts
	var found []domainquiz.NoteExcerpt
	for i := 0; i < 10; i++ {
		found = append(found, excerpt("note"))
	}

	// When they are bounded
	kept := boundNotes(found)

	// Then only the first eight are kept
	assert.Len(t, kept, maxNoteChunks)
}

func TestBoundNotes_cutsTheLastExcerptToTheCharacterBudget(t *testing.T) {
	// Given two excerpts of 4000 characters
	found := []domainquiz.NoteExcerpt{excerpt(strings.Repeat("a", 4000)), excerpt(strings.Repeat("b", 4000))}

	// When they are bounded
	kept := boundNotes(found)

	// Then the second is cut so the total is exactly the budget
	assert.Len(t, kept, 2)
	assert.Len(t, kept[0].Content, 4000)
	assert.Len(t, kept[1].Content, maxNoteChars-4000)
}

func TestBoundNotes_stopsOnceTheBudgetIsSpent(t *testing.T) {
	// Given an excerpt that uses the whole budget followed by another
	found := []domainquiz.NoteExcerpt{excerpt(strings.Repeat("a", maxNoteChars)), excerpt("more")}

	// When they are bounded
	kept := boundNotes(found)

	// Then the second one is dropped
	assert.Len(t, kept, 1)
}

func TestBoundNotes_countsCharactersNotBytes(t *testing.T) {
	// Given an excerpt of multi-byte characters longer than the budget
	found := []domainquiz.NoteExcerpt{excerpt(strings.Repeat("é", maxNoteChars+10))}

	// When it is bounded
	kept := boundNotes(found)

	// Then it is cut at the budget in characters, never in the middle of one
	assert.Equal(t, strings.Repeat("é", maxNoteChars), kept[0].Content)
}

func TestBoundNotes_noExcerptsKeepsNone(t *testing.T) {
	assert.Empty(t, boundNotes(nil))
}
