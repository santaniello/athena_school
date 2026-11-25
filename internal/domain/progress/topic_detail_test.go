package progress

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func concept(name string, status ConceptStatus, day int) DetailConcept {
	return DetailConcept{Name: name, Status: status, UpdatedAt: at(day)}
}

func labAttempt(diagram, defense *int, result LabResult, day int, weak ...string) DetailLabAttempt {
	return DetailLabAttempt{DiagramScore: diagram, DefenseScore: defense, Result: result, WeakCriteria: weak, At: at(day)}
}

func TestBuildTopicDetail_Header(t *testing.T) {
	in := TopicDetailInput{
		SessionID: "s1", Name: "Cache", FolderName: "System Design",
		Quizzes:    []DetailQuiz{{QuizID: "q1"}, {QuizID: "q2"}},
		Challenges: []DetailChallenge{{ChallengeID: "c1", Title: "Design a cache"}},
		Concepts:   []DetailConcept{concept("a", StatusIntroduced, 1), concept("b", StatusDemonstrated, 2), concept("c", StatusNotAssessed, 3)},
	}

	detail := BuildTopicDetail(in)

	assert.Equal(t, "s1", detail.SessionID)
	assert.Equal(t, "Cache", detail.Name)
	assert.Equal(t, "System Design", detail.FolderName)
	assert.Equal(t, 2, detail.QuizCount)
	assert.Equal(t, 1, detail.ChallengeCount)
	assert.Equal(t, 3, detail.ConceptCount)
}

func TestBuildTopicDetail_QuizResults(t *testing.T) {
	in := TopicDetailInput{Quizzes: []DetailQuiz{
		{QuizID: "q1", Difficulty: "BEGINNER", Attempts: []ScoredAttempt{{Percent: 60, At: at(1)}, {Percent: 90, At: at(2)}}},
		{QuizID: "q2", Difficulty: "ADVANCED"},
	}}

	detail := BuildTopicDetail(in)

	require.Len(t, detail.Quizzes, 2)
	assert.Equal(t, QuizResult{QuizID: "q1", Number: 1, Difficulty: "BEGINNER", Percent: ptr(90), Attempts: 2}, detail.Quizzes[0])
	assert.Equal(t, QuizResult{QuizID: "q2", Number: 2, Difficulty: "ADVANCED", Percent: nil, Attempts: 0}, detail.Quizzes[1])
}

func TestBuildTopicDetail_NoQuizLeavesEmptyNonNilList(t *testing.T) {
	detail := BuildTopicDetail(TopicDetailInput{})

	assert.NotNil(t, detail.Quizzes)
	assert.Empty(t, detail.Quizzes)
	assert.NotNil(t, detail.Challenges)
	assert.NotNil(t, detail.Understood)
	assert.NotNil(t, detail.Difficulties)
	assert.NotNil(t, detail.ExplainedOnly)
}

func TestBuildTopicDetail_ChallengeResultUsesLatestEvaluatedAttempt(t *testing.T) {
	in := TopicDetailInput{Challenges: []DetailChallenge{{
		ChallengeID: "c1", Title: "Design a cache", Difficulty: "INTERMEDIATE", TimeLimitMinutes: 45,
		Attempts: []DetailLabAttempt{
			labAttempt(ptr(50), ptr(40), LabResultFailed, 1),
			labAttempt(ptr(85), ptr(90), LabResultPassed, 2),
			labAttempt(nil, nil, LabResultPending, 3), // started, never evaluated
		},
	}}}

	detail := BuildTopicDetail(in)

	require.Len(t, detail.Challenges, 1)
	assert.Equal(t, ChallengeResult{
		ChallengeID: "c1", Title: "Design a cache", Difficulty: "INTERMEDIATE", TimeLimitMinutes: 45,
		DiagramScore: ptr(85), DefenseScore: ptr(90), Result: LabResultPassed, Attempts: 3,
	}, detail.Challenges[0])
}

func TestBuildTopicDetail_ChallengeWithoutDefenseIsPending(t *testing.T) {
	in := TopicDetailInput{Challenges: []DetailChallenge{{
		ChallengeID: "c1", Attempts: []DetailLabAttempt{labAttempt(ptr(70), nil, LabResultPending, 1)},
	}}}

	result := BuildTopicDetail(in).Challenges[0]

	assert.Equal(t, ptr(70), result.DiagramScore)
	assert.Nil(t, result.DefenseScore)
	assert.Equal(t, LabResultPending, result.Result)
}

func TestBuildTopicDetail_ChallengeWithoutEvaluatedAttemptHasNoScores(t *testing.T) {
	in := TopicDetailInput{Challenges: []DetailChallenge{
		{ChallengeID: "c1"},
		{ChallengeID: "c2", Attempts: []DetailLabAttempt{labAttempt(nil, nil, LabResultPending, 1)}},
	}}

	detail := BuildTopicDetail(in)

	assert.Equal(t, ChallengeResult{ChallengeID: "c1", Result: LabResultPending}, detail.Challenges[0])
	assert.Equal(t, ChallengeResult{ChallengeID: "c2", Result: LabResultPending, Attempts: 1}, detail.Challenges[1])
}

