package exercise

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalProblems_EveryProblemIsComplete(t *testing.T) {
	// Given the shipped list of canonical problems
	problems := CanonicalProblems()

	// Then there are the six first problems, in a fixed order
	ids := make([]string, len(problems))
	for i, problem := range problems {
		ids[i] = problem.ID
	}
	assert.Equal(t, []string{"url-shortener", "rate-limiter", "news-feed", "chat-application", "notification-service", "ticket-booking"}, ids)

	seen := map[string]bool{}
	for _, problem := range problems {
		t.Run(problem.ID, func(t *testing.T) {
			assert.False(t, seen[problem.ID], "duplicated id")
			seen[problem.ID] = true
			assert.NotEmpty(t, problem.Name.EN)
			assert.NotEmpty(t, problem.Name.PT)
			assert.NotEmpty(t, problem.Summary.EN)
			assert.NotEmpty(t, problem.Summary.PT)
			assert.Contains(t, []Difficulty{DifficultyBeginner, DifficultyIntermediate, DifficultyAdvanced}, problem.Level)
			assert.GreaterOrEqual(t, len(problem.Core), 2)
			assert.LessOrEqual(t, len(problem.Core), 4)
			for _, difficulty := range []Difficulty{DifficultyBeginner, DifficultyIntermediate, DifficultyAdvanced} {
				assert.NotEmpty(t, problem.Scale[difficulty], difficulty)
			}
		})
	}
}

func TestCanonicalProblems_ReturnsACopy(t *testing.T) {
	// Given a caller that changes the list it received
	first := CanonicalProblems()
	first[0].ID = "changed"
	first[0].Core[0] = "changed"

	// When the list is read again
	second := CanonicalProblems()

	// Then it is untouched
	assert.Equal(t, "url-shortener", second[0].ID)
	assert.NotEqual(t, "changed", second[0].Core[0])
}

func TestFindCanonicalProblem(t *testing.T) {
	t.Run("returns the problem with that id", func(t *testing.T) {
		problem, err := FindCanonicalProblem("rate-limiter")

		require.NoError(t, err)
		assert.Equal(t, "rate-limiter", problem.ID)
	})

	t.Run("fails for an unknown id", func(t *testing.T) {
		_, err := FindCanonicalProblem("nope")

		assert.ErrorIs(t, err, ErrCanonicalProblemNotFound)
	})
}

func TestLocalizedName_For(t *testing.T) {
	name := LocalizedName{EN: "Rate limiter", PT: "Limitador de taxa"}

	assert.Equal(t, "Limitador de taxa", name.For("pt"))
	assert.Equal(t, "Rate limiter", name.For("en"))
	assert.Equal(t, "Rate limiter", name.For(""))
}

func TestCanonicalProblems_ClassifyEachProblemByItsLevel(t *testing.T) {
	levels := map[string]Difficulty{}
	for _, problem := range CanonicalProblems() {
		levels[problem.ID] = problem.Level
	}

	assert.Equal(t, map[string]Difficulty{
		"url-shortener":        DifficultyBeginner,
		"rate-limiter":         DifficultyIntermediate,
		"news-feed":            DifficultyAdvanced,
		"chat-application":     DifficultyAdvanced,
		"notification-service": DifficultyIntermediate,
		"ticket-booking":       DifficultyIntermediate,
	}, levels)
}
