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
