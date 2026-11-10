package exercise

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainmocks "github.com/santaniello/athena/internal/domain/exercise/mocks"
)

type feedbackFixture struct {
	service     *FeedbackService
	links       *domainmocks.MockSessionExerciseRepository
	challenges  *domainmocks.MockChallengeRepository
	rubrics     *domainmocks.MockRubricRepository
	attempts    *domainmocks.MockAttemptRepository
	evaluations *domainmocks.MockEvaluationRepository
}

func newFeedbackFixture(t *testing.T) *feedbackFixture {
	t.Helper()
	f := &feedbackFixture{
		links:       domainmocks.NewMockSessionExerciseRepository(t),
		challenges:  domainmocks.NewMockChallengeRepository(t),
		rubrics:     domainmocks.NewMockRubricRepository(t),
		attempts:    domainmocks.NewMockAttemptRepository(t),
		evaluations: domainmocks.NewMockEvaluationRepository(t),
	}
	f.service = NewFeedbackService(f.links, f.challenges, f.rubrics, f.attempts, f.evaluations)
	return f
}

func feedbackRubric() domainexercise.RubricVersion {
	return domainexercise.RubricVersion{
		ID: "rubric-1", ChallengeID: "challenge-1",
		Criteria: []domainexercise.RubricCriterion{
			{ID: "c1", Name: "Scalability", Weight: 50, Mandatory: true},
			{ID: "c2", Name: "Caching", Weight: 50},
		},
	}
}

func feedbackAttempt(id, linkID string, state domainexercise.AttemptState, submittedDay int) domainexercise.ChallengeAttempt {
	return domainexercise.ChallengeAttempt{
		ID: id, ChallengeID: "challenge-1", RubricVersionID: "rubric-1", SessionExerciseID: linkID,
		State: state, PassScore: 80, SubmittedAt: time.Date(2026, 11, submittedDay, 10, 0, 0, 0, time.UTC),
	}
}

func feedbackEvaluation(kind domainexercise.EvaluationKind, score, mandatoryLevel int, gaps, recommendations []string) domainexercise.Evaluation {
	return domainexercise.Evaluation{
		Kind: kind, Score: score, Gaps: gaps, Recommendations: recommendations,
		Results: []domainexercise.CriterionResult{
			{CriterionID: "c1", Level: mandatoryLevel, Justification: "secret justification"},
			{CriterionID: "c2", Level: 4},
		},
	}
}

func (f *feedbackFixture) expectLinks(links ...domainexercise.SessionExercise) {
	f.links.EXPECT().ListBySession(context.Background(), "session-1").Return(links, nil).Once()
}

func (f *feedbackFixture) expectFinished(attemptID string, diagram, understanding domainexercise.Evaluation) {
	f.rubrics.EXPECT().Get(context.Background(), "rubric-1").Return(feedbackRubric(), nil).Once()
	f.evaluations.EXPECT().GetByAttempt(context.Background(), attemptID, domainexercise.EvaluationKindDiagram).Return(diagram, nil).Once()
	f.evaluations.EXPECT().GetByAttempt(context.Background(), attemptID, domainexercise.EvaluationKindUnderstanding).Return(understanding, nil).Once()
	f.challenges.EXPECT().Get(context.Background(), "challenge-1").Return(domainexercise.Challenge{
		ID: "challenge-1", Title: "Design a feed", Topic: "caching", Difficulty: domainexercise.DifficultyIntermediate,
	}, nil).Once()
}

