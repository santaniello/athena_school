package progress

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainprogress "github.com/santaniello/athena/internal/domain/progress"
	domainstudy "github.com/santaniello/athena/internal/domain/study"

	progressmocks "github.com/santaniello/athena/internal/application/progress/mocks"
)

var base = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

// fakeReader is a Reader that returns what it was given. A generated mock would
// import this package for FolderData, an import cycle (see .mockery.yaml).
type fakeReader struct {
	folders  []FolderData
	err      error
	topic    TopicData
	topicErr error
}

func (f fakeReader) ReadFolders(context.Context) ([]FolderData, error) { return f.folders, f.err }

func (f fakeReader) ReadTopic(context.Context, string) (TopicData, error) { return f.topic, f.topicErr }

func newService(t *testing.T, passScore int, folders ...FolderData) *Service {
	t.Helper()
	reader := fakeReader{folders: folders}
	passScores := progressmocks.NewMockPassScoreProvider(t)
	passScores.EXPECT().Current(context.Background()).Return(passScore, nil)
	return NewService(reader, passScores)
}

func oneSession(session SessionData) FolderData {
	session.ID, session.Topic, session.StartedAt = "s1", "Cache", base
	return FolderData{ID: "f1", Name: "System Design", Sessions: []SessionData{session}}
}

func onlyTopic(t *testing.T, dashboard domainprogress.Dashboard) domainprogress.TopicSummary {
	t.Helper()
	require.Len(t, dashboard.Folders, 1)
	require.Len(t, dashboard.Folders[0].Topics, 1)
	return dashboard.Folders[0].Topics[0]
}

func rubric(mandatoryCriterion bool) *domainexercise.RubricVersion {
	return &domainexercise.RubricVersion{
		ID:       "rv1",
		Criteria: []domainexercise.RubricCriterion{{ID: "k1", Weight: 100, Mandatory: mandatoryCriterion}},
	}
}

func evaluation(kind domainexercise.EvaluationKind, score, level int) *domainexercise.Evaluation {
	return &domainexercise.Evaluation{
		Kind:    kind,
		Score:   score,
		Results: []domainexercise.CriterionResult{{CriterionID: "k1", Level: level}},
	}
}

func record(diagram, understanding *domainexercise.Evaluation, rv *domainexercise.RubricVersion, day int) domainexercise.AttemptRecord {
	return domainexercise.AttemptRecord{
		Attempt:       domainexercise.ChallengeAttempt{ID: "a", StartedAt: base.AddDate(0, 0, day)},
		Diagram:       diagram,
		Understanding: understanding,
		Rubric:        rv,
	}
}

func TestGetDashboard_returnsTheErrorOfThePassScore(t *testing.T) {
	// Given a pass score that cannot be verified
	passScores := progressmocks.NewMockPassScoreProvider(t)
	passScores.EXPECT().Current(context.Background()).Return(0, errors.New("config unreadable"))

	// When the dashboard is requested
	_, err := NewService(fakeReader{}, passScores).GetDashboard(context.Background())

	// Then it fails instead of guessing a threshold
	require.ErrorContains(t, err, "config unreadable")
}

func TestGetDashboard_returnsTheErrorOfTheReader(t *testing.T) {
	// Given a reader that fails
	reader := fakeReader{err: errors.New("database locked")}
	passScores := progressmocks.NewMockPassScoreProvider(t)
	passScores.EXPECT().Current(context.Background()).Return(80, nil)

	// When the dashboard is requested
	_, err := NewService(reader, passScores).GetDashboard(context.Background())

	// Then the error is returned
	require.ErrorContains(t, err, "database locked")
}

func TestGetDashboard_returnsAnEmptyDashboardWithoutFolders(t *testing.T) {
	// Given no folders
	service := newService(t, 80)

	// When the dashboard is requested
	dashboard, err := service.GetDashboard(context.Background())

	// Then it is empty
	require.NoError(t, err)
	assert.Empty(t, dashboard.Folders)
	assert.Equal(t, domainprogress.Counters{}, dashboard.Counters)
}

func TestGetDashboard_namesTheTopicAfterTheSession(t *testing.T) {
	// Given a session with nothing attached
	service := newService(t, 80, oneSession(SessionData{}))

	// When the dashboard is requested
	dashboard, err := service.GetDashboard(context.Background())

	// Then the topic carries the session's id and topic, and has insufficient evidence
	require.NoError(t, err)
	topic := onlyTopic(t, dashboard)
	assert.Equal(t, "s1", topic.SessionID)
	assert.Equal(t, "Cache", topic.Name)
	assert.Equal(t, domainprogress.StatusInsufficientEvidence, topic.Status)
	assert.Equal(t, "f1", dashboard.Folders[0].ID)
}

