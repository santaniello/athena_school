package exercise

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChallengeSpec_Normalized_defaultsToAnInterviewAndDropsItsGoal(t *testing.T) {
	// Given a spec with no kind and a stray goal
	spec := ChallengeSpec{Difficulty: DifficultyBeginner, PracticeGoal: "cache aside"}

	// When normalizing it
	got := spec.Normalized()

	// Then it is an interview without a goal
	assert.Equal(t, ChallengeSpec{Difficulty: DifficultyBeginner, Kind: ChallengeKindInterview}, got)
}

func TestChallengeSpec_Normalized_trimsThePracticeGoal(t *testing.T) {
	// Given a practice spec whose goal has surrounding blanks
	spec := ChallengeSpec{Difficulty: DifficultyBeginner, Kind: ChallengeKindPractice, PracticeGoal: "  cache aside \n"}

	// When normalizing it
	got := spec.Normalized()

	// Then the goal is trimmed and the kind is kept
	assert.Equal(t, "cache aside", got.PracticeGoal)
	assert.Equal(t, ChallengeKindPractice, got.Kind)
}

func TestChallengeSpec_Validate(t *testing.T) {
	tests := []struct {
		name string
		spec ChallengeSpec
		want error
	}{
		{"interview", ChallengeSpec{Difficulty: DifficultyAdvanced, Kind: ChallengeKindInterview}, nil},
		{"practice with a goal", ChallengeSpec{Difficulty: DifficultyBeginner, Kind: ChallengeKindPractice, PracticeGoal: "cache aside"}, nil},
		{"practice goal at the limit", ChallengeSpec{Difficulty: DifficultyBeginner, Kind: ChallengeKindPractice, PracticeGoal: strings.Repeat("g", MaxPracticeGoalLength)}, nil},
		{"unknown difficulty", ChallengeSpec{Difficulty: "EXPERT", Kind: ChallengeKindInterview}, ErrDifficultyInvalid},
		{"empty difficulty", ChallengeSpec{Kind: ChallengeKindInterview}, ErrDifficultyInvalid},
		{"unknown kind", ChallengeSpec{Difficulty: DifficultyBeginner, Kind: "QUIZ"}, ErrChallengeKindInvalid},
		{"practice without a goal", ChallengeSpec{Difficulty: DifficultyBeginner, Kind: ChallengeKindPractice}, ErrPracticeGoalRequired},
		{"practice goal too long", ChallengeSpec{Difficulty: DifficultyBeginner, Kind: ChallengeKindPractice, PracticeGoal: strings.Repeat("g", MaxPracticeGoalLength+1)}, ErrPracticeGoalTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a spec
			// When validating it
			err := tt.spec.Validate()

			// Then the specific error, or none, is returned
			if tt.want == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.want)
		})
	}
}
