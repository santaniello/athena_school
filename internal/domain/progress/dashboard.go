package progress

import "sort"

// FolderInput is a folder with the topics (study sessions) it holds.
type FolderInput struct {
	ID     string
	Name   string
	Topics []TopicInput
}

// Counters are the numbers of the dashboard header.
type Counters struct {
	Folders        int // folders with at least one topic
	Topics         int
	TopicsToReview int // topics in NEEDS_REINFORCEMENT
}

// FolderSummary is a folder of the dashboard with its topics, the one with the
// most recent activity first.
type FolderSummary struct {
	ID     string
	Name   string
	Topics []TopicSummary
}

// Dashboard is everything the Progress screen shows.
type Dashboard struct {
	Counters Counters
	Folders  []FolderSummary
}

// BuildDashboard summarizes every topic and drops the folders that have none.
// The folders keep the order they were given in; the topics of each one are
// ordered by most recent activity, ties keeping the given order.
func BuildDashboard(folders []FolderInput, passScore int) Dashboard {
	dashboard := Dashboard{Folders: []FolderSummary{}}
	for _, folder := range folders {
		if len(folder.Topics) == 0 {
			continue
		}
		summary := FolderSummary{ID: folder.ID, Name: folder.Name, Topics: make([]TopicSummary, len(folder.Topics))}
		for i, topic := range folder.Topics {
			summary.Topics[i] = SummarizeTopic(topic, passScore)
			if summary.Topics[i].Status == StatusNeedsReinforcement {
				dashboard.Counters.TopicsToReview++
			}
		}
		sort.SliceStable(summary.Topics, func(i, j int) bool {
			return summary.Topics[i].LastActivity.After(summary.Topics[j].LastActivity)
		})
		dashboard.Counters.Folders++
		dashboard.Counters.Topics += len(summary.Topics)
		dashboard.Folders = append(dashboard.Folders, summary)
	}
	return dashboard
}