func TestGetDashboard_scoresAQuizByItsLatestAttemptInPercent(t *testing.T) {
	// Given a quiz retaken from 6/10 to 9/10
	service := newService(t, 80, oneSession(SessionData{Quizzes: []QuizData{{ID: "q1", Attempts: []QuizAttemptData{
		{CorrectCount: 6, Total: 10, FinishedAt: base},
		{CorrectCount: 9, Total: 10, FinishedAt: base.AddDate(0, 0, 1)},
	}}}}))

	// When the dashboard is requested
	dashboard, err := service.GetDashboard(context.Background())

	// Then the score is the latest attempt, and the 30-point rise is improving
	require.NoError(t, err)
	topic := onlyTopic(t, dashboard)
	require.NotNil(t, topic.QuizScore)
	assert.Equal(t, 90, *topic.QuizScore)
	assert.Equal(t, domainprogress.StatusImproving, topic.Status)
}

func TestGetDashboard_roundsAQuizPercentHalfUp(t *testing.T) {
	// Given an attempt of 1 correct answer out of 8 (12.5%)
	service := newService(t, 80, oneSession(SessionData{Quizzes: []QuizData{{ID: "q1", Attempts: []QuizAttemptData{
		{CorrectCount: 1, Total: 8, FinishedAt: base},
	}}}}))

	// When the dashboard is requested
	dashboard, err := service.GetDashboard(context.Background())

	// Then the percent is 13
	require.NoError(t, err)
	assert.Equal(t, 13, *onlyTopic(t, dashboard).QuizScore)
}

func TestGetDashboard_roundsAQuizPercentDownBelowHalf(t *testing.T) {
	// Given an attempt of 1 correct answer out of 3 (33.3%)
	service := newService(t, 80, oneSession(SessionData{Quizzes: []QuizData{{ID: "q1", Attempts: []QuizAttemptData{
		{CorrectCount: 1, Total: 3, FinishedAt: base},
	}}}}))

	// When the dashboard is requested
	dashboard, err := service.GetDashboard(context.Background())

	// Then the percent is 33
	require.NoError(t, err)
	assert.Equal(t, 33, *onlyTopic(t, dashboard).QuizScore)
}

func TestGetDashboard_ignoresAQuizAttemptWithoutQuestions(t *testing.T) {
	// Given a quiz attempt that recorded no question
	service := newService(t, 80, oneSession(SessionData{Quizzes: []QuizData{{ID: "q1", Attempts: []QuizAttemptData{
		{CorrectCount: 0, Total: 0, FinishedAt: base},
	}}}}))

	// When the dashboard is requested
	dashboard, err := service.GetDashboard(context.Background())

	// Then it counts as no attempt
	require.NoError(t, err)
	topic := onlyTopic(t, dashboard)
	assert.Nil(t, topic.QuizScore)
	assert.Equal(t, domainprogress.StatusInsufficientEvidence, topic.Status)
}

func TestGetDashboard_scoresALabAndFailsItOnLowDefense(t *testing.T) {
	// Given a challenge whose latest attempt has diagram 85 and defense 40
	attempt := record(
		evaluation(domainexercise.EvaluationKindDiagram, 85, 4),
		evaluation(domainexercise.EvaluationKindUnderstanding, 40, 4),
		rubric(false), 1)
	service := newService(t, 80, oneSession(SessionData{Challenges: []ChallengeData{{ID: "c1", Attempts: []domainexercise.AttemptRecord{attempt}}}}))

	// When the dashboard is requested
	dashboard, err := service.GetDashboard(context.Background())

	// Then the lab is 63% and the topic needs reinforcement
	require.NoError(t, err)
	topic := onlyTopic(t, dashboard)
	require.NotNil(t, topic.LabScore)
	assert.Equal(t, 63, *topic.LabScore)
	assert.Equal(t, domainprogress.StatusNeedsReinforcement, topic.Status)
}

