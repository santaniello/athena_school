package exercise

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func validChallenge() Challenge {
	return Challenge{
		ID:                     "challenge-1",
		Type:                   ChallengeTypeArchitecture,
		Title:                  "Design a URL shortener",
		Scenario:               "A startup needs a service that shortens URLs.",
		FunctionalRequirements: []string{"Create a short link", "Redirect a short link"},
		Objectives:             []string{"Justify the storage choice"},
		Difficulty:             DifficultyIntermediate,
		Topic:                  "url shortener",
		SourceSessionID:        "session-1",
		Origin:                 OriginManual,
		TimeLimitMinutes:       45,
		CreatedAt:              time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC),
		UpdatedAt:              time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC),
	}
}

func TestChallenge_Validate_acceptsValidChallenge(t *testing.T) {
	// Given a fully valid challenge
	challenge := validChallenge()

	// When validating it
	err := challenge.Validate()

	// Then no error is returned
	assert.NoError(t, err)
}

func TestChallenge_Validate_acceptsOptionalListsAndNoSourceSession(t *testing.T) {
	// Given a challenge whose source session was deleted and that has no optional lists
	challenge := validChallenge()
	challenge.SourceSessionID = ""
	challenge.NonFunctionalRequirements = nil
	challenge.Constraints = nil

	// When validating it
	err := challenge.Validate()

	// Then it is still valid
	assert.NoError(t, err)
}

func TestChallenge_Validate_rejectsInvalidChallenge(t *testing.T) {
	longItem := strings.Repeat("x", MaxListItemLength+1)
	tooManyItems := make([]string, MaxListItems+1)
	for i := range tooManyItems {
		tooManyItems[i] = strings.Repeat("a", i+1)
	}

	tests := []struct {
		name   string
		mutate func(*Challenge)
		want   error
	}{
		{"blank title", func(c *Challenge) { c.Title = "  \t" }, ErrChallengeTitleRequired},
		{"blank scenario", func(c *Challenge) { c.Scenario = "\n" }, ErrChallengeScenarioRequired},
		{"blank topic", func(c *Challenge) { c.Topic = " " }, ErrChallengeTopicRequired},
		{"unknown type", func(c *Challenge) { c.Type = "ALGORITHMS" }, ErrChallengeTypeInvalid},
		{"empty type", func(c *Challenge) { c.Type = "" }, ErrChallengeTypeInvalid},
		{"unknown difficulty", func(c *Challenge) { c.Difficulty = "EXPERT" }, ErrDifficultyInvalid},
		{"unknown origin", func(c *Challenge) { c.Origin = "IMPORTED" }, ErrOriginInvalid},
		{"no functional requirement", func(c *Challenge) { c.FunctionalRequirements = nil }, ErrFunctionalRequirementsRequired},
		{"no objective", func(c *Challenge) { c.Objectives = []string{} }, ErrObjectivesRequired},
		{"blank functional requirement", func(c *Challenge) { c.FunctionalRequirements = []string{"ok", " "} }, ErrListItemEmpty},
		{"blank non functional requirement", func(c *Challenge) { c.NonFunctionalRequirements = []string{""} }, ErrListItemEmpty},
		{"blank constraint", func(c *Challenge) { c.Constraints = []string{"\t"} }, ErrListItemEmpty},
		{"blank objective", func(c *Challenge) { c.Objectives = []string{" "} }, ErrListItemEmpty},
		{"duplicate item", func(c *Challenge) { c.Constraints = []string{"Budget", "Budget"} }, ErrListItemDuplicate},
		{"duplicate item after trimming", func(c *Challenge) { c.Objectives = []string{"Scale", " Scale "} }, ErrListItemDuplicate},
		{"title too long", func(c *Challenge) { c.Title = strings.Repeat("t", MaxTitleLength+1) }, ErrChallengeTitleTooLong},
		{"scenario too long", func(c *Challenge) { c.Scenario = strings.Repeat("s", MaxScenarioLength+1) }, ErrChallengeScenarioTooLong},
		{"list item too long", func(c *Challenge) { c.Constraints = []string{longItem} }, ErrListItemTooLong},
		{"too many list items", func(c *Challenge) { c.FunctionalRequirements = tooManyItems }, ErrTooManyListItems},
		{"time limit zero", func(c *Challenge) { c.TimeLimitMinutes = 0 }, ErrTimeLimitInvalid},
		{"time limit negative", func(c *Challenge) { c.TimeLimitMinutes = -1 }, ErrTimeLimitInvalid},
		{"time limit above maximum", func(c *Challenge) { c.TimeLimitMinutes = MaxTimeLimitMinutes + 1 }, ErrTimeLimitInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a valid challenge broken in exactly one way
			challenge := validChallenge()
			tt.mutate(&challenge)

			// When validating it
			err := challenge.Validate()

			// Then the specific error is returned
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestChallenge_Validate_acceptsLimitsExactly(t *testing.T) {
	// Given a challenge sitting exactly at every size limit
	challenge := validChallenge()
	challenge.Title = strings.Repeat("t", MaxTitleLength)
	challenge.Scenario = strings.Repeat("s", MaxScenarioLength)
	challenge.Constraints = []string{strings.Repeat("c", MaxListItemLength)}
	items := make([]string, MaxListItems)
	for i := range items {
		items[i] = strings.Repeat("a", i+1)
	}
	challenge.FunctionalRequirements = items

	// When validating it
	err := challenge.Validate()

	// Then the limits are inclusive
	assert.NoError(t, err)
}

func TestChallenge_Validate_countsRunesNotBytes(t *testing.T) {
	// Given a title of MaxTitleLength multi-byte characters
	challenge := validChallenge()
	challenge.Title = strings.Repeat("é", MaxTitleLength)

	// When validating it
	err := challenge.Validate()

	// Then it is accepted, since the limit is in characters
	assert.NoError(t, err)
}

func TestChallenge_Validate_acceptsTimeLimitBounds(t *testing.T) {
	for _, minutes := range []int{MinTimeLimitMinutes, MaxTimeLimitMinutes} {
		// Given a challenge whose time limit sits exactly on a bound
		challenge := validChallenge()
		challenge.TimeLimitMinutes = minutes

		// When validating it
		err := challenge.Validate()

		// Then the bound is inclusive
		assert.NoError(t, err, "minutes=%d", minutes)
	}
}

func TestDefaultTimeLimit_followsDifficulty(t *testing.T) {
	tests := []struct {
		difficulty Difficulty
		want       int
	}{
		{DifficultyBeginner, 30},
		{DifficultyIntermediate, 45},
		{DifficultyAdvanced, 60},
		{"EXPERT", 0},
	}
	for _, tt := range tests {
		t.Run(string(tt.difficulty), func(t *testing.T) {
			// Given a difficulty
			// When asking for its default time limit
			got := DefaultTimeLimit(tt.difficulty)

			// Then it matches the spec table, and an unknown difficulty has none
			assert.Equal(t, tt.want, got)
		})
	}
}
