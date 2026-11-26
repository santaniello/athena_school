package progress

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	progressmocks "github.com/santaniello/athena/internal/application/progress/mocks"
	domainprogress "github.com/santaniello/athena/internal/domain/progress"
	insightmocks "github.com/santaniello/athena/internal/domain/progress/mocks"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

const validFolderInsightAnswer = `{"analysis":[{"text":"Cache needs work.","activities":["T1"]}],"recommendations":[{"text":"Review queues.","activities":["T2"]}]}`

type folderInsightFixture struct {
	service   *Service
	repo      *insightmocks.MockFolderInsightRepository
	generator *insightmocks.MockFolderInsightGenerator
	language  *progressmocks.MockLanguageProvider
}

func newFolderInsightFixture(t *testing.T, folder FolderTopicsData, folderErr error) folderInsightFixture {
	t.Helper()
	f := folderInsightFixture{
		repo:      insightmocks.NewMockFolderInsightRepository(t),
		generator: insightmocks.NewMockFolderInsightGenerator(t),
		language:  progressmocks.NewMockLanguageProvider(t),
	}
	clock := progressmocks.NewMockClock(t)
	clock.EXPECT().Now().Return(insightNow).Maybe()
	passScores := progressmocks.NewMockPassScoreProvider(t)
	passScores.EXPECT().Current(context.Background()).Return(80, nil).Maybe()
	f.service = NewService(fakeReader{folder: folder, folderErr: folderErr}, passScores,
		WithInsights(insightmocks.NewMockInsightRepository(t), insightmocks.NewMockInsightGenerator(t), f.language, clock),
		WithFolderInsights(f.repo, f.generator))
	return f
}

// queuesTopic is a topic whose only evidence is a concept to review.
func queuesTopic() TopicData {
	return TopicData{
		SessionID: "s2", Topic: "Queues", FolderName: "System Design",
		Concepts: []TopicConceptData{{Name: "ordering", DisplayName: "Ordering", Status: domainstudy.StatusNeedsReview, EvidenceCount: 1, UpdatedAt: base}},
	}
}

func twoTopicFolder() FolderTopicsData {
	return FolderTopicsData{ID: "f1", Name: "System Design", Topics: []TopicData{quizTopic(), queuesTopic()}}
}

func twoTopicFolderInput() domainprogress.FolderInsightInput {
	quizScore := 75
	queues := domainprogress.InsightInput{
		SessionID: "s2", Topic: "Queues",
		Concepts: []domainprogress.InsightConcept{{Name: "Ordering", Status: domainprogress.StatusNeedsReview, UpdatedAt: base}},
	}
	return domainprogress.FolderInsightInput{
		FolderID: "f1", FolderName: "System Design",
		Topics: []domainprogress.FolderInsightTopic{
			{SessionID: "s1", Name: "Cache", QuizScore: &quizScore, Status: domainprogress.StatusNeedsReinforcement,
				Fingerprint: domainprogress.Fingerprint(quizInsightInput(), "")},
			{SessionID: "s2", Name: "Queues", Status: domainprogress.StatusNeedsReinforcement,
				Fingerprint:  domainprogress.Fingerprint(queues, ""),
				Difficulties: []domainprogress.FolderInsightDifficulty{{Name: "Ordering", ToReview: true}}},
		},
	}
}

func TestGenerateFolderInsight_storesAndReturnsTheValidatedInsight(t *testing.T) {
	// Given a folder with two topics, a Portuguese assistant and a valid model answer
	f := newFolderInsightFixture(t, twoTopicFolder(), nil)
	ctx := context.Background()
	input := twoTopicFolderInput()
	f.language.EXPECT().AssistantLanguage(ctx).Return("pt", nil)
	f.generator.EXPECT().GenerateFolderInsight(ctx, input, "pt").Return(domainprogress.RawInsight{Content: validFolderInsightAnswer, Model: "anthropic/x"}, nil)
	want := domainprogress.Insight{
		Analysis:        []domainprogress.Statement{{Text: "Cache needs work.", Citations: []domainprogress.Citation{{Kind: domainprogress.CitationTopic, ID: "s1", Label: "Cache"}}}},
		Recommendations: []domainprogress.Statement{{Text: "Review queues.", Citations: []domainprogress.Citation{{Kind: domainprogress.CitationTopic, ID: "s2", Label: "Queues"}}}},
		Model:           "anthropic/x", Language: "pt", Fingerprint: domainprogress.FolderFingerprint(input, "pt"), CreatedAt: insightNow,
	}
	f.repo.EXPECT().Save(ctx, "f1", want).Return(nil)

	// When the insight is generated
	insight, err := f.service.GenerateFolderInsight(ctx, "f1")

	// Then the model got the summaries and difficulties, and the insight is stored and returned
	require.NoError(t, err)
	assert.Equal(t, want, insight)
}

