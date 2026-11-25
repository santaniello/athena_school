// Package progress holds the use case behind the Progress dashboard: it reads
// what the study, quiz and Arch Lab contexts already stored, computes the pass
// outcome of the lab attempts with the Arch Lab's own rule and hands everything
// to the progress domain. Nothing is stored and no LLM is called. See
// specs/phases/phase-05-progress/01-progress-dashboard.md.
package progress

import (
	"context"
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
}

// PassScoreProvider supplies the minimum passing score in force.
type PassScoreProvider interface {
	Current(ctx context.Context) (int, error)
}

// FolderData is a folder with its sessions.
type FolderData struct {
	ID       string
	Name     string
	Sessions []SessionData
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