func TestSessionExerciseFeedback_fillsTheSummaryOfAFailedAttempt(t *testing.T) {
	// Given a session with one finished attempt that failed a mandatory criterion
	f := newFeedbackFixture(t)
	f.expectLinks(domainexercise.SessionExercise{ID: "link-1", ChallengeID: "challenge-1"})
	f.attempts.EXPECT().ListByChallenge(context.Background(), "challenge-1").
		Return([]domainexercise.ChallengeAttempt{feedbackAttempt("a1", "link-1", domainexercise.AttemptFinished, 5)}, nil).Once()
	f.expectFinished("a1",
		feedbackEvaluation(domainexercise.EvaluationKindDiagram, 90, 1, []string{"no sharding", "  "}, []string{"add a cache"}),
		feedbackEvaluation(domainexercise.EvaluationKindUnderstanding, 85, 1, []string{"weak on failover"}, []string{"review replication", "review quorum", "review CAP", "review PACELC"}))

	// When reading the session's feedback
	feedback, err := f.service.SessionExerciseFeedback(context.Background(), "session-1")

	// Then it carries the facts, the unmet mandatory criterion once, and at most three of each list in order
	require.NoError(t, err)
	assert.Equal(t, []SessionFeedback{{
		ChallengeTitle: "Design a feed", Topic: "caching", Difficulty: domainexercise.DifficultyIntermediate,
		Result: domainexercise.ResultFailed, DiagramScore: 90, UnderstandingScore: 85, PassScore: 80,
		UnmetMandatory:  []string{"Scalability"},
		Gaps:            []string{"no sharding", "weak on failover"},
		Recommendations: []string{"add a cache", "review replication", "review quorum"},
	}}, feedback)
}

func TestSessionExerciseFeedback_reportsAPassedAttemptWithoutUnmetCriteria(t *testing.T) {
	// Given a finished attempt that passed
	f := newFeedbackFixture(t)
	f.expectLinks(domainexercise.SessionExercise{ID: "link-1", ChallengeID: "challenge-1"})
	f.attempts.EXPECT().ListByChallenge(context.Background(), "challenge-1").
		Return([]domainexercise.ChallengeAttempt{feedbackAttempt("a1", "link-1", domainexercise.AttemptFinished, 5)}, nil).Once()
	f.expectFinished("a1",
		feedbackEvaluation(domainexercise.EvaluationKindDiagram, 95, 4, nil, nil),
		feedbackEvaluation(domainexercise.EvaluationKindUnderstanding, 90, 4, nil, nil))

	// When reading the feedback
	feedback, err := f.service.SessionExerciseFeedback(context.Background(), "session-1")

	// Then it is Passed with nothing unmet
	require.NoError(t, err)
	require.Len(t, feedback, 1)
	assert.Equal(t, domainexercise.ResultPassed, feedback[0].Result)
	assert.Empty(t, feedback[0].UnmetMandatory)
}

func TestSessionExerciseFeedback_ignoresStandaloneAndUnfinishedAttempts(t *testing.T) {
	// Given attempts of the linked challenge: one standalone, one of another link, one not finished
	f := newFeedbackFixture(t)
	f.expectLinks(domainexercise.SessionExercise{ID: "link-1", ChallengeID: "challenge-1"})
	f.attempts.EXPECT().ListByChallenge(context.Background(), "challenge-1").Return([]domainexercise.ChallengeAttempt{
		feedbackAttempt("standalone", "", domainexercise.AttemptFinished, 1),
		feedbackAttempt("other", "link-9", domainexercise.AttemptFinished, 2),
		feedbackAttempt("running", "link-1", domainexercise.AttemptInProgress, 3),
		feedbackAttempt("evaluated", "link-1", domainexercise.AttemptEvaluated, 4),
	}, nil).Once()

	// When reading the feedback
	feedback, err := f.service.SessionExerciseFeedback(context.Background(), "session-1")

	// Then there is nothing to report
	require.NoError(t, err)
	assert.Empty(t, feedback)
}

func TestSessionExerciseFeedback_noLinksGivesNothing(t *testing.T) {
	// Given a session with no exercise
	f := newFeedbackFixture(t)
	f.expectLinks()

	// When reading the feedback
	feedback, err := f.service.SessionExerciseFeedback(context.Background(), "session-1")

	// Then it is empty
	require.NoError(t, err)
	assert.Empty(t, feedback)
}

