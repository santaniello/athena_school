package progress

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// FolderInsightDifficulty is something a topic of the folder struggles with
// (spec 5.2), by name.
type FolderInsightDifficulty struct {
	Name     string
	ToReview bool
}

// FolderInsightTopic is a topic of the folder as the folder insight sees it:
// its summary and its difficulties, never its raw attempts, so the prompt
// stays bounded however large the folder. Fingerprint is the topic's own
// (see Fingerprint), so new activity in it moves the folder's.
type FolderInsightTopic struct {
	SessionID    string
	Name         string
	QuizScore    *int
	LabScore     *int
	Status       Status
	Fingerprint  string
	Difficulties []FolderInsightDifficulty
}

// FolderInsightInput is what a folder insight is written from. Each topic is
// given a ref ("T1") in the prompt, which the model cites back.
type FolderInsightInput struct {
	FolderID   string
	FolderName string
	Topics     []FolderInsightTopic
}

// TopicRef is the id of the i-th topic (from 0) in the prompt.
func (FolderInsightInput) TopicRef(i int) string { return fmt.Sprintf("T%d", i+1) }

// HasEvidence reports whether some topic is outside Insufficient evidence;
// without one there is nothing to analyse (spec 5.5, decision 6).
func (in FolderInsightInput) HasEvidence() bool {
	for _, topic := range in.Topics {
		if topic.Status != StatusInsufficientEvidence {
			return true
		}
	}
	return false
}

// FolderFingerprint is a hash of the language, the folder's name and, for each
// topic in any order, everything the prompt shows of it (name, status, scores,
// difficulties) with its own fingerprint, which catches new activity. Moving a
// session in or out of the folder, deleting it, renaming it, a new pass score
// that moves a status, or new activity in any topic moves it.
func FolderFingerprint(in FolderInsightInput, language string) string {
	lines := make([]string, len(in.Topics))
	for i, topic := range in.Topics {
		lines[i] = fmt.Sprintf("topic %q %q %q %q %s %s %q", topic.SessionID, topic.Name, string(topic.Status),
			topic.Fingerprint, scoreKey(topic.QuizScore), scoreKey(topic.LabScore), difficultiesKey(topic.Difficulties))
	}
	sort.Strings(lines)
	header := fmt.Sprintf("language %q\nfolder %q\n", language, in.FolderName)
	sum := sha256.Sum256([]byte(header + strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

// scoreKey tells a missing score from a zero one.
func scoreKey(score *int) string {
	if score == nil {
		return "none"
	}
	return fmt.Sprintf("%d", *score)
}

func difficultiesKey(difficulties []FolderInsightDifficulty) string {
	parts := make([]string, len(difficulties))
	for i, difficulty := range difficulties {
		parts[i] = fmt.Sprintf("%q:%t", difficulty.Name, difficulty.ToReview)
	}
	return strings.Join(parts, ",")
}

// FolderInsightGenerator writes the insight of a folder from its topics, in the
// given assistant language. Implemented in the application layer on top of the
// LLM provider.
type FolderInsightGenerator interface {
	GenerateFolderInsight(ctx context.Context, in FolderInsightInput, language string) (RawInsight, error)
}

// FolderInsightRepository stores the latest insight of each folder.
type FolderInsightRepository interface {
	// Get returns the insight of the folder, or ErrInsightNotFound.
	Get(ctx context.Context, folderID string) (Insight, error)
	// Save stores the insight of the folder, replacing any previous one.
	Save(ctx context.Context, folderID string, insight Insight) error
}

// ParseFolderInsight validates the model's JSON answer like ParseInsight, but
// the activities it may cite are the topics of the folder.
func ParseFolderInsight(raw string, in FolderInsightInput) (InsightContent, error) {
	known := make(map[string]Citation, len(in.Topics))
	for i, topic := range in.Topics {
		known[in.TopicRef(i)] = Citation{Kind: CitationTopic, ID: topic.SessionID, Label: topic.Name}
	}
	return parseContent(raw, known)
}
