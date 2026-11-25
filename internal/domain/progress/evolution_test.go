package progress

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func day(d int) time.Time { return time.Date(2026, 11, d, 10, 0, 0, 0, time.UTC) }

func TestBuildEvolution_numbersEachSeriesChronologicallyAcrossItems(t *testing.T) {
	// Given two quizzes whose attempts interleave in time, and a challenge with one attempt
	in := TopicInput{
		Quizzes: []QuizInput{
			{QuizID: "q1", Attempts: []ScoredAttempt{{Percent: 50, At: day(1)}, {Percent: 90, At: day(5)}}},
			{QuizID: "q2", Attempts: []ScoredAttempt{{Percent: 70, At: day(3)}}},
		},
		Challenges: []ChallengeInput{
			{ChallengeID: "c1", Title: "Design a cache", Attempts: []LabAttempt{{DiagramScore: 80, UnderstandingScore: 71, At: day(2)}}},
		},
	}

	// When the evolution is built
	evolution := BuildEvolution(in, 80)

	// Then each series is ordered by date and numbered on its own
	assert.Equal(t, []EvolutionPoint{
		{Order: 1, ItemNumber: 1, Score: 50, At: day(1)},
		{Order: 2, ItemNumber: 2, Score: 70, At: day(3)},
		{Order: 3, ItemNumber: 1, Score: 90, At: day(5)},
	}, evolution.Quiz)
	assert.Equal(t, []EvolutionPoint{
		{Order: 1, ItemNumber: 1, Title: "Design a cache", Score: 76, At: day(2)},
	}, evolution.Lab)
	assert.Equal(t, 80, evolution.PassScore)
	assert.True(t, evolution.Enough)
}

func TestBuildEvolution_isNotEnoughWithASinglePointInTotal(t *testing.T) {
	// Given a topic with a single attempt
	in := TopicInput{Quizzes: []QuizInput{{QuizID: "q1", Attempts: []ScoredAttempt{{Percent: 50, At: day(1)}}}}}

	// When the evolution is built
	evolution := BuildEvolution(in, 80)

	// Then it is not enough and the empty series are non-nil
	assert.False(t, evolution.Enough)
	assert.Len(t, evolution.Quiz, 1)
	assert.NotNil(t, evolution.Lab)
	assert.Empty(t, evolution.Lab)
}

func TestBuildEvolution_countsPointsOfBothSeriesTowardsEnough(t *testing.T) {
	// Given one quiz attempt and one lab attempt
	in := TopicInput{
		Quizzes:    []QuizInput{{QuizID: "q1", Attempts: []ScoredAttempt{{Percent: 50, At: day(1)}}}},
		Challenges: []ChallengeInput{{ChallengeID: "c1", Attempts: []LabAttempt{{DiagramScore: 60, UnderstandingScore: 60, At: day(2)}}}},
	}

	// When / Then two points in total are enough
	assert.True(t, BuildEvolution(in, 80).Enough)
}

func TestBuildEvolution_keepsTheOrderOfEqualDates(t *testing.T) {
	// Given two attempts at the same instant
	in := TopicInput{Quizzes: []QuizInput{
		{QuizID: "q1", Attempts: []ScoredAttempt{{Percent: 10, At: day(1)}}},
		{QuizID: "q2", Attempts: []ScoredAttempt{{Percent: 20, At: day(1)}}},
	}}

	// When the evolution is built
	evolution := BuildEvolution(in, 80)

	// Then the order of the quizzes decides
	assert.Equal(t, []int{10, 20}, []int{evolution.Quiz[0].Score, evolution.Quiz[1].Score})
}