func TestSessionExerciseFeedback_keepsTheThreeMostRecentNewestFirst(t *testing.T) {
	// Given four finished attempts of the session, listed oldest first
	f := newFeedbackFixture(t)
	f.expectLinks(domainexercise.SessionExercise{ID: "link-1", ChallengeID: "challenge-1"})
	f.attempts.EXPECT().ListByChallenge(context.Background(), "challenge-1").Return([]domainexercise.ChallengeAttempt{
		feedbackAttempt("a1", "link-1", domainexercise.AttemptFinished, 1),
		feedbackAttempt("a2", "link-1", domainexercise.AttemptFinished, 2),
		feedbackAttempt("a3", "link-1", domainexercise.AttemptFinished, 3),
		feedbackAttempt("a4", "link-1", domainexercise.AttemptFinished, 4),
	}, nil).Once()
	for index, id := range []string{"a4", "a3", "a2"} {
		f.expectFinished(id,
			feedbackEvaluation(domainexercise.EvaluationKindDiagram, 90+index, 4, nil, nil),
			feedbackEvaluation(domainexercise.EvaluationKindUnderstanding, 90, 4, nil, nil))
	}

	// When reading the feedback
	feedback, err := f.service.SessionExerciseFeedback(context.Background(), "session-1")

	// Then only the three newest are loaded, newest first
	require.NoError(t, err)
	require.Len(t, feedback, 3)
	assert.Equal(t, []int{90, 91, 92}, []int{feedback[0].DiagramScore, feedback[1].DiagramScore, feedback[2].DiagramScore})
}

func TestSessionExerciseFeedback_wrapsRepositoryErrors(t *testing.T) {
	boom := errors.New("boom")
	finishedAttempts := []domainexercise.ChallengeAttempt{feedbackAttempt("a1", "link-1", domainexercise.AttemptFinished, 5)}
	link := domainexercise.SessionExercise{ID: "link-1", ChallengeID: "challenge-1"}
	ctx := context.Background()
	tests := []struct {
		name  string
		setup func(f *feedbackFixture)
	}{
		{"links", func(f *feedbackFixture) {
			f.links.EXPECT().ListBySession(ctx, "session-1").Return(nil, boom).Once()
		}},
		{"attempts", func(f *feedbackFixture) {
			f.expectLinks(link)
			f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return(nil, boom).Once()
		}},
		{"challenge", func(f *feedbackFixture) {
			f.expectLinks(link)
			f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return(finishedAttempts, nil).Once()
			f.challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{}, boom).Once()
		}},
		{"rubric", func(f *feedbackFixture) {
			f.expectLinks(link)
			f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return(finishedAttempts, nil).Once()
			f.challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{}, nil).Once()
			f.rubrics.EXPECT().Get(ctx, "rubric-1").Return(domainexercise.RubricVersion{}, boom).Once()
		}},
		{"diagram evaluation", func(f *feedbackFixture) {
			f.expectLinks(link)
			f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return(finishedAttempts, nil).Once()
			f.challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{}, nil).Once()
			f.rubrics.EXPECT().Get(ctx, "rubric-1").Return(domainexercise.RubricVersion{}, nil).Once()
			f.evaluations.EXPECT().GetByAttempt(ctx, "a1", domainexercise.EvaluationKindDiagram).Return(domainexercise.Evaluation{}, boom).Once()
		}},
		{"understanding evaluation", func(f *feedbackFixture) {
			f.expectLinks(link)
			f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return(finishedAttempts, nil).Once()
			f.challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{}, nil).Once()
			f.rubrics.EXPECT().Get(ctx, "rubric-1").Return(domainexercise.RubricVersion{}, nil).Once()
			f.evaluations.EXPECT().GetByAttempt(ctx, "a1", domainexercise.EvaluationKindDiagram).Return(domainexercise.Evaluation{}, nil).Once()
			f.evaluations.EXPECT().GetByAttempt(ctx, "a1", domainexercise.EvaluationKindUnderstanding).Return(domainexercise.Evaluation{}, boom).Once()
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFeedbackFixture(t)
			tt.setup(f)

			feedback, err := f.service.SessionExerciseFeedback(ctx, "session-1")

			require.ErrorIs(t, err, boom)
			assert.Nil(t, feedback)
		})
	}
}