func TestGenerateFolderInsight_refusesAFolderWithoutEvidenceAndNeverCallsTheModel(t *testing.T) {
	// Given a folder whose only topic has no evidence
	folder := FolderTopicsData{ID: "f1", Name: "System Design", Topics: []TopicData{{SessionID: "s1", Topic: "Cache"}}}
	f := newFolderInsightFixture(t, folder, nil)

	// When the insight is requested
	_, err := f.service.GenerateFolderInsight(context.Background(), "f1")

	// Then it is refused (the mocks fail the test if the model or the store were used)
	require.ErrorIs(t, err, domainprogress.ErrInsufficientEvidence)
}

func TestGenerateFolderInsight_refusesAnEmptyFolder(t *testing.T) {
	f := newFolderInsightFixture(t, FolderTopicsData{ID: "f1", Name: "Empty"}, nil)

	_, err := f.service.GenerateFolderInsight(context.Background(), "f1")

	require.ErrorIs(t, err, domainprogress.ErrInsufficientEvidence)
}

func TestGenerateFolderInsight_aMalformedAnswerStoresNothing(t *testing.T) {
	// Given a model that answers with prose
	f := newFolderInsightFixture(t, twoTopicFolder(), nil)
	ctx := context.Background()
	f.language.EXPECT().AssistantLanguage(ctx).Return("en", nil)
	f.generator.EXPECT().GenerateFolderInsight(ctx, twoTopicFolderInput(), "en").Return(domainprogress.RawInsight{Content: "Sure"}, nil)

	// When the insight is generated
	_, err := f.service.GenerateFolderInsight(ctx, "f1")

	// Then it fails as invalid and nothing is saved
	require.ErrorIs(t, err, domainprogress.ErrInvalidInsight)
}

func TestGenerateFolderInsight_aModelErrorIsReturnedAndStoresNothing(t *testing.T) {
	f := newFolderInsightFixture(t, twoTopicFolder(), nil)
	ctx := context.Background()
	boom := errors.New("provider down")
	f.language.EXPECT().AssistantLanguage(ctx).Return("en", nil)
	f.generator.EXPECT().GenerateFolderInsight(ctx, twoTopicFolderInput(), "en").Return(domainprogress.RawInsight{}, boom)

	_, err := f.service.GenerateFolderInsight(ctx, "f1")

	require.ErrorIs(t, err, boom)
}

func TestGenerateFolderInsight_aStoreErrorIsReturned(t *testing.T) {
	f := newFolderInsightFixture(t, twoTopicFolder(), nil)
	ctx := context.Background()
	boom := errors.New("disk full")
	f.language.EXPECT().AssistantLanguage(ctx).Return("en", nil)
	f.generator.EXPECT().GenerateFolderInsight(ctx, twoTopicFolderInput(), "en").Return(domainprogress.RawInsight{Content: validFolderInsightAnswer}, nil)
	f.repo.EXPECT().Save(ctx, "f1", domainprogress.Insight{
		Analysis:        []domainprogress.Statement{{Text: "Cache needs work.", Citations: []domainprogress.Citation{{Kind: domainprogress.CitationTopic, ID: "s1", Label: "Cache"}}}},
		Recommendations: []domainprogress.Statement{{Text: "Review queues.", Citations: []domainprogress.Citation{{Kind: domainprogress.CitationTopic, ID: "s2", Label: "Queues"}}}},
		Language:        "en", Fingerprint: domainprogress.FolderFingerprint(twoTopicFolderInput(), "en"), CreatedAt: insightNow,
	}).Return(boom)

	_, err := f.service.GenerateFolderInsight(ctx, "f1")

	require.ErrorIs(t, err, boom)
}

func TestGenerateFolderInsight_returnsTheErrorsOfTheFolderAndTheLanguage(t *testing.T) {
	// Given a reader that cannot find the folder
	f := newFolderInsightFixture(t, FolderTopicsData{}, ErrFolderNotFound)
	_, err := f.service.GenerateFolderInsight(context.Background(), "f1")
	require.ErrorIs(t, err, ErrFolderNotFound)

	// And a language that cannot be read
	f = newFolderInsightFixture(t, twoTopicFolder(), nil)
	boom := errors.New("profile unreadable")
	f.language.EXPECT().AssistantLanguage(context.Background()).Return("", boom)
	_, err = f.service.GenerateFolderInsight(context.Background(), "f1")
	require.ErrorIs(t, err, boom)
}

