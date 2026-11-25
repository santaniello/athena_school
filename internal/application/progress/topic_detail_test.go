package progress

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainprogress "github.com/santaniello/athena/internal/domain/progress"
	domainstudy "github.com/santaniello/athena/internal/domain/study"

	progressmocks "github.com/santaniello/athena/internal/application/progress/mocks"
)

func topicService(t *testing.T, passScore int, topic TopicData) *Service {
	t.Helper()
	passScores := progressmocks.NewMockPassScoreProvider(t)
	passScores.EXPECT().Current(context.Background()).Return(passScore, nil)
	return NewService(fakeReader{topic: topic}, passScores)
}

func twoCriteriaRubric() *domainexercise.RubricVersion {
	return &domainexercise.RubricVersion{ID: "rv1", Criteria: []domainexercise.RubricCriterion{
		{ID: "k1", Name: "Scalability", Weight: 50},
		{ID: "k2", Name: "Consistency", Weight: 50},
	}}
}

func twoCriteriaEvaluation(kind domainexercise.EvaluationKind, score int, results ...domainexercise.CriterionResult) *domainexercise.Evaluation {
	return &domainexercise.Evaluation{Kind: kind, Score: score, Results: results}
}

func level(id string, level int, insufficient bool) domainexercise.CriterionResult {
	return domainexercise.CriterionResult{CriterionID: id, Level: level, InsufficientEvidence: insufficient}
}

func TestGetTopicDetail_returnsTheErrorOfThePassScore(t *testing.T) {
	// Given a pass score that cannot be verified
	passScores := progressmocks.NewMockPassScoreProvider(t)
	passScores.EXPECT().Current(context.Background()).Return(0, errors.New("config unreadable"))

	// When the detail is requested
	_, err := NewService(fakeReader{}, passScores).GetTopicDetail(context.Background(), "s1")

	// Then it fails
	require.ErrorContains(t, err, "config unreadable")
}

func TestGetTopicDetail_returnsTheErrorOfTheReader(t *testing.T) {
	// Given a reader that does not know the session
	passScores := progressmocks.NewMockPassScoreProvider(t)
	passScores.EXPECT().Current(context.Background()).Return(80, nil)
	service := NewService(fakeReader{topicErr: ErrTopicNotFound}, passScores)

	// When the detail is requested
	_, err := service.GetTopicDetail(context.Background(), "gone")

	// Then the not-found error is kept for the caller
	require.ErrorIs(t, err, ErrTopicNotFound)
}

func TestGetTopicDetail_mapsTheHeaderQuizzesAndConcepts(t *testing.T) {
	// Given a topic with a quiz, an INTRODUCED concept and a proof
	service := topicService(t, 80, TopicData{
		SessionID: "s1", Topic: "Cache", FolderName: "System Design",
		Quizzes: []TopicQuizData{{ID: "q1", Difficulty: "BEGINNER", Attempts: []QuizAttemptData{{CorrectCount: 3, Total: 4, FinishedAt: base}}}},
		Concepts: []TopicConceptData{{
			Name: "cache-aside", DisplayName: "Cache-aside", Status: domainstudy.StatusDemonstrated, EvidenceCount: 1, UpdatedAt: base,
			Proofs: []domainstudy.Evidence{{Assessment: domainstudy.AssessmentCorrect, Description: "explained it", Assisted: true, CreatedAt: base}},
		}},
	})

	// When the detail is requested
	detail, err := service.GetTopicDetail(context.Background(), "s1")

	// Then the quiz percent, the display name and the proof come through
	require.NoError(t, err)
	assert.Equal(t, "Cache", detail.Name)
	assert.Equal(t, "System Design", detail.FolderName)
	require.Len(t, detail.Quizzes, 1)
	assert.Equal(t, 75, *detail.Quizzes[0].Percent)
	assert.Equal(t, "BEGINNER", detail.Quizzes[0].Difficulty)
	require.Len(t, detail.Understood, 1)
	assert.Equal(t, "Cache-aside", detail.Understood[0].Name)
	assert.Equal(t, []domainprogress.DetailProof{{Assessment: "CORRECT", Description: "explained it", Assisted: true, At: base}}, detail.Understood[0].Proofs)
}