func TestGetDashboard_failsALabOnAMandatoryCriterionEvenWithPassingScores(t *testing.T) {
	// Given both scores above the minimum but a mandatory criterion judged at level 1
	attempt := record(
		evaluation(domainexercise.EvaluationKindDiagram, 90, 1),
		evaluation(domainexercise.EvaluationKindUnderstanding, 90, 4),
		rubric(true), 1)
	service := newService(t, 80, oneSession(SessionData{Challenges: []ChallengeData{{ID: "c1", Attempts: []domainexercise.AttemptRecord{attempt}}}}))

	// When the dashboard is requested
	dashboard, err := service.GetDashboard(context.Background())

	// Then the attempt is failed by the Arch Lab's rule
	require.NoError(t, err)
	assert.Equal(t, domainprogress.StatusNeedsReinforcement, onlyTopic(t, dashboard).Status)
}

func TestGetDashboard_passesALabThatMeetsEveryRule(t *testing.T) {
	// Given an attempt above the minimum on both evaluations
	attempt := record(
		evaluation(domainexercise.EvaluationKindDiagram, 90, 4),
		evaluation(domainexercise.EvaluationKindUnderstanding, 80, 4),
		rubric(true), 1)
	service := newService(t, 80, oneSession(SessionData{Challenges: []ChallengeData{{ID: "c1", Attempts: []domainexercise.AttemptRecord{attempt}}}}))

	// When the dashboard is requested
	dashboard, err := service.GetDashboard(context.Background())

	// Then the lab score is the mean and the topic is stable
	require.NoError(t, err)
	topic := onlyTopic(t, dashboard)
	assert.Equal(t, 85, *topic.LabScore)
	assert.Equal(t, domainprogress.StatusStable, topic.Status)
}

func TestGetDashboard_usesThePassScoreInForceNotTheOneFrozenOnTheAttempt(t *testing.T) {
	// Given an attempt of 70/70 frozen with a minimum of 60, and a minimum now of 80
	attempt := record(
		evaluation(domainexercise.EvaluationKindDiagram, 70, 4),
		evaluation(domainexercise.EvaluationKindUnderstanding, 70, 4),
		rubric(false), 1)
	attempt.Attempt.PassScore = 60
	service := newService(t, 80, oneSession(SessionData{Challenges: []ChallengeData{{ID: "c1", Attempts: []domainexercise.AttemptRecord{attempt}}}}))

	// When the dashboard is requested
	dashboard, err := service.GetDashboard(context.Background())

	// Then it is judged against 80
	require.NoError(t, err)
	assert.Equal(t, domainprogress.StatusNeedsReinforcement, onlyTopic(t, dashboard).Status)
}

func TestGetDashboard_leavesOutAnAttemptStoppedBeforeTheDefense(t *testing.T) {
	// Given a passed attempt followed by one that has only the diagram evaluation
	passed := record(
		evaluation(domainexercise.EvaluationKindDiagram, 90, 4),
		evaluation(domainexercise.EvaluationKindUnderstanding, 90, 4),
		rubric(false), 1)
	stopped := record(evaluation(domainexercise.EvaluationKindDiagram, 10, 0), nil, rubric(false), 2)
	service := newService(t, 80, oneSession(SessionData{Challenges: []ChallengeData{{ID: "c1", Attempts: []domainexercise.AttemptRecord{passed, stopped}}}}))

	// When the dashboard is requested
	dashboard, err := service.GetDashboard(context.Background())

	// Then the lab score and the status come from the passed attempt only
	require.NoError(t, err)
	topic := onlyTopic(t, dashboard)
	assert.Equal(t, 90, *topic.LabScore)
	assert.Equal(t, domainprogress.StatusStable, topic.Status)
}

func TestGetDashboard_leavesOutAnAttemptWithoutEvaluations(t *testing.T) {
	// Given a challenge whose only attempt has no evaluation
	service := newService(t, 80, oneSession(SessionData{Challenges: []ChallengeData{{ID: "c1", Attempts: []domainexercise.AttemptRecord{record(nil, nil, nil, 1)}}}}))

	// When the dashboard is requested
	dashboard, err := service.GetDashboard(context.Background())

	// Then there is no lab score and no evidence
	require.NoError(t, err)
	topic := onlyTopic(t, dashboard)
	assert.Nil(t, topic.LabScore)
	assert.Equal(t, domainprogress.StatusInsufficientEvidence, topic.Status)
}

