package progress

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	progressmocks "github.com/santaniello/athena/internal/application/progress/mocks"
	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainprogress "github.com/santaniello/athena/internal/domain/progress"
	insightmocks "github.com/santaniello/athena/internal/domain/progress/mocks"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

var insightNow = time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)

const validInsightAnswer = `{"analysis":[{"text":"Quiz is solid.","activities":["Q1"]}],"recommendations":[{"text":"Retake it.","activities":[]}]}`

type insightFixture struct {
	service    *Service
	repo       *insightmocks.MockInsightRepository
	generator  *insightmocks.MockInsightGenerator
	language   *progressmocks.MockLanguageProvider
	passScores *progressmocks.MockPassScoreProvider
}

func newInsightFixture(t *testing.T, topic TopicData, topicErr error) insightFixture {
	t.Helper()
	f := insightFixture{
		repo:       insightmocks.NewMockInsightRepository(t),
		generator:  insightmocks.NewMockInsightGenerator(t),
		language:   progressmocks.NewMockLanguageProvider(t),
		passScores: progressmocks.NewMockPassScoreProvider(t),
	}
	clock := progressmocks.NewMockClock(t)
	clock.EXPECT().Now().Return(insightNow).Maybe()
	f.passScores.EXPECT().Current(context.Background()).Return(80, nil).Maybe()
	f.service = NewService(fakeReader{topic: topic, topicErr: topicErr}, f.passScores, WithInsights(f.repo, f.generator, f.language, clock))
	return f
}

func quizTopic() TopicData {
	return TopicData{
		SessionID: "s1", Topic: "Cache", FolderName: "System Design",
		Quizzes: []TopicQuizData{{ID: "q1", Difficulty: "EASY", Attempts: []QuizAttemptData{{CorrectCount: 3, Total: 4, FinishedAt: base}}}},
	}
}

func quizInsightInput() domainprogress.InsightInput {
	return domainprogress.InsightInput{
		SessionID: "s1", Topic: "Cache",
		Quizzes: []domainprogress.InsightQuiz{{ID: "q1", Number: 1, Difficulty: "EASY", Attempts: []domainprogress.InsightQuizAttempt{{Percent: 75, At: base}}}},
	}
}

func TestGenerateTopicInsight_storesAndReturnsTheValidatedInsight(t *testing.T) {
	// Given a topic with a quiz attempt, a Portuguese assistant and a valid model answer
	f := newInsightFixture(t, quizTopic(), nil)
	ctx := context.Background()
	input := quizInsightInput()
	f.language.EXPECT().AssistantLanguage(ctx).Return("pt", nil)
	f.generator.EXPECT().GenerateInsight(ctx, input, "pt").Return(domainprogress.RawInsight{Content: validInsightAnswer, Model: "anthropic/x"}, nil)
	want := domainprogress.Insight{
		Analysis:        []domainprogress.Statement{{Text: "Quiz is solid.", Citations: []domainprogress.Citation{{Kind: domainprogress.CitationQuiz, ID: "q1", Label: "Quiz 1"}}}},
		Recommendations: []domainprogress.Statement{{Text: "Retake it.", Citations: []domainprogress.Citation{}}},
		Model:           "anthropic/x", Language: "pt", Fingerprint: domainprogress.Fingerprint(input, "pt"), CreatedAt: insightNow,
	}
	f.repo.EXPECT().Save(ctx, "s1", want).Return(nil)

	// When the insight is generated
	insight, err := f.service.GenerateTopicInsight(ctx, "s1")

	// Then it is stored and returned, stamped with the fingerprint of its input
	require.NoError(t, err)
	assert.Equal(t, want, insight)
}

func TestGenerateTopicInsight_refusesATopicWithoutEvidenceAndNeverCallsTheModel(t *testing.T) {
	// Given a topic with nothing to analyse
	f := newInsightFixture(t, TopicData{SessionID: "s1", Topic: "Cache"}, nil)

	// When the insight is requested
	_, err := f.service.GenerateTopicInsight(context.Background(), "s1")

	// Then it is refused (the mocks fail the test if the model or the store were used)
	require.ErrorIs(t, err, domainprogress.ErrInsufficientEvidence)
}