func TestGetTopicDetail_carriesWhatOpeningAChallengeNeeds(t *testing.T) {
	// Given a challenge with a difficulty and a time limit
	service := topicService(t, 80, TopicData{Challenges: []TopicChallengeData{{
		ID: "c1", Title: "Design a cache", Difficulty: "ADVANCED", TimeLimitMinutes: 60,
	}}})

	// When the detail is requested
	detail, err := service.GetTopicDetail(context.Background(), "s1")

	// Then both reach the result
	require.NoError(t, err)
	assert.Equal(t, "ADVANCED", detail.Challenges[0].Difficulty)
	assert.Equal(t, 60, detail.Challenges[0].TimeLimitMinutes)
}

func TestGetTopicDetail_aQuizAttemptWithoutQuestionsIsIgnored(t *testing.T) {
	// Given a quiz whose only attempt has no questions
	service := topicService(t, 80, TopicData{
		Quizzes: []TopicQuizData{{ID: "q1", Attempts: []QuizAttemptData{{CorrectCount: 0, Total: 0}}}},
	})

	// When the detail is requested
	detail, err := service.GetTopicDetail(context.Background(), "s1")

	// Then the quiz has no result
	require.NoError(t, err)
	assert.Nil(t, detail.Quizzes[0].Percent)
	assert.Zero(t, detail.Quizzes[0].Attempts)
}

func TestGetTopicDetail_aDefenseCriterionAtLevelOneIsADifficultyFromTheLab(t *testing.T) {
	// Given a challenge whose defense judged Consistency at level 1
	rv := twoCriteriaRubric()
	diagram := twoCriteriaEvaluation(domainexercise.EvaluationKindDiagram, 90, level("k1", 3, false), level("k2", 3, false))
	defense := twoCriteriaEvaluation(domainexercise.EvaluationKindUnderstanding, 60, level("k1", 3, false), level("k2", 1, false))
	service := topicService(t, 80, TopicData{Challenges: []TopicChallengeData{{
		ID: "c1", Title: "Design a cache", Attempts: []domainexercise.AttemptRecord{record(diagram, defense, rv, 1)},
	}}})

	// When the detail is requested
	detail, err := service.GetTopicDetail(context.Background(), "s1")

	// Then it is listed once, from "Design a cache", and the attempt failed
	require.NoError(t, err)
	assert.Equal(t, []domainprogress.Difficulty{
		{Name: "Consistency", Source: domainprogress.DifficultySourceArchLab, ChallengeTitle: "Design a cache"},
	}, detail.Difficulties)
	assert.Equal(t, domainprogress.LabResultFailed, detail.Challenges[0].Result)
	assert.Equal(t, 60, *detail.Challenges[0].DefenseScore)
	assert.Equal(t, 90, *detail.Challenges[0].DiagramScore)
}

func TestGetTopicDetail_aCriterionLowInBothEvaluationsIsListedOnce(t *testing.T) {
	// Given Scalability at level 0 in the diagram and level 1 in the defense
	rv := twoCriteriaRubric()
	diagram := twoCriteriaEvaluation(domainexercise.EvaluationKindDiagram, 50, level("k1", 0, false), level("k2", 4, false))
	defense := twoCriteriaEvaluation(domainexercise.EvaluationKindUnderstanding, 50, level("k1", 1, false), level("k2", 4, false))
	service := topicService(t, 80, TopicData{Challenges: []TopicChallengeData{{
		ID: "c1", Title: "Lab", Attempts: []domainexercise.AttemptRecord{record(diagram, defense, rv, 1)},
	}}})

	// When the detail is requested
	detail, err := service.GetTopicDetail(context.Background(), "s1")

	// Then Scalability appears a single time, in rubric order
	require.NoError(t, err)
	require.Len(t, detail.Difficulties, 1)
	assert.Equal(t, "Scalability", detail.Difficulties[0].Name)
}

