// Package study holds the StudySession/Message domain model and the ports
// (SessionRepository, MessageRepository) infrastructure adapters implement.
// See specs/phases/phase-01-desktop-mvp/06-study-mode.md.
package study

import "time"

// ModeStudy identifies study-mode sessions in the shared sessions table;
// other modes (challenge, interview) land in later phases.
const ModeStudy = "study"

// Session is a single study conversation about a topic.
type Session struct {
	ID        string
	Topic     string
	Mode      string
	FolderID  string // always populated; falls back to folder.DefaultFolderID
	StartedAt time.Time
	EndedAt   time.Time
}

// IsOpen reports whether the session has not been ended yet.
func (s Session) IsOpen() bool {
	return s.EndedAt.IsZero()
}