func TestGenerateTopicInsight_aMalformedAnswerStoresNothing(t *testing.T) {
	// Given a model that answers with prose
	f := newInsightFixture(t, quizTopic(), nil)
	ctx := context.Background()
	f.language.EXPECT().AssistantLanguage(ctx).Return("en", nil)
	f.generator.EXPECT().GenerateInsight(ctx, quizInsightInput(), "en").Return(domainprogress.RawInsight{Content: "Sure, here you go"}, nil)

	// When the insight is generated
	_, err := f.service.GenerateTopicInsight(ctx, "s1")

	// Then it fails as invalid and nothing is saved (the previous insight stays)
	require.ErrorIs(t, err, domainprogress.ErrInvalidInsight)
}

func TestGenerateTopicInsight_aModelErrorIsReturnedAndStoresNothing(t *testing.T) {
	// Given a failing model
	f := newInsightFixture(t, quizTopic(), nil)
	ctx := context.Background()
	boom := errors.New("provider down")
	f.language.EXPECT().AssistantLanguage(ctx).Return("en", nil)
	f.generator.EXPECT().GenerateInsight(ctx, quizInsightInput(), "en").Return(domainprogress.RawInsight{}, boom)

	// When the insight is generated
	_, err := f.service.GenerateTopicInsight(ctx, "s1")

	// Then the cause is returned
	require.ErrorIs(t, err, boom)
}

func TestGenerateTopicInsight_aStoreErrorIsReturned(t *testing.T) {
	// Given a store that fails
	f := newInsightFixture(t, quizTopic(), nil)
	ctx := context.Background()
	boom := errors.New("disk full")
	f.language.EXPECT().AssistantLanguage(ctx).Return("en", nil)
	f.generator.EXPECT().GenerateInsight(ctx, quizInsightInput(), "en").Return(domainprogress.RawInsight{Content: validInsightAnswer}, nil)
	f.repo.EXPECT().Save(ctx, "s1", domainprogress.Insight{
		Analysis:        []domainprogress.Statement{{Text: "Quiz is solid.", Citations: []domainprogress.Citation{{Kind: domainprogress.CitationQuiz, ID: "q1", Label: "Quiz 1"}}}},
		Recommendations: []domainprogress.Statement{{Text: "Retake it.", Citations: []domainprogress.Citation{}}},
		Language:        "en", Fingerprint: domainprogress.Fingerprint(quizInsightInput(), "en"), CreatedAt: insightNow,
	}).Return(boom)

	// When the insight is generated
	_, err := f.service.GenerateTopicInsight(ctx, "s1")

	// Then the cause is returned
	require.ErrorIs(t, err, boom)
}

func TestGenerateTopicInsight_returnsTheErrorsOfTheTopicTheLanguageAndThePassScore(t *testing.T) {
	ctx := context.Background()
	t.Run("topic not found", func(t *testing.T) {
		f := newInsightFixture(t, TopicData{}, ErrTopicNotFound)
		_, err := f.service.GenerateTopicInsight(ctx, "gone")
		require.ErrorIs(t, err, ErrTopicNotFound)
	})
	t.Run("language unreadable", func(t *testing.T) {
		f := newInsightFixture(t, quizTopic(), nil)
		f.language.EXPECT().AssistantLanguage(ctx).Return("", errors.New("no profile"))
		_, err := f.service.GenerateTopicInsight(ctx, "s1")
		require.ErrorContains(t, err, "no profile")
	})
	t.Run("pass score unreadable", func(t *testing.T) {
		passScores := progressmocks.NewMockPassScoreProvider(t)
		passScores.EXPECT().Current(ctx).Return(0, errors.New("config unreadable"))
		clock := progressmocks.NewMockClock(t)
		service := NewService(fakeReader{topic: quizTopic()}, passScores,
			WithInsights(insightmocks.NewMockInsightRepository(t), insightmocks.NewMockInsightGenerator(t), progressmocks.NewMockLanguageProvider(t), clock))
		_, err := service.GenerateTopicInsight(ctx, "s1")
		require.ErrorContains(t, err, "config unreadable")
	})
}