func TestGetDashboard_refusesAnAttemptWithBothEvaluationsButNoRubric(t *testing.T) {
	// Given an inconsistent record
	attempt := record(
		evaluation(domainexercise.EvaluationKindDiagram, 90, 4),
		evaluation(domainexercise.EvaluationKindUnderstanding, 90, 4),
		nil, 1)
	service := newService(t, 80, oneSession(SessionData{Challenges: []ChallengeData{{ID: "c1", Attempts: []domainexercise.AttemptRecord{attempt}}}}))

	// When the dashboard is requested
	_, err := service.GetDashboard(context.Background())

	// Then it is refused rather than judged without the rubric
	require.ErrorContains(t, err, "no rubric")
}

func TestGetDashboard_datesALabAttemptBySubmissionOrElseByStart(t *testing.T) {
	submitted := record(
		evaluation(domainexercise.EvaluationKindDiagram, 90, 4),
		evaluation(domainexercise.EvaluationKindUnderstanding, 90, 4),
		rubric(false), 1)
	submitted.Attempt.SubmittedAt = base.AddDate(0, 0, 5)
	unsubmitted := record(
		evaluation(domainexercise.EvaluationKindDiagram, 90, 4),
		evaluation(domainexercise.EvaluationKindUnderstanding, 90, 4),
		rubric(false), 1)

	tests := []struct {
		name    string
		attempt domainexercise.AttemptRecord
		want    time.Time
	}{
		{"submission time when set", submitted, base.AddDate(0, 0, 5)},
		{"start time otherwise", unsubmitted, base.AddDate(0, 0, 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a session started before the attempt
			service := newService(t, 80, oneSession(SessionData{Challenges: []ChallengeData{{ID: "c1", Attempts: []domainexercise.AttemptRecord{tt.attempt}}}}))

			// When the dashboard is requested
			dashboard, err := service.GetDashboard(context.Background())

			// Then the topic's last activity is the attempt's date
			require.NoError(t, err)
			assert.True(t, tt.want.Equal(onlyTopic(t, dashboard).LastActivity), "got %s", onlyTopic(t, dashboard).LastActivity)
		})
	}
}

func TestGetDashboard_mapsConcepts(t *testing.T) {
	tests := []struct {
		name     string
		concepts []ConceptData
		want     domainprogress.Status
	}{
		{"a concept needing review with evidence", []ConceptData{{Status: domainstudy.StatusNeedsReview, EvidenceCount: 2}}, domainprogress.StatusNeedsReinforcement},
		{"a demonstrated concept with evidence", []ConceptData{{Status: domainstudy.StatusDemonstrated, EvidenceCount: 2}}, domainprogress.StatusStable},
		{"concepts only introduced by the tutor", []ConceptData{{Status: domainstudy.StatusIntroduced}}, domainprogress.StatusInsufficientEvidence},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a session with those concepts
			service := newService(t, 80, oneSession(SessionData{Concepts: tt.concepts}))

			// When the dashboard is requested
			dashboard, err := service.GetDashboard(context.Background())

			// Then the status follows them
			require.NoError(t, err)
			assert.Equal(t, tt.want, onlyTopic(t, dashboard).Status)
		})
	}
}

func TestGetDashboard_datesTheTopicByItsConceptUpdates(t *testing.T) {
	// Given a concept updated after the session started
	updated := base.AddDate(0, 0, 3)
	service := newService(t, 80, oneSession(SessionData{Concepts: []ConceptData{{Status: domainstudy.StatusIntroduced, UpdatedAt: updated}}}))

	// When the dashboard is requested
	dashboard, err := service.GetDashboard(context.Background())

	// Then the last activity is that update
	require.NoError(t, err)
	assert.True(t, updated.Equal(onlyTopic(t, dashboard).LastActivity))
}

func TestGetDashboard_returnsTheErrorOfAnInconsistentLabInAnyFolder(t *testing.T) {
	// Given a second folder whose session holds an inconsistent record
	bad := record(
		evaluation(domainexercise.EvaluationKindDiagram, 90, 4),
		evaluation(domainexercise.EvaluationKindUnderstanding, 90, 4),
		nil, 1)
	second := FolderData{ID: "f2", Name: "Other", Sessions: []SessionData{{
		ID: "s2", Topic: "Broken", StartedAt: base,
		Challenges: []ChallengeData{{ID: "c1", Attempts: []domainexercise.AttemptRecord{bad}}},
	}}}
	service := newService(t, 80, oneSession(SessionData{}), second)

	// When the dashboard is requested
	_, err := service.GetDashboard(context.Background())

	// Then the error surfaces
	require.Error(t, err)
}