func TestGetTopicDetail_criteriaAtTheMinimumLevelOrWithInsufficientEvidenceAreNotDifficulties(t *testing.T) {
	// Given Scalability at exactly the minimum level and Consistency flagged as insufficient evidence at level 0
	rv := twoCriteriaRubric()
	diagram := twoCriteriaEvaluation(domainexercise.EvaluationKindDiagram, 90, level("k1", domainexercise.MandatoryMinLevel, false), level("k2", 0, true))
	service := topicService(t, 80, TopicData{Challenges: []TopicChallengeData{{
		ID: "c1", Title: "Lab", Attempts: []domainexercise.AttemptRecord{record(diagram, nil, rv, 1)},
	}}})

	// When the detail is requested
	detail, err := service.GetTopicDetail(context.Background(), "s1")

	// Then there is no difficulty and the pending defense keeps the result pending
	require.NoError(t, err)
	assert.Empty(t, detail.Difficulties)
	assert.Equal(t, domainprogress.LabResultPending, detail.Challenges[0].Result)
	assert.Nil(t, detail.Challenges[0].DefenseScore)
}

func TestGetTopicDetail_anAttemptWithoutEvaluationsIsCountedButNotScored(t *testing.T) {
	// Given a challenge with one attempt that was never evaluated (no rubric)
	service := topicService(t, 80, TopicData{Challenges: []TopicChallengeData{{
		ID: "c1", Title: "Lab", Attempts: []domainexercise.AttemptRecord{record(nil, nil, nil, 1)},
	}}})

	// When the detail is requested
	detail, err := service.GetTopicDetail(context.Background(), "s1")

	// Then it counts as an attempt with no scores
	require.NoError(t, err)
	assert.Equal(t, 1, detail.Challenges[0].Attempts)
	assert.Nil(t, detail.Challenges[0].DiagramScore)
	assert.Equal(t, domainprogress.LabResultPending, detail.Challenges[0].Result)
}

func TestGetTopicDetail_anEvaluatedAttemptWithoutRubricIsAnError(t *testing.T) {
	// Given an evaluation whose rubric is missing
	diagram := twoCriteriaEvaluation(domainexercise.EvaluationKindDiagram, 90)
	service := topicService(t, 80, TopicData{Challenges: []TopicChallengeData{{
		ID: "c1", Attempts: []domainexercise.AttemptRecord{record(diagram, nil, nil, 1)},
	}}})

	// When the detail is requested
	_, err := service.GetTopicDetail(context.Background(), "s1")

	// Then it fails instead of guessing
	require.ErrorContains(t, err, "no rubric")
}

func TestGetTopicDetail_aPassedAttemptIsPassed(t *testing.T) {
	// Given both evaluations above the pass score, mandatory-free rubric
	rv := twoCriteriaRubric()
	diagram := twoCriteriaEvaluation(domainexercise.EvaluationKindDiagram, 85, level("k1", 3, false), level("k2", 3, false))
	defense := twoCriteriaEvaluation(domainexercise.EvaluationKindUnderstanding, 80, level("k1", 3, false), level("k2", 3, false))
	service := topicService(t, 80, TopicData{Challenges: []TopicChallengeData{{
		ID: "c1", Attempts: []domainexercise.AttemptRecord{record(diagram, defense, rv, 1)},
	}}})

	// When the detail is requested
	detail, err := service.GetTopicDetail(context.Background(), "s1")

	// Then the result is passed
	require.NoError(t, err)
	assert.Equal(t, domainprogress.LabResultPassed, detail.Challenges[0].Result)
}