func TestGenerateTopicInsight_isUnavailableWithoutTheInsightPorts(t *testing.T) {
	// Given a service built without insights
	passScores := progressmocks.NewMockPassScoreProvider(t)

	// When an insight is requested
	_, err := NewService(fakeReader{topic: quizTopic()}, passScores).GenerateTopicInsight(context.Background(), "s1")

	// Then it is an error
	require.ErrorIs(t, err, ErrInsightsUnavailable)
}

func labTopic() TopicData {
	rv := &domainexercise.RubricVersion{ID: "rv1", Criteria: []domainexercise.RubricCriterion{
		{ID: "k1", Name: "Scalability", Weight: 50}, {ID: "k2", Name: "Consistency", Weight: 50},
	}}
	diagram := &domainexercise.Evaluation{
		Kind: domainexercise.EvaluationKindDiagram, Score: 90, CreatedAt: base.AddDate(0, 0, 3),
		Strengths: []string{"Clear"}, Gaps: []string{"No TTL"}, Recommendations: []string{"Add TTL"},
		Results: []domainexercise.CriterionResult{
			{CriterionID: "k1", Level: 3, Justification: "Sharded"},
			{CriterionID: "k2", InsufficientEvidence: true},
			{CriterionID: "unknown", Level: 4, Justification: "dropped"},
		},
	}
	defense := &domainexercise.Evaluation{
		Kind: domainexercise.EvaluationKindUnderstanding, Score: 70, CreatedAt: base.AddDate(0, 0, 4),
		Results: []domainexercise.CriterionResult{{CriterionID: "k2", Level: 2, Justification: "Vague"}},
	}
	return TopicData{
		SessionID: "s1", Topic: "Cache",
		Challenges: []TopicChallengeData{{
			ID: "c1", Title: "Design a cache",
			Attempts: []domainexercise.AttemptRecord{
				{Attempt: domainexercise.ChallengeAttempt{ID: "a0", StartedAt: base}},
				{Attempt: domainexercise.ChallengeAttempt{ID: "a1", StartedAt: base.AddDate(0, 0, 1), SubmittedAt: base.AddDate(0, 0, 2)}, Diagram: diagram, Understanding: defense, Rubric: rv},
			},
		}},
		Concepts: []TopicConceptData{
			{Name: "ttl", DisplayName: "TTL", Status: domainstudy.StatusNeedsReview, EvidenceCount: 1, UpdatedAt: base.AddDate(0, 0, 5),
				Proofs: []domainstudy.Evidence{{Assessment: domainstudy.AssessmentIncorrect, Description: "Vague on expiry", Assisted: true, CreatedAt: base.AddDate(0, 0, 6)}}},
			{Name: "lru", Status: domainstudy.StatusNotAssessed, UpdatedAt: base},
		},
	}
}

