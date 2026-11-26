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

// FolderFingerprint is a hash of the language and of the set of sessions of the
// folder with each one's own fingerprint, in any order. Moving a session in or
// out of the folder, deleting it, or new activity in any topic moves it.
func FolderFingerprint(in FolderInsightInput, language string) string {
	lines := make([]string, len(in.Topics))
	for i, topic := range in.Topics {
		lines[i] = fmt.Sprintf("topic %q %q", topic.SessionID, topic.Fingerprint)
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(fmt.Sprintf("language %q\n", language) + strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
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
