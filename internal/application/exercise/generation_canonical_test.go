package exercise

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// canonicalOutput is a model answer for a canonical problem: no title.
func canonicalOutput() map[string]any {
	output := validOutput()
	delete(output, "title")
	return output
}

func (f *generationFixture) expectCanonicalChat(ctx context.Context, difficulty domainexercise.Difficulty, content string, err error) {
	problem, findErr := domainexercise.FindCanonicalProblem("url-shortener")
	require.NoError(f.t, findErr)
	f.llm.EXPECT().Chat(ctx, domainllm.ChatRequest{
		Task:     domainllm.TaskArchitectureLab,
		Messages: buildCanonicalPrompt(problem, difficulty, testProfile),
	}).Return(domainllm.ChatResponse{Content: content}, err).Once()
}

// expectedCanonicalChallenge is the challenge canonicalOutput produces for the
// URL shortener at INTERMEDIATE difficulty, with the fixture's id and clock and
// a title in the profile's language (Portuguese).
func expectedCanonicalChallenge() domainexercise.Challenge {
	return domainexercise.Challenge{
		ID:                        "challenge-1",
		Type:                      domainexercise.ChallengeTypeArchitecture,
		Kind:                      domainexercise.ChallengeKindInterview,
		Title:                     "Encurtador de URL",
		Scenario:                  "A shop serves 50k requests per second worldwide.",
		FunctionalRequirements:    []string{"Serve product pages"},
		NonFunctionalRequirements: []string{"p99 under 100 ms"},
		Constraints:               []string{"Three regions"},
		Objectives:                []string{"Justify the eviction policy"},
		Difficulty:                domainexercise.DifficultyIntermediate,
		Topic:                     "url shortener",
		CanonicalProblemID:        "url-shortener",
		Origin:                    domainexercise.OriginAIGenerated,
		TimeLimitMinutes:          45,
		DefenseQuestions:          5,
		CreatedAt:                 fixedNow,
		UpdatedAt:                 fixedNow,
	}
}

func expectedCanonicalRubric(t *testing.T) domainexercise.RubricVersion {
	t.Helper()
	generated, err := parseGeneration(encode(t, validOutput()), "url shortener", interviewSpec(domainexercise.DifficultyIntermediate), nil)
	require.NoError(t, err)
	criteria := make([]domainexercise.RubricCriterion, len(generated.Criteria))
	for i, input := range generated.Criteria {
		criteria[i] = domainexercise.RubricCriterion{
			Key: input.Key, Name: input.Name, Description: input.Description,
			Weight: input.Weight, Mandatory: input.Mandatory, Levels: input.Levels,
		}
	}
	merged, err := domainexercise.MergeRubric(domainexercise.BaseRubric(), criteria)
	require.NoError(t, err)
	for i := range merged {
		merged[i].ID = fmt.Sprintf("id-%d", i+1)
	}
	return domainexercise.RubricVersion{
		ID: fmt.Sprintf("id-%d", len(merged)+1), ChallengeID: "challenge-1", Version: 1,
		BaseVersion: domainexercise.BaseRubricVersion, Criteria: merged, CreatedAt: fixedNow,
	}
}

func TestGenerationService_GenerateCanonical_generatesAndSavesWithTheTitleFromCode(t *testing.T) {
	// Given no challenge yet for the problem and difficulty, and a model that even writes a title of its own
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.challenges.EXPECT().FindCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate).Return(domainexercise.Challenge{}, domainexercise.ErrChallengeNotFound).Once()
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil)
	output := canonicalOutput()
	output["title"] = "Use Redis and a CDN"
	f.expectCanonicalChat(ctx, domainexercise.DifficultyIntermediate, encode(t, output), nil)
	f.runTx(ctx)
	stored := expectedCanonicalChallenge()
	f.challenges.EXPECT().Create(insideTx(), stored).Return(nil)
	f.challenges.EXPECT().Get(insideTx(), "challenge-1").Return(stored, nil)
	f.rubrics.EXPECT().Latest(insideTx(), "challenge-1").Return(domainexercise.RubricVersion{}, domainexercise.ErrRubricNotFound)
	rubric := expectedCanonicalRubric(t)
	f.rubrics.EXPECT().Create(insideTx(), rubric).Return(nil)

	// When the canonical challenge is generated
	got, err := f.service.GenerateCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate)

	// Then it has no session, the problem's id, and the title is the problem's name in the assistant language
	require.NoError(t, err)
	assert.Equal(t, stored, got)
	assert.Empty(t, got.SourceSessionID)
	assert.Equal(t, "url-shortener", got.CanonicalProblemID)
	assert.Equal(t, "Encurtador de URL", got.Title)
	assert.Equal(t, domainexercise.OriginAIGenerated, got.Origin)

	// And its first rubric version holds the base criteria plus the generated ones
	require.Len(t, rubric.Criteria, 7)
}

func TestGenerationService_GenerateCanonical_reusesTheStoredChallengeWithNoModelCall(t *testing.T) {
	// Given a challenge already stored for the problem and difficulty
	f := newGenerationFixture(t)
	ctx := context.Background()
	stored := expectedCanonicalChallenge()
	f.challenges.EXPECT().FindCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate).Return(stored, nil)

	// When it is generated again
	got, err := f.service.GenerateCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate)

	// Then the stored one is returned and nothing else is touched (strict mocks)
	require.NoError(t, err)
	assert.Equal(t, stored, got)
}

