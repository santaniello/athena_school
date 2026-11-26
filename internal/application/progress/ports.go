// Package progress holds the use case behind the Progress dashboard: it reads
// what the study, quiz and Arch Lab contexts already stored, computes the pass
// outcome of the lab attempts with the Arch Lab's own rule and hands everything
// to the progress domain. Nothing is stored and no LLM is called. See
// specs/phases/phase-05-progress/01-progress-dashboard.md.
package progress

import (
	"context"
	"errors"
	"time"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Reader reads, in one pass, the folders and, for every session in them, the
// quizzes, challenges and concepts that belong to it (decision 2 of the spec).
// Defined here (consumer side) per Go convention; implemented by the SQLite
// read adapter.
type Reader interface {
	// ReadFolders returns every folder, in the order of the Folders screen,
	// with its sessions. A folder without sessions comes back empty.
	ReadFolders(ctx context.Context) ([]FolderData, error)
	// ReadTopic returns one session with the detail of everything attached to
	// it, or ErrTopicNotFound when the session does not exist or is in no
	// folder.
	ReadTopic(ctx context.Context, sessionID string) (TopicData, error)
	// ReadFolderTopics returns a folder with the detail of every topic in it,
	// or ErrFolderNotFound when the folder does not exist.
	ReadFolderTopics(ctx context.Context, folderID string) (FolderTopicsData, error)
}

// ErrTopicNotFound is returned when the session of a topic does not exist.
var ErrTopicNotFound = errors.New("progress topic not found")

// ErrFolderNotFound is returned when a folder does not exist.
var ErrFolderNotFound = errors.New("progress folder not found")

// PassScoreProvider supplies the minimum passing score in force.
type PassScoreProvider interface {
	Current(ctx context.Context) (int, error)
}

// LanguageProvider supplies the assistant language of the profile ("pt" or
// "en"), the one the insights are written in (spec 5.4, decision 8).
type LanguageProvider interface {
	AssistantLanguage(ctx context.Context) (string, error)
}

// Clock tells the current time, so the time an insight is stored with is
// testable.
type Clock interface {
	Now() time.Time
}

// FolderData is a folder with its sessions.
type FolderData struct {
	ID       string
	Name     string
	Sessions []SessionData
}

// FolderTopicsData is a folder with the detail of each of its topics, the
// oldest session first.
type FolderTopicsData struct {
	ID     string
	Name   string
	Topics []TopicData
}

// SessionData is a session with everything attached to it.
type SessionData struct {
	ID         string
	Topic      string
	StartedAt  time.Time
	Quizzes    []QuizData
	Challenges []ChallengeData
	Concepts   []ConceptData
}

// QuizData is a quiz of the session with its attempts, oldest first.
type QuizData struct {
	ID       string
	Attempts []QuizAttemptData
}

// QuizAttemptData is one finished quiz attempt.
type QuizAttemptData struct {
	CorrectCount int
	Total        int
	FinishedAt   time.Time
}

// ChallengeData is a challenge of the session, counted once, with all its
// attempts, oldest first, whatever they are linked to.
type ChallengeData struct {
	ID       string
	Attempts []domainexercise.AttemptRecord
}

// ConceptData is the state of one concept of the session.
type ConceptData struct {
	Status        domainstudy.ConceptStatus
	EvidenceCount int
	UpdatedAt     time.Time
}

// TopicData is a topic (a study session) with what the detail panel needs:
// the quizzes and challenges that belong to it, oldest first, and its
// concepts with their proofs.
type TopicData struct {
	SessionID  string
	Topic      string
	FolderName string
	Quizzes    []TopicQuizData
	Challenges []TopicChallengeData
	Concepts   []TopicConceptData
}

// TopicQuizData is a quiz of the topic with its attempts, oldest first.
type TopicQuizData struct {
	ID         string
	Difficulty string
	Attempts   []QuizAttemptData
}

// TopicChallengeData is a challenge of the topic, counted once, with its title
// and all its attempts, oldest first, whatever they are linked to.
type TopicChallengeData struct {
	ID               string
	Title            string
	Difficulty       string
	TimeLimitMinutes int
	Attempts         []domainexercise.AttemptRecord
}

// TopicConceptData is a concept of the session with its proofs.
type TopicConceptData struct {
	Name          string
	DisplayName   string
	Status        domainstudy.ConceptStatus
	EvidenceCount int
	UpdatedAt     time.Time
	Proofs        []domainstudy.Evidence
}