func TestGenerateFolderInsight_isUnavailableWithoutThePorts(t *testing.T) {
	passScores := progressmocks.NewMockPassScoreProvider(t)

	// Given a service built without the folder insight ports
	_, err := NewService(fakeReader{}, passScores).GenerateFolderInsight(context.Background(), "f1")
	require.ErrorIs(t, err, ErrInsightsUnavailable)

	// And one with them but without the topic insight ports they depend on
	_, err = NewService(fakeReader{}, passScores, WithFolderInsights(
		insightmocks.NewMockFolderInsightRepository(t), insightmocks.NewMockFolderInsightGenerator(t),
	)).GenerateFolderInsight(context.Background(), "f1")
	require.ErrorIs(t, err, ErrInsightsUnavailable)
}

func storedFolderInsight(input domainprogress.FolderInsightInput, language string) domainprogress.Insight {
	return domainprogress.Insight{
		Analysis:        []domainprogress.Statement{{Text: "A.", Citations: []domainprogress.Citation{}}},
		Recommendations: []domainprogress.Statement{{Text: "B.", Citations: []domainprogress.Citation{}}},
		Model:           "m", Language: language, Fingerprint: domainprogress.FolderFingerprint(input, language), CreatedAt: insightNow,
	}
}

func TestGetFolderInsight_showsTheStoredInsightWithoutCallingTheModel(t *testing.T) {
	// Given a stored insight written from the current topics and language
	f := newFolderInsightFixture(t, twoTopicFolder(), nil)
	ctx := context.Background()
	stored := storedFolderInsight(twoTopicFolderInput(), "en")
	f.repo.EXPECT().Get(ctx, "f1").Return(stored, nil)
	f.language.EXPECT().AssistantLanguage(ctx).Return("en", nil)

	// When the folder insight is read
	view, err := f.service.GetFolderInsight(ctx, "f1")

	// Then it is there, fresh, and the generator was never used
	require.NoError(t, err)
	require.NotNil(t, view.Current)
	assert.Equal(t, stored, *view.Current)
	assert.False(t, view.Stale)
	assert.True(t, view.CanGenerate)
}

func TestGetFolderInsight_isStaleWhenASessionMovesOutOfTheFolder(t *testing.T) {
	// Given an insight written for two topics, and the folder now holding only one
	folder := twoTopicFolder()
	folder.Topics = folder.Topics[:1]
	f := newFolderInsightFixture(t, folder, nil)
	ctx := context.Background()
	f.repo.EXPECT().Get(ctx, "f1").Return(storedFolderInsight(twoTopicFolderInput(), "en"), nil)
	f.language.EXPECT().AssistantLanguage(ctx).Return("en", nil)

	// When the folder insight is read
	view, err := f.service.GetFolderInsight(ctx, "f1")

	// Then it is stale
	require.NoError(t, err)
	assert.True(t, view.Stale)
}

func TestGetFolderInsight_isStaleWhenASessionMovesIntoTheFolder(t *testing.T) {
	// Given an insight written for one topic, and a session moved in
	oneTopic := twoTopicFolderInput()
	oneTopic.Topics = oneTopic.Topics[:1]
	f := newFolderInsightFixture(t, twoTopicFolder(), nil)
	ctx := context.Background()
	f.repo.EXPECT().Get(ctx, "f1").Return(storedFolderInsight(oneTopic, "en"), nil)
	f.language.EXPECT().AssistantLanguage(ctx).Return("en", nil)

	// When the folder insight is read
	view, err := f.service.GetFolderInsight(ctx, "f1")

	// Then it is stale
	require.NoError(t, err)
	assert.True(t, view.Stale)
}

func TestGetFolderInsight_isStaleAfterNewActivityInATopic(t *testing.T) {
	// Given an insight written before a quiz of one topic was retaken
	folder := twoTopicFolder()
	folder.Topics[0].Quizzes[0].Attempts = append(folder.Topics[0].Quizzes[0].Attempts,
		QuizAttemptData{CorrectCount: 4, Total: 4, FinishedAt: base.AddDate(0, 0, 1)})
	f := newFolderInsightFixture(t, folder, nil)
	ctx := context.Background()
	f.repo.EXPECT().Get(ctx, "f1").Return(storedFolderInsight(twoTopicFolderInput(), "en"), nil)
	f.language.EXPECT().AssistantLanguage(ctx).Return("en", nil)

	// When the folder insight is read
	view, err := f.service.GetFolderInsight(ctx, "f1")

	// Then it is stale
	require.NoError(t, err)
	assert.True(t, view.Stale)
}