func TestBuildTopicDetail_ConceptsAreGroupedByStatus(t *testing.T) {
	in := TopicDetailInput{Concepts: []DetailConcept{
		concept("demonstrated", StatusDemonstrated, 1),
		concept("introduced", StatusIntroduced, 2),
		concept("partial", StatusPartiallyUnderstood, 3),
		concept("review", StatusNeedsReview, 4),
		concept("not assessed", StatusNotAssessed, 5),
	}}

	detail := BuildTopicDetail(in)

	require.Len(t, detail.Understood, 1)
	assert.Equal(t, "demonstrated", detail.Understood[0].Name)
	require.Len(t, detail.ExplainedOnly, 1)
	assert.Equal(t, "introduced", detail.ExplainedOnly[0].Name)
	require.Len(t, detail.Difficulties, 2)
	assert.Equal(t, "review", detail.Difficulties[0].Name)
	assert.True(t, detail.Difficulties[0].ToReview)
	assert.Equal(t, DifficultySourceTutor, detail.Difficulties[0].Source)
	assert.Equal(t, "partial", detail.Difficulties[1].Name)
	assert.False(t, detail.Difficulties[1].ToReview)
}

func TestBuildTopicDetail_IntroducedConceptIsNeverUnderstood(t *testing.T) {
	in := TopicDetailInput{Concepts: []DetailConcept{concept("cache-aside", StatusIntroduced, 1)}}

	detail := BuildTopicDetail(in)

	assert.Empty(t, detail.Understood)
	assert.Empty(t, detail.Difficulties)
	assert.Len(t, detail.ExplainedOnly, 1)
}

func TestBuildTopicDetail_ConceptShowsItsDisplayNameOrTheNormalizedOne(t *testing.T) {
	withDisplay := concept("cache-aside", StatusDemonstrated, 1)
	withDisplay.DisplayName = "Cache-aside"
	legacy := concept("write-through", StatusDemonstrated, 2)

	detail := BuildTopicDetail(TopicDetailInput{Concepts: []DetailConcept{withDisplay, legacy}})

	require.Len(t, detail.Understood, 2)
	assert.Equal(t, "write-through", detail.Understood[0].Name)
	assert.Equal(t, "Cache-aside", detail.Understood[1].Name)
}

func TestBuildTopicDetail_ConceptsAreOrderedByRecentUpdateThenName(t *testing.T) {
	in := TopicDetailInput{Concepts: []DetailConcept{
		concept("b", StatusDemonstrated, 1),
		concept("a", StatusDemonstrated, 1),
		concept("c", StatusDemonstrated, 3),
	}}

	detail := BuildTopicDetail(in)

	names := []string{detail.Understood[0].Name, detail.Understood[1].Name, detail.Understood[2].Name}
	assert.Equal(t, []string{"c", "a", "b"}, names)
}

func TestBuildTopicDetail_ConceptListsItsProofsNewestFirst(t *testing.T) {
	c := concept("cache", StatusDemonstrated, 3)
	c.EvidenceCount = 2
	c.Proofs = []DetailProof{
		{Assessment: "PARTIALLY_CORRECT", Description: "old", At: at(1)},
		{Assessment: "CORRECT", Description: "new", Assisted: true, At: at(2)},
	}

	detail := BuildTopicDetail(TopicDetailInput{Concepts: []DetailConcept{c}})

	got := detail.Understood[0]
	assert.Equal(t, 2, got.EvidenceCount)
	require.Len(t, got.Proofs, 2)
	assert.Equal(t, "new", got.Proofs[0].Description)
	assert.True(t, got.Proofs[0].Assisted)
	assert.Equal(t, "old", got.Proofs[1].Description)
}

func TestBuildTopicDetail_WeakCriterionOfLatestEvaluatedAttemptIsADifficulty(t *testing.T) {
	in := TopicDetailInput{Challenges: []DetailChallenge{{
		ChallengeID: "c1", Title: "Design a cache",
		Attempts: []DetailLabAttempt{
			labAttempt(ptr(40), ptr(40), LabResultFailed, 1, "Scalability"),
			labAttempt(ptr(85), ptr(60), LabResultFailed, 2, "Consistency", "Failure handling"),
			labAttempt(nil, nil, LabResultPending, 3),
		},
	}}}

	detail := BuildTopicDetail(in)

	require.Len(t, detail.Difficulties, 2)
	assert.Equal(t, Difficulty{Name: "Consistency", Source: DifficultySourceArchLab, ChallengeTitle: "Design a cache"}, detail.Difficulties[0])
	assert.Equal(t, "Failure handling", detail.Difficulties[1].Name)
}

func TestBuildTopicDetail_TutorDifficultiesComeBeforeArchLabOnes(t *testing.T) {
	in := TopicDetailInput{
		Concepts: []DetailConcept{concept("partial", StatusPartiallyUnderstood, 1)},
		Challenges: []DetailChallenge{{
			ChallengeID: "c1", Title: "Lab", Attempts: []DetailLabAttempt{labAttempt(ptr(10), ptr(10), LabResultFailed, 1, "Scalability")},
		}},
	}

	detail := BuildTopicDetail(in)

	require.Len(t, detail.Difficulties, 2)
	assert.Equal(t, DifficultySourceTutor, detail.Difficulties[0].Source)
	assert.Equal(t, DifficultySourceArchLab, detail.Difficulties[1].Source)
}
