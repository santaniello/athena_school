package exercise

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"

	domainmocks "github.com/santaniello/athena/internal/domain/exercise/mocks"
)

func newTestRubricService(t *testing.T) (*RubricService, *domainmocks.MockChallengeRepository, *domainmocks.MockRubricRepository) {
	t.Helper()
	challenges := domainmocks.NewMockChallengeRepository(t)
	rubrics := domainmocks.NewMockRubricRepository(t)
	service := NewRubricService(challenges, rubrics)
	service.now = func() time.Time { return fixedNow }
	counter := 0
	service.newID = func() string {
		counter++
		return fmt.Sprintf("id-%d", counter)
	}
	return service, challenges, rubrics
}

// mockRubricVersion is a typed matcher that accepts any RubricVersion and
// records it in out, so a test can compare what was stored with what was returned.
func mockRubricVersion(out *domainexercise.RubricVersion) interface{} {
	return mock.MatchedBy(func(version domainexercise.RubricVersion) bool {
		*out = version
		return true
	})
}

func criterionInput(key string, weight int) ChallengeCriterionInput {
	return ChallengeCriterionInput{
		Key:         key,
		Name:        "Name " + key,
		Description: "Description " + key,
		Weight:      weight,
		Levels:      [domainexercise.LevelCount]string{"zero", "one", "two", "three", "four"},
	}
}

func TestRubricService_SetChallengeRubric_firstRubricIsVersionOneMergedWithTheBase(t *testing.T) {
	// Given a challenge with no rubric yet
	service, challenges, rubrics := newTestRubricService(t)
	ctx := context.Background()
	challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{ID: "challenge-1"}, nil)
	rubrics.EXPECT().Latest(ctx, "challenge-1").Return(domainexercise.RubricVersion{}, domainexercise.ErrRubricNotFound)
	var stored domainexercise.RubricVersion
	rubrics.EXPECT().Create(ctx, mockRubricVersion(&stored)).Return(nil)
	mandatory := criterionInput("cache-policy", 20)
	mandatory.Mandatory = true

	// When setting its rubric with one challenge criterion
	got, err := service.SetChallengeRubric(ctx, "challenge-1", []ChallengeCriterionInput{mandatory})

	// Then version 1 is stored with the base criteria first, then the challenge one, weights summing to 100
	require.NoError(t, err)
	assert.Equal(t, stored, got)
	assert.Equal(t, 1, got.Version)
	assert.Equal(t, "challenge-1", got.ChallengeID)
	assert.Equal(t, domainexercise.BaseRubricVersion, got.BaseVersion)
	assert.Equal(t, fixedNow, got.CreatedAt)
	require.Len(t, got.Criteria, 6)
	assert.Equal(t, "architecture", got.Criteria[0].Key)
	assert.Equal(t, domainexercise.CriterionSourceChallenge, got.Criteria[5].Source)
	assert.True(t, got.Criteria[5].Mandatory)
	assert.Equal(t, [domainexercise.LevelCount]string{"zero", "one", "two", "three", "four"}, got.Criteria[5].Levels)
	assert.NoError(t, got.Validate())
}

func TestRubricService_SetChallengeRubric_assignsADistinctIDToTheVersionAndEveryCriterion(t *testing.T) {
	// Given a service generating sequential ids
	service, challenges, rubrics := newTestRubricService(t)
	ctx := context.Background()
	challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{ID: "challenge-1"}, nil)
	rubrics.EXPECT().Latest(ctx, "challenge-1").Return(domainexercise.RubricVersion{}, domainexercise.ErrRubricNotFound)
	rubrics.EXPECT().Create(ctx, mockRubricVersion(new(domainexercise.RubricVersion))).Return(nil)

	// When setting a rubric
	got, err := service.SetChallengeRubric(ctx, "challenge-1", []ChallengeCriterionInput{criterionInput("a", 10)})

	// Then the version and each of its six criteria have different, non-empty ids
	require.NoError(t, err)
	ids := map[string]struct{}{got.ID: {}}
	for _, criterion := range got.Criteria {
		ids[criterion.ID] = struct{}{}
	}
	assert.NotContains(t, ids, "")
	assert.Len(t, ids, 7)
}

func TestRubricService_SetChallengeRubric_secondCallCreatesTheNextVersion(t *testing.T) {
	// Given a challenge whose latest rubric is version 1
	service, challenges, rubrics := newTestRubricService(t)
	ctx := context.Background()
	challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{ID: "challenge-1"}, nil)
	rubrics.EXPECT().Latest(ctx, "challenge-1").Return(domainexercise.RubricVersion{ID: "old", Version: 1}, nil)
	rubrics.EXPECT().Create(ctx, mockRubricVersion(new(domainexercise.RubricVersion))).Return(nil)

	// When setting its rubric again
	got, err := service.SetChallengeRubric(ctx, "challenge-1", []ChallengeCriterionInput{criterionInput("a", 10)})

	// Then the new rubric is version 2, a new row, and nothing updates version 1
	require.NoError(t, err)
	assert.Equal(t, 2, got.Version)
	assert.NotEqual(t, "old", got.ID)
}