func TestGenerateTopicInsight_mapsTheEvidenceOfLabsAndConceptsForTheModel(t *testing.T) {
	// Given a topic with an unevaluated attempt, an evaluated one, a judged and a never assessed concept
	f := newInsightFixture(t, labTopic(), nil)
	ctx := context.Background()
	f.language.EXPECT().AssistantLanguage(ctx).Return("en", nil)
	want := domainprogress.InsightInput{
		SessionID: "s1", Topic: "Cache",
		Challenges: []domainprogress.InsightChallenge{{
			ID: "c1", Title: "Design a cache",
			Attempts: []domainprogress.InsightLabAttempt{{
				Result: domainprogress.LabResultFailed,
				// the latest of the submission and both evaluations
				At: base.AddDate(0, 0, 4),
				Diagram: &domainprogress.InsightEvaluation{
					Score: 90, Strengths: []string{"Clear"}, Gaps: []string{"No TTL"}, Recommendations: []string{"Add TTL"},
					Criteria: []domainprogress.InsightCriterion{
						{Name: "Scalability", Level: 3, Justification: "Sharded"},
						{Name: "Consistency", InsufficientEvidence: true},
					},
				},
				Defense: &domainprogress.InsightEvaluation{
					Score:    70,
					Criteria: []domainprogress.InsightCriterion{{Name: "Consistency", Level: 2, Justification: "Vague"}},
				},
			}},
		}},
		Concepts: []domainprogress.InsightConcept{{
			Name: "TTL", Status: domainprogress.StatusNeedsReview, UpdatedAt: base.AddDate(0, 0, 5),
			Proofs: []domainprogress.InsightProof{{Assessment: "INCORRECT", Description: "Vague on expiry", Assisted: true, At: base.AddDate(0, 0, 6)}},
		}},
	}
	f.generator.EXPECT().GenerateInsight(ctx, want, "en").Return(domainprogress.RawInsight{Content: validInsightAnswer}, nil)
	f.repo.EXPECT().Save(ctx, "s1", domainprogress.Insight{
		Analysis:        []domainprogress.Statement{{Text: "Quiz is solid.", Citations: []domainprogress.Citation{}}},
		Recommendations: []domainprogress.Statement{{Text: "Retake it.", Citations: []domainprogress.Citation{}}},
		Language:        "en", Fingerprint: domainprogress.Fingerprint(want, "en"), CreatedAt: insightNow,
	}).Return(nil)

	// When the insight is generated
	_, err := f.service.GenerateTopicInsight(ctx, "s1")

	// Then the model got exactly that evidence
	require.NoError(t, err)
}

func TestInsightInput_skipsAnEvaluatedAttemptWithoutItsRubric(t *testing.T) {
	// Given an attempt with an evaluation but no frozen rubric
	topic := labTopic()
	topic.Challenges[0].Attempts[1].Rubric = nil

	// When the evidence is mapped
	input, err := insightInput(topic, 80)

	// Then the attempt is not evidence, and it is not an error
	require.NoError(t, err)
	assert.Empty(t, input.Challenges[0].Attempts)
}

func storedInsight(input domainprogress.InsightInput, language string) domainprogress.Insight {
	return domainprogress.Insight{
		Analysis:        []domainprogress.Statement{{Text: "A.", Citations: []domainprogress.Citation{}}},
		Recommendations: []domainprogress.Statement{{Text: "B.", Citations: []domainprogress.Citation{}}},
		Model:           "m", Language: language, Fingerprint: domainprogress.Fingerprint(input, language), CreatedAt: insightNow,
	}
}

func TestGetTopicDetail_showsTheStoredInsightWithoutCallingTheModel(t *testing.T) {
	// Given a stored insight written from the current evidence and language
	f := newInsightFixture(t, quizTopic(), nil)
	ctx := context.Background()
	stored := storedInsight(quizInsightInput(), "en")
	f.repo.EXPECT().Get(ctx, "s1").Return(stored, nil)
	f.language.EXPECT().AssistantLanguage(ctx).Return("en", nil)

	// When the detail is read
	detail, err := f.service.GetTopicDetail(ctx, "s1")

	// Then the insight is there, fresh, and the generator was never used
	require.NoError(t, err)
	require.NotNil(t, detail.Insight.Current)
	assert.Equal(t, stored, *detail.Insight.Current)
	assert.False(t, detail.Insight.Stale)
	assert.True(t, detail.Insight.CanGenerate)
}