func TestGenerationService_GenerateCanonical_refusesAnUnknownProblemOrDifficultyBeforeAnyRead(t *testing.T) {
	f := newGenerationFixture(t)
	ctx := context.Background()

	_, unknownProblem := f.service.GenerateCanonical(ctx, "nope", domainexercise.DifficultyIntermediate)
	_, unknownDifficulty := f.service.GenerateCanonical(ctx, "url-shortener", domainexercise.Difficulty("EXPERT"))

	assert.ErrorIs(t, unknownProblem, domainexercise.ErrCanonicalProblemNotFound)
	assert.ErrorIs(t, unknownDifficulty, domainexercise.ErrDifficultyInvalid)
}

func TestGenerationService_GenerateCanonical_failsWhenTheLookupFails(t *testing.T) {
	f := newGenerationFixture(t)
	ctx := context.Background()
	lookupFailure := errors.New("database is locked")
	f.challenges.EXPECT().FindCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate).Return(domainexercise.Challenge{}, lookupFailure)

	_, err := f.service.GenerateCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate)

	assert.ErrorIs(t, err, lookupFailure)
}

func TestGenerationService_GenerateCanonical_failsWhenTheProfileCannotBeRead(t *testing.T) {
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.challenges.EXPECT().FindCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate).Return(domainexercise.Challenge{}, domainexercise.ErrChallengeNotFound)
	profileFailure := errors.New("no profile")
	f.profiles.EXPECT().ReadProfile(ctx).Return(domainexercise.GenerationProfile{}, profileFailure)

	_, err := f.service.GenerateCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate)

	assert.ErrorIs(t, err, profileFailure)
}

func TestGenerationService_GenerateCanonical_storesNothingWhenTheModelFails(t *testing.T) {
	for name, failure := range map[string]error{"provider error": errors.New("openrouter is down"), "cancelled": context.Canceled} {
		t.Run(name, func(t *testing.T) {
			// Given a model call that fails
			f := newGenerationFixture(t)
			ctx := context.Background()
			f.challenges.EXPECT().FindCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate).Return(domainexercise.Challenge{}, domainexercise.ErrChallengeNotFound)
			f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil)
			f.expectCanonicalChat(ctx, domainexercise.DifficultyIntermediate, "", failure)

			// When generating
			_, err := f.service.GenerateCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate)

			// Then the error comes back and no transaction is opened (strict mocks)
			require.ErrorIs(t, err, failure)
		})
	}
}

func TestGenerationService_GenerateCanonical_storesNothingWhenTheOutputIsInvalid(t *testing.T) {
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.challenges.EXPECT().FindCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate).Return(domainexercise.Challenge{}, domainexercise.ErrChallengeNotFound)
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil)
	f.expectCanonicalChat(ctx, domainexercise.DifficultyIntermediate, "I cannot do that.", nil)

	_, err := f.service.GenerateCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate)

	require.ErrorIs(t, err, ErrInvalidGeneration)
}

func TestGenerationService_GenerateCanonical_returnsTheErrorWhenTheRubricCannotBeSaved(t *testing.T) {
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.challenges.EXPECT().FindCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate).Return(domainexercise.Challenge{}, domainexercise.ErrChallengeNotFound)
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil)
	f.expectCanonicalChat(ctx, domainexercise.DifficultyIntermediate, encode(t, canonicalOutput()), nil)
	f.runTx(ctx)
	stored := expectedCanonicalChallenge()
	f.challenges.EXPECT().Create(insideTx(), stored).Return(nil)
	f.challenges.EXPECT().Get(insideTx(), "challenge-1").Return(stored, nil)
	f.rubrics.EXPECT().Latest(insideTx(), "challenge-1").Return(domainexercise.RubricVersion{}, domainexercise.ErrRubricNotFound)
	saveFailure := errors.New("disk full")
	f.rubrics.EXPECT().Create(insideTx(), expectedCanonicalRubric(t)).Return(saveFailure)

	_, err := f.service.GenerateCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate)

	require.ErrorIs(t, err, saveFailure)
}

func TestGenerationService_GenerateCanonical_returnsTheWinnerWhenAnotherGenerationStoredFirst(t *testing.T) {
	// Given two generations racing: the index refuses this one's challenge
	f := newGenerationFixture(t)
	ctx := context.Background()
	winner := expectedCanonicalChallenge()
	winner.ID = "other"
	f.challenges.EXPECT().FindCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate).Return(domainexercise.Challenge{}, domainexercise.ErrChallengeNotFound).Once()
	f.challenges.EXPECT().FindCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate).Return(winner, nil).Once()
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil)
	f.expectCanonicalChat(ctx, domainexercise.DifficultyIntermediate, encode(t, canonicalOutput()), nil)
	f.runTx(ctx)
	f.challenges.EXPECT().Create(insideTx(), expectedCanonicalChallenge()).Return(domainexercise.ErrCanonicalChallengeExists)

	// When generating
	got, err := f.service.GenerateCanonical(ctx, "url-shortener", domainexercise.DifficultyIntermediate)

	// Then the challenge that won the race is returned: one per problem and difficulty
	require.NoError(t, err)
	assert.Equal(t, winner, got)
}