func TestRubricService_SetChallengeRubric_trimsCriterionText(t *testing.T) {
	// Given a criterion with padded text
	service, challenges, rubrics := newTestRubricService(t)
	ctx := context.Background()
	challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{ID: "challenge-1"}, nil)
	rubrics.EXPECT().Latest(ctx, "challenge-1").Return(domainexercise.RubricVersion{}, domainexercise.ErrRubricNotFound)
	rubrics.EXPECT().Create(ctx, mockRubricVersion(new(domainexercise.RubricVersion))).Return(nil)
	input := ChallengeCriterionInput{
		Key: " cache ", Name: " Cache ", Description: " Policy ", Weight: 10,
		Levels: [domainexercise.LevelCount]string{" a ", " b ", " c ", " d ", " e "},
	}

	// When setting the rubric
	got, err := service.SetChallengeRubric(ctx, "challenge-1", []ChallengeCriterionInput{input})

	// Then the stored text is trimmed
	require.NoError(t, err)
	criterion := got.Criteria[5]
	assert.Equal(t, "cache", criterion.Key)
	assert.Equal(t, "Cache", criterion.Name)
	assert.Equal(t, "Policy", criterion.Description)
	assert.Equal(t, [domainexercise.LevelCount]string{"a", "b", "c", "d", "e"}, criterion.Levels)
}

func TestRubricService_SetChallengeRubric_unknownChallengeStoresNothing(t *testing.T) {
	// Given a challenge that does not exist
	service, challenges, _ := newTestRubricService(t)
	ctx := context.Background()
	challenges.EXPECT().Get(ctx, "missing").Return(domainexercise.Challenge{}, domainexercise.ErrChallengeNotFound)

	// When setting its rubric
	_, err := service.SetChallengeRubric(ctx, "missing", []ChallengeCriterionInput{criterionInput("a", 10)})

	// Then ErrChallengeNotFound is returned and no rubric is read or written
	assert.ErrorIs(t, err, domainexercise.ErrChallengeNotFound)
}

func TestRubricService_SetChallengeRubric_invalidCriteriaStoreNothing(t *testing.T) {
	blank := criterionInput("a", 10)
	blank.Name = "  "
	tests := []struct {
		name  string
		input []ChallengeCriterionInput
		want  error
	}{
		{"no criteria", nil, domainexercise.ErrChallengeCriteriaCount},
		{"base key reused", []ChallengeCriterionInput{criterionInput("resilience", 10)}, domainexercise.ErrCriterionKeyDuplicate},
		{"bad weight", []ChallengeCriterionInput{criterionInput("a", 0)}, domainexercise.ErrCriterionWeightInvalid},
		{"blank name", []ChallengeCriterionInput{blank}, domainexercise.ErrCriterionTextRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given an existing challenge and invalid criteria
			service, challenges, _ := newTestRubricService(t)
			ctx := context.Background()
			challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{ID: "challenge-1"}, nil)

			// When setting the rubric
			_, err := service.SetChallengeRubric(ctx, "challenge-1", tt.input)

			// Then the domain error is returned and nothing is looked up or stored
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestRubricService_SetChallengeRubric_versionConflictRetriesWithTheNewLatest(t *testing.T) {
	// Given another call took version 2 between this call's read and write
	service, challenges, rubrics := newTestRubricService(t)
	ctx := context.Background()
	challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{ID: "challenge-1"}, nil)
	rubrics.EXPECT().Latest(ctx, "challenge-1").Return(domainexercise.RubricVersion{Version: 1}, nil).Once()
	rubrics.EXPECT().Create(ctx, mockRubricVersion(new(domainexercise.RubricVersion))).Return(domainexercise.ErrRubricVersionConflict).Once()
	rubrics.EXPECT().Latest(ctx, "challenge-1").Return(domainexercise.RubricVersion{Version: 2}, nil).Once()
	rubrics.EXPECT().Create(ctx, mockRubricVersion(new(domainexercise.RubricVersion))).Return(nil).Once()

	// When setting the rubric
	got, err := service.SetChallengeRubric(ctx, "challenge-1", []ChallengeCriterionInput{criterionInput("a", 10)})

	// Then it re-reads the latest version and stores the next one, version 3
	require.NoError(t, err)
	assert.Equal(t, 3, got.Version)
}

func TestRubricService_SetChallengeRubric_givesUpAfterRepeatedVersionConflicts(t *testing.T) {
	// Given every attempt to store collides with another call
	service, challenges, rubrics := newTestRubricService(t)
	ctx := context.Background()
	challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{ID: "challenge-1"}, nil)
	rubrics.EXPECT().Latest(ctx, "challenge-1").Return(domainexercise.RubricVersion{Version: 1}, nil).Times(maxVersionAttempts)
	rubrics.EXPECT().Create(ctx, mockRubricVersion(new(domainexercise.RubricVersion))).
		Return(domainexercise.ErrRubricVersionConflict).Times(maxVersionAttempts)

	// When setting the rubric
	got, err := service.SetChallengeRubric(ctx, "challenge-1", []ChallengeCriterionInput{criterionInput("a", 10)})

	// Then it stops after the maximum attempts and reports the typed conflict
	assert.ErrorIs(t, err, domainexercise.ErrRubricVersionConflict)
	assert.Empty(t, got.ID)
}