func TestGetFolderInsight_isStaleWhenTheLanguageChanges(t *testing.T) {
	f := newFolderInsightFixture(t, twoTopicFolder(), nil)
	ctx := context.Background()
	f.repo.EXPECT().Get(ctx, "f1").Return(storedFolderInsight(twoTopicFolderInput(), "en"), nil)
	f.language.EXPECT().AssistantLanguage(ctx).Return("pt", nil)

	view, err := f.service.GetFolderInsight(ctx, "f1")

	require.NoError(t, err)
	assert.True(t, view.Stale)
}

func TestGetFolderInsight_hasNoInsightYetButMayGenerateOne(t *testing.T) {
	f := newFolderInsightFixture(t, twoTopicFolder(), nil)
	ctx := context.Background()
	f.repo.EXPECT().Get(ctx, "f1").Return(domainprogress.Insight{}, domainprogress.ErrInsightNotFound)

	view, err := f.service.GetFolderInsight(ctx, "f1")

	require.NoError(t, err)
	assert.Nil(t, view.Current)
	assert.True(t, view.CanGenerate)
}

func TestGetFolderInsight_cannotGenerateWithoutEvidence(t *testing.T) {
	// Given a folder with no evidence in any topic
	folder := FolderTopicsData{ID: "f1", Name: "System Design", Topics: []TopicData{{SessionID: "s1", Topic: "Cache"}}}
	f := newFolderInsightFixture(t, folder, nil)
	f.repo.EXPECT().Get(context.Background(), "f1").Return(domainprogress.Insight{}, domainprogress.ErrInsightNotFound)

	// When the folder insight is read
	view, err := f.service.GetFolderInsight(context.Background(), "f1")

	// Then there is no button
	require.NoError(t, err)
	assert.False(t, view.CanGenerate)
}

func TestGetFolderInsight_isStaleWhenANewPassScoreMovesAStatus(t *testing.T) {
	// Given an insight written when the quiz of the first topic was still solid
	written := twoTopicFolderInput()
	written.Topics[0].Status = domainprogress.StatusStable
	f := newFolderInsightFixture(t, twoTopicFolder(), nil)
	ctx := context.Background()
	f.repo.EXPECT().Get(ctx, "f1").Return(storedFolderInsight(written, "en"), nil)
	f.language.EXPECT().AssistantLanguage(ctx).Return("en", nil)

	// When the folder insight is read with the pass score now in force
	view, err := f.service.GetFolderInsight(ctx, "f1")

	// Then it is stale
	require.NoError(t, err)
	assert.True(t, view.Stale)
}

func TestGetFolderInsight_aStoreErrorDegradesToACardThatCanGenerate(t *testing.T) {
	// Given a stored insight that cannot be read
	f := newFolderInsightFixture(t, twoTopicFolder(), nil)
	f.repo.EXPECT().Get(context.Background(), "f1").Return(domainprogress.Insight{}, errors.New("corrupt payload"))

	// When the folder insight is read
	view, err := f.service.GetFolderInsight(context.Background(), "f1")

	// Then the card still offers to generate, which overwrites the bad row
	require.NoError(t, err)
	assert.Nil(t, view.Current)
	assert.True(t, view.CanGenerate)
}

func TestGetFolderInsight_aLanguageErrorDegradesToACardThatCanGenerate(t *testing.T) {
	f := newFolderInsightFixture(t, twoTopicFolder(), nil)
	f.repo.EXPECT().Get(context.Background(), "f1").Return(storedFolderInsight(twoTopicFolderInput(), "en"), nil)
	f.language.EXPECT().AssistantLanguage(context.Background()).Return("", errors.New("profile unreadable"))

	view, err := f.service.GetFolderInsight(context.Background(), "f1")

	require.NoError(t, err)
	assert.Nil(t, view.Current)
	assert.True(t, view.CanGenerate)
}

func TestGetFolderInsight_returnsTheErrorOfTheFolder(t *testing.T) {
	f := newFolderInsightFixture(t, FolderTopicsData{}, ErrFolderNotFound)

	_, err := f.service.GetFolderInsight(context.Background(), "f1")

	require.ErrorIs(t, err, ErrFolderNotFound)
}

func TestGetFolderInsight_withoutThePortsIsTheEmptyView(t *testing.T) {
	// Given a service built without the folder insights
	passScores := progressmocks.NewMockPassScoreProvider(t)

	// When the folder insight is read
	view, err := NewService(fakeReader{}, passScores).GetFolderInsight(context.Background(), "f1")

	// Then there is nothing to show and nothing to generate
	require.NoError(t, err)
	assert.Equal(t, domainprogress.InsightView{}, view)
}
