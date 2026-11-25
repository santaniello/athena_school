package progress

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func topic(id, name string, startedDay int) TopicInput {
	return TopicInput{SessionID: id, Name: name, StartedAt: at(startedDay)}
}

func TestBuildDashboard_ListsOnlyFoldersWithSessions(t *testing.T) {
	folders := []FolderInput{
		{ID: "f1", Name: "System Design", Topics: []TopicInput{topic("s1", "Cache", 1)}},
		{ID: "f2", Name: "Empty"},
	}

	dashboard := BuildDashboard(folders, testPassScore)

	require.Len(t, dashboard.Folders, 1)
	assert.Equal(t, "f1", dashboard.Folders[0].ID)
	assert.Equal(t, "System Design", dashboard.Folders[0].Name)
	assert.Equal(t, 1, dashboard.Counters.Folders)
}

func TestBuildDashboard_KeepsTheFolderOrderItWasGiven(t *testing.T) {
	folders := []FolderInput{
		{ID: "f2", Name: "B", Topics: []TopicInput{topic("s2", "Two", 1)}},
		{ID: "f1", Name: "A", Topics: []TopicInput{topic("s1", "One", 1)}},
	}

	dashboard := BuildDashboard(folders, testPassScore)

	require.Len(t, dashboard.Folders, 2)
	assert.Equal(t, "f2", dashboard.Folders[0].ID)
	assert.Equal(t, "f1", dashboard.Folders[1].ID)
}

func TestBuildDashboard_OrdersTopicsByMostRecentActivity(t *testing.T) {
	recentQuiz := topic("s2", "Quizzed", 1)
	recentQuiz.Quizzes = []QuizInput{{QuizID: "q", Attempts: []ScoredAttempt{{Percent: 90, At: at(30)}}}}
	folders := []FolderInput{{ID: "f1", Name: "F", Topics: []TopicInput{
		topic("s1", "Old", 2),
		recentQuiz,
		topic("s3", "Newer", 10),
	}}}

	dashboard := BuildDashboard(folders, testPassScore)

	require.Len(t, dashboard.Folders[0].Topics, 3)
	assert.Equal(t, "s2", dashboard.Folders[0].Topics[0].SessionID)
	assert.Equal(t, "s3", dashboard.Folders[0].Topics[1].SessionID)
	assert.Equal(t, "s1", dashboard.Folders[0].Topics[2].SessionID)
}

func TestBuildDashboard_KeepsTheGivenOrderOnTies(t *testing.T) {
	folders := []FolderInput{{ID: "f1", Name: "F", Topics: []TopicInput{
		topic("s1", "First", 1),
		topic("s2", "Second", 1),
	}}}

	dashboard := BuildDashboard(folders, testPassScore)

	assert.Equal(t, "s1", dashboard.Folders[0].Topics[0].SessionID)
	assert.Equal(t, "s2", dashboard.Folders[0].Topics[1].SessionID)
}

func TestBuildDashboard_CountsFoldersTopicsAndTopicsToReview(t *testing.T) {
	weak := topic("s2", "Weak", 2)
	weak.Quizzes = []QuizInput{quiz("q1", 40)}
	alsoWeak := topic("s3", "Also weak", 3)
	alsoWeak.Concepts = []ConceptInput{{NeedsReview: true, EvidenceCount: 1}}
	folders := []FolderInput{
		{ID: "f1", Name: "A", Topics: []TopicInput{topic("s1", "Fine", 1), weak}},
		{ID: "f2", Name: "B", Topics: []TopicInput{alsoWeak}},
		{ID: "f3", Name: "C"},
	}

	dashboard := BuildDashboard(folders, testPassScore)

	assert.Equal(t, Counters{Folders: 2, Topics: 3, TopicsToReview: 2}, dashboard.Counters)
}

func TestBuildDashboard_WithoutFoldersIsEmpty(t *testing.T) {
	dashboard := BuildDashboard(nil, testPassScore)

	assert.Empty(t, dashboard.Folders)
	assert.Equal(t, Counters{}, dashboard.Counters)
}

func TestBuildDashboard_SummarizesEachTopicWithThePassScore(t *testing.T) {
	scored := topic("s1", "Cache", 1)
	scored.Quizzes = []QuizInput{quiz("q1", 70)}
	folders := []FolderInput{{ID: "f1", Name: "F", Topics: []TopicInput{scored}}}

	strict := BuildDashboard(folders, 80)
	lenient := BuildDashboard(folders, 60)

	assert.Equal(t, StatusNeedsReinforcement, strict.Folders[0].Topics[0].Status)
	assert.Equal(t, StatusStable, lenient.Folders[0].Topics[0].Status)
}
