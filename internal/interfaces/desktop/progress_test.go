package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainprogress "github.com/santaniello/athena/internal/domain/progress"
)

type fakeProgressDashboard struct {
	dashboard domainprogress.Dashboard
	err       error
	detail    domainprogress.TopicDetail
	detailErr error
	asked     *string
}

func (f fakeProgressDashboard) GetTopicDetail(_ context.Context, sessionID string) (domainprogress.TopicDetail, error) {
	if f.asked != nil {
		*f.asked = sessionID
	}
	return f.detail, f.detailErr
}

func (f fakeProgressDashboard) GetDashboard(context.Context) (domainprogress.Dashboard, error) {
	return f.dashboard, f.err
}

func intPtr(v int) *int { return &v }

func TestGetProgressDashboard_failsWhenTheServiceIsNotWired(t *testing.T) {
	// Given an app without the progress service
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// When the dashboard is requested
	_, err := app.GetProgressDashboard()

	// Then it fails clearly instead of panicking
	require.Error(t, err)
}

func TestGetProgressDashboard_returnsTheErrorOfTheService(t *testing.T) {
	// Given a service that fails
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		WithProgressService(fakeProgressDashboard{err: errors.New("database locked")}))

	// When the dashboard is requested
	_, err := app.GetProgressDashboard()

	// Then the error reaches the frontend
	require.ErrorContains(t, err, "database locked")
}

func TestGetProgressDashboard_mapsCountersFoldersAndTopics(t *testing.T) {
	// Given a dashboard with a scored topic and an unscored one
	dashboard := domainprogress.Dashboard{
		Counters: domainprogress.Counters{Folders: 1, Topics: 2, TopicsToReview: 1},
		Folders: []domainprogress.FolderSummary{{
			ID: "f1", Name: "System Design",
			Topics: []domainprogress.TopicSummary{
				{SessionID: "s1", Name: "Cache", QuizScore: intPtr(85), LabScore: intPtr(63), Status: domainprogress.StatusNeedsReinforcement, LastActivity: time.Now()},
				{SessionID: "s2", Name: "Queues", Status: domainprogress.StatusInsufficientEvidence},
			},
		}},
	}
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithProgressService(fakeProgressDashboard{dashboard: dashboard}))

	// When the dashboard is requested
	result, err := app.GetProgressDashboard()

	// Then everything the screen shows is mapped
	require.NoError(t, err)
	assert.Equal(t, ProgressCountersResult{Folders: 1, Topics: 2, TopicsToReview: 1}, result.Counters)
	require.Len(t, result.Folders, 1)
	assert.Equal(t, "f1", result.Folders[0].ID)
	assert.Equal(t, "System Design", result.Folders[0].Name)
	assert.Equal(t, 2, result.Folders[0].TopicCount)
	require.Len(t, result.Folders[0].Topics, 2)
	assert.Equal(t, ProgressTopicResult{SessionID: "s1", Name: "Cache", QuizScore: intPtr(85), LabScore: intPtr(63), Status: "NEEDS_REINFORCEMENT"}, result.Folders[0].Topics[0])
	assert.Nil(t, result.Folders[0].Topics[1].QuizScore)
	assert.Nil(t, result.Folders[0].Topics[1].LabScore)
	assert.Equal(t, "INSUFFICIENT_EVIDENCE", result.Folders[0].Topics[1].Status)
}

func TestGetProgressDashboard_serializesForTheFrontend(t *testing.T) {
	// Given a dashboard with an unscored topic and no folder in the other case
	dashboard := domainprogress.Dashboard{Folders: []domainprogress.FolderSummary{{
		ID: "f1", Name: "F", Topics: []domainprogress.TopicSummary{{SessionID: "s1", Name: "Cache", Status: domainprogress.StatusStable}},
	}}}
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithProgressService(fakeProgressDashboard{dashboard: dashboard}))

	// When the result is serialized
	result, err := app.GetProgressDashboard()
	require.NoError(t, err)
	raw, err := json.Marshal(result)
	require.NoError(t, err)

	// Then the keys are the camelCase ones the frontend reads, and a missing score is null
	assert.JSONEq(t, `{
		"counters": {"folders": 0, "topics": 0, "topicsToReview": 0},
		"folders": [{"id": "f1", "name": "F", "topicCount": 1, "topics": [
			{"sessionId": "s1", "name": "Cache", "quizScore": null, "labScore": null, "status": "STABLE"}
		]}]
	}`, string(raw))
}