func TestGetTopicDetail_marksTheInsightStaleAfterNewActivity(t *testing.T) {
	// Given an insight written before the quiz was retaken
	topic := quizTopic()
	f := newInsightFixture(t, topic, nil)
	ctx := context.Background()
	stored := storedInsight(quizInsightInput(), "en")
	f.repo.EXPECT().Get(ctx, "s1").Return(stored, nil)
	f.language.EXPECT().AssistantLanguage(ctx).Return("en", nil)
	topic.Quizzes[0].Attempts = append(topic.Quizzes[0].Attempts, QuizAttemptData{CorrectCount: 4, Total: 4, FinishedAt: base.AddDate(0, 0, 2)})
	f.service.reader = fakeReader{topic: topic}

	// When the detail is read
	detail, err := f.service.GetTopicDetail(ctx, "s1")

	// Then the insight is stale
	require.NoError(t, err)
	assert.True(t, detail.Insight.Stale)
	assert.NotNil(t, detail.Insight.Current)
}

func TestGetTopicDetail_marksTheInsightStaleWhenTheLanguageChanges(t *testing.T) {
	// Given an English insight and a profile that now says Portuguese
	f := newInsightFixture(t, quizTopic(), nil)
	ctx := context.Background()
	f.repo.EXPECT().Get(ctx, "s1").Return(storedInsight(quizInsightInput(), "en"), nil)
	f.language.EXPECT().AssistantLanguage(ctx).Return("pt", nil)

	// When the detail is read
	detail, err := f.service.GetTopicDetail(ctx, "s1")

	// Then the insight is stale
	require.NoError(t, err)
	assert.True(t, detail.Insight.Stale)
}

func TestGetTopicDetail_hasNoInsightYetButMayGenerateOne(t *testing.T) {
	// Given a topic with evidence and no stored insight
	f := newInsightFixture(t, quizTopic(), nil)
	ctx := context.Background()
	f.repo.EXPECT().Get(ctx, "s1").Return(domainprogress.Insight{}, domainprogress.ErrInsightNotFound)

	// When the detail is read
	detail, err := f.service.GetTopicDetail(ctx, "s1")

	// Then there is no insight, it is not stale, and one can be generated
	require.NoError(t, err)
	assert.Nil(t, detail.Insight.Current)
	assert.False(t, detail.Insight.Stale)
	assert.True(t, detail.Insight.CanGenerate)
}

func TestGetTopicDetail_cannotGenerateAnInsightWithoutEvidence(t *testing.T) {
	// Given a topic with nothing in it
	f := newInsightFixture(t, TopicData{SessionID: "s1", Topic: "Cache"}, nil)
	ctx := context.Background()
	f.repo.EXPECT().Get(ctx, "s1").Return(domainprogress.Insight{}, domainprogress.ErrInsightNotFound)

	// When the detail is read
	detail, err := f.service.GetTopicDetail(ctx, "s1")

	// Then generating is not offered
	require.NoError(t, err)
	assert.False(t, detail.Insight.CanGenerate)
}

func TestGetTopicDetail_anInsightFailureNeverBlocksTheRestOfTheDetail(t *testing.T) {
	ctx := context.Background()
	t.Run("store", func(t *testing.T) {
		f := newInsightFixture(t, quizTopic(), nil)
		f.repo.EXPECT().Get(ctx, "s1").Return(domainprogress.Insight{}, errors.New("db locked"))

		detail, err := f.service.GetTopicDetail(ctx, "s1")

		// Then the detail is there, with no insight and a new one can be asked for
		require.NoError(t, err)
		assert.Len(t, detail.Quizzes, 1)
		assert.Nil(t, detail.Insight.Current)
		assert.True(t, detail.Insight.CanGenerate)
	})
	t.Run("language", func(t *testing.T) {
		f := newInsightFixture(t, quizTopic(), nil)
		f.repo.EXPECT().Get(ctx, "s1").Return(storedInsight(quizInsightInput(), "en"), nil)
		f.language.EXPECT().AssistantLanguage(ctx).Return("", errors.New("no profile"))

		detail, err := f.service.GetTopicDetail(ctx, "s1")

		require.NoError(t, err)
		assert.Len(t, detail.Quizzes, 1)
		assert.Nil(t, detail.Insight.Current)
	})
}