func TestRubricService_SetChallengeRubric_storageFailuresAreReturned(t *testing.T) {
	boom := errors.New("boom")
	input := []ChallengeCriterionInput{criterionInput("a", 10)}

	t.Run("reading the challenge fails", func(t *testing.T) {
		// Given a challenge repository that fails
		service, challenges, _ := newTestRubricService(t)
		challenges.EXPECT().Get(context.Background(), "challenge-1").Return(domainexercise.Challenge{}, boom)

		// When setting the rubric
		_, err := service.SetChallengeRubric(context.Background(), "challenge-1", input)

		// Then the failure is returned
		assert.ErrorIs(t, err, boom)
	})
	t.Run("reading the latest rubric fails", func(t *testing.T) {
		// Given a rubric repository whose Latest fails with something other than not-found
		service, challenges, rubrics := newTestRubricService(t)
		ctx := context.Background()
		challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{ID: "challenge-1"}, nil)
		rubrics.EXPECT().Latest(ctx, "challenge-1").Return(domainexercise.RubricVersion{}, boom)

		// When setting the rubric
		_, err := service.SetChallengeRubric(ctx, "challenge-1", input)

		// Then the failure is returned and no version is created
		assert.ErrorIs(t, err, boom)
	})
	t.Run("storing the rubric fails", func(t *testing.T) {
		// Given a rubric repository whose Create fails
		service, challenges, rubrics := newTestRubricService(t)
		ctx := context.Background()
		challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{ID: "challenge-1"}, nil)
		rubrics.EXPECT().Latest(ctx, "challenge-1").Return(domainexercise.RubricVersion{}, domainexercise.ErrRubricNotFound)
		rubrics.EXPECT().Create(ctx, mockRubricVersion(new(domainexercise.RubricVersion))).Return(boom)

		// When setting the rubric
		got, err := service.SetChallengeRubric(ctx, "challenge-1", input)

		// Then the failure is returned with no rubric
		assert.ErrorIs(t, err, boom)
		assert.Empty(t, got.ID)
	})
}

func TestRubricService_reads_delegateToTheRepository(t *testing.T) {
	ctx := context.Background()
	version := domainexercise.RubricVersion{ID: "rubric-1", ChallengeID: "challenge-1", Version: 3}
	boom := errors.New("boom")

	t.Run("Get", func(t *testing.T) {
		// Given a stored version
		service, _, rubrics := newTestRubricService(t)
		rubrics.EXPECT().Get(ctx, "rubric-1").Return(version, nil)

		// When getting it
		got, err := service.Get(ctx, "rubric-1")

		// Then it is returned
		require.NoError(t, err)
		assert.Equal(t, version, got)
	})
	t.Run("Get not found", func(t *testing.T) {
		// Given no such version
		service, _, rubrics := newTestRubricService(t)
		rubrics.EXPECT().Get(ctx, "missing").Return(domainexercise.RubricVersion{}, domainexercise.ErrRubricNotFound)

		// When getting it
		_, err := service.Get(ctx, "missing")

		// Then ErrRubricNotFound is preserved
		assert.ErrorIs(t, err, domainexercise.ErrRubricNotFound)
	})
	t.Run("Latest", func(t *testing.T) {
		// Given a challenge with a rubric
		service, _, rubrics := newTestRubricService(t)
		rubrics.EXPECT().Latest(ctx, "challenge-1").Return(version, nil)

		// When asking for the latest
		got, err := service.Latest(ctx, "challenge-1")

		// Then it is returned
		require.NoError(t, err)
		assert.Equal(t, version, got)
	})
	t.Run("Latest without a rubric", func(t *testing.T) {
		// Given a challenge without a rubric
		service, _, rubrics := newTestRubricService(t)
		rubrics.EXPECT().Latest(ctx, "challenge-2").Return(domainexercise.RubricVersion{}, domainexercise.ErrRubricNotFound)

		// When asking for the latest
		_, err := service.Latest(ctx, "challenge-2")

		// Then ErrRubricNotFound is preserved
		assert.ErrorIs(t, err, domainexercise.ErrRubricNotFound)
	})
	t.Run("ListVersions", func(t *testing.T) {
		// Given two stored versions
		service, _, rubrics := newTestRubricService(t)
		rubrics.EXPECT().ListVersions(ctx, "challenge-1").Return([]domainexercise.RubricVersion{version, version}, nil)

		// When listing them
		got, err := service.ListVersions(ctx, "challenge-1")

		// Then both are returned
		require.NoError(t, err)
		assert.Len(t, got, 2)
	})
	t.Run("ListVersions fails", func(t *testing.T) {
		// Given a failing repository
		service, _, rubrics := newTestRubricService(t)
		rubrics.EXPECT().ListVersions(ctx, "challenge-1").Return(nil, boom)

		// When listing
		_, err := service.ListVersions(ctx, "challenge-1")

		// Then the failure is returned
		assert.ErrorIs(t, err, boom)
	})
}