func TestGetProgressDashboard_returnsEmptyListsNotNull(t *testing.T) {
	// Given a dashboard without folders
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithProgressService(fakeProgressDashboard{}))

	// When it is serialized
	result, err := app.GetProgressDashboard()
	require.NoError(t, err)
	raw, err := json.Marshal(result)
	require.NoError(t, err)

	// Then the folders are [], so the frontend can map over them
	assert.Contains(t, string(raw), `"folders":[]`)
}

func TestGetProgressTopic_failsWhenTheServiceIsNotWired(t *testing.T) {
	// Given an app without the progress service
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// When a topic is requested
	_, err := app.GetProgressTopic("s1")

	// Then it fails clearly instead of panicking
	require.Error(t, err)
}

func TestGetProgressTopic_returnsTheErrorOfTheService(t *testing.T) {
	// Given a service that fails
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		WithProgressService(fakeProgressDashboard{detailErr: errors.New("database locked")}))

	// When a topic is requested
	_, err := app.GetProgressTopic("s1")

	// Then the error reaches the frontend
	require.ErrorContains(t, err, "database locked")
}

func TestGetProgressTopic_mapsTheDetailAndAsksForTheSession(t *testing.T) {
	// Given a detail with every kind of entry
	proofAt := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	proofs := []domainprogress.DetailProof{{Assessment: "CORRECT", Description: "explained it", Assisted: true, At: proofAt}}
	detail := domainprogress.TopicDetail{
		SessionID: "s1", Name: "Cache", FolderName: "System Design", QuizCount: 1, ChallengeCount: 1, ConceptCount: 3,
		Quizzes: []domainprogress.QuizResult{{QuizID: "q1", Number: 2, Difficulty: "BEGINNER", Percent: intPtr(75), Attempts: 3}},
		Challenges: []domainprogress.ChallengeResult{{
			ChallengeID: "c1", Title: "Design a cache", Difficulty: "ADVANCED", TimeLimitMinutes: 60, DiagramScore: intPtr(85), Result: domainprogress.LabResultPending, Attempts: 1,
		}},
		Understood: []domainprogress.ConceptDetail{{Name: "Cache-aside", EvidenceCount: 1, Proofs: proofs}},
		Difficulties: []domainprogress.Difficulty{
			{Name: "ttl", Source: domainprogress.DifficultySourceTutor, ToReview: true, EvidenceCount: 2},
			{Name: "Consistency", Source: domainprogress.DifficultySourceArchLab, ChallengeTitle: "Design a cache"},
		},
		ExplainedOnly: []domainprogress.ConceptDetail{{Name: "lru"}},
	}
	var asked string
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		WithProgressService(fakeProgressDashboard{detail: detail, asked: &asked}))

	// When the topic is requested
	result, err := app.GetProgressTopic("s1")
	require.NoError(t, err)
	raw, err := json.Marshal(result)
	require.NoError(t, err)

	// Then the session id is passed on and the JSON has the keys the frontend reads
	assert.Equal(t, "s1", asked)
	assert.JSONEq(t, `{
		"sessionId": "s1", "name": "Cache", "folderName": "System Design",
		"quizCount": 1, "challengeCount": 1, "conceptCount": 3,
		"quizzes": [{"quizId": "q1", "number": 2, "difficulty": "BEGINNER", "percent": 75, "attempts": 3}],
		"challenges": [{"challengeId": "c1", "title": "Design a cache", "difficulty": "ADVANCED", "timeLimitMinutes": 60, "diagramScore": 85, "defenseScore": null, "result": "PENDING", "attempts": 1}],
		"understood": [{"name": "Cache-aside", "evidenceCount": 1, "proofs": [
			{"assessment": "CORRECT", "description": "explained it", "assisted": true, "createdAt": "2026-11-02T10:00:00Z"}]}],
		"difficulties": [
			{"name": "ttl", "source": "TUTOR", "challengeTitle": "", "toReview": true, "evidenceCount": 2, "proofs": []},
			{"name": "Consistency", "source": "ARCH_LAB", "challengeTitle": "Design a cache", "toReview": false, "evidenceCount": 0, "proofs": []}],
		"explainedOnly": [{"name": "lru", "evidenceCount": 0, "proofs": []}]
	}`, string(raw))
}

func TestGetProgressTopic_returnsEmptyListsNotNull(t *testing.T) {
	// Given an empty detail
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithProgressService(fakeProgressDashboard{}))

	// When it is serialized
	result, err := app.GetProgressTopic("s1")
	require.NoError(t, err)
	raw, err := json.Marshal(result)
	require.NoError(t, err)

	// Then every list is [], so the frontend can map over it
	for _, key := range []string{"quizzes", "challenges", "understood", "difficulties", "explainedOnly"} {
		assert.Contains(t, string(raw), `"`+key+`":[]`)
	}
}
