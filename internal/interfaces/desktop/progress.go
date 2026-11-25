package desktop

import (
	"context"
	"errors"

	domainprogress "github.com/santaniello/athena/internal/domain/progress"
)

// progressDashboard is the use case behind GetProgressDashboard (spec 5.1).
type progressDashboard interface {
	GetDashboard(ctx context.Context) (domainprogress.Dashboard, error)
}

// WithProgressService wires the Progress dashboard.
func WithProgressService(dashboard progressDashboard) Option {
	return func(a *App) { a.progress = dashboard }
}

var errProgressUnavailable = errors.New("progress is not available")

// ProgressCountersResult is the header of the Progress screen.
type ProgressCountersResult struct {
	Folders        int `json:"folders"`
	Topics         int `json:"topics"`
	TopicsToReview int `json:"topicsToReview"`
}

// ProgressTopicResult is one topic (a study session) of a folder. A nil score
// means there is nothing to score yet.
type ProgressTopicResult struct {
	SessionID string `json:"sessionId"`
	Name      string `json:"name"`
	QuizScore *int   `json:"quizScore"`
	LabScore  *int   `json:"labScore"`
	Status    string `json:"status"`
}

// ProgressFolderResult is a folder that has at least one topic.
type ProgressFolderResult struct {
	ID         string                `json:"id"`
	Name       string                `json:"name"`
	TopicCount int                   `json:"topicCount"`
	Topics     []ProgressTopicResult `json:"topics"`
}

// ProgressDashboardResult is everything the Progress screen shows.
type ProgressDashboardResult struct {
	Counters ProgressCountersResult `json:"counters"`
	Folders  []ProgressFolderResult `json:"folders"`
}

// GetProgressDashboard returns the counters and, per folder with sessions, its
// topics with their scores and status. It is derived on read: no LLM call.
func (a *App) GetProgressDashboard() (ProgressDashboardResult, error) {
	if a.progress == nil {
		return ProgressDashboardResult{}, errProgressUnavailable
	}
	dashboard, err := a.progress.GetDashboard(a.ctx)
	if err != nil {
		return ProgressDashboardResult{}, err
	}
	result := ProgressDashboardResult{
		Counters: ProgressCountersResult{
			Folders:        dashboard.Counters.Folders,
			Topics:         dashboard.Counters.Topics,
			TopicsToReview: dashboard.Counters.TopicsToReview,
		},
		Folders: make([]ProgressFolderResult, len(dashboard.Folders)),
	}
	for i, folder := range dashboard.Folders {
		result.Folders[i] = toProgressFolderResult(folder)
	}
	return result, nil
}

func toProgressFolderResult(folder domainprogress.FolderSummary) ProgressFolderResult {
	result := ProgressFolderResult{
		ID: folder.ID, Name: folder.Name, TopicCount: len(folder.Topics),
		Topics: make([]ProgressTopicResult, len(folder.Topics)),
	}
	for i, topic := range folder.Topics {
		result.Topics[i] = ProgressTopicResult{
			SessionID: topic.SessionID, Name: topic.Name,
			QuizScore: topic.QuizScore, LabScore: topic.LabScore, Status: string(topic.Status),
		}
	}
	return result
}
