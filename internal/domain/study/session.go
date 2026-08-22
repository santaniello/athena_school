// Package study holds the StudySession/Message domain model and the ports
// (SessionRepository, MessageRepository) infrastructure adapters implement.
// See specs/phases/phase-01-desktop-mvp/06-study-mode.md.
package study

import (
	"errors"
	"time"
)

// ModeStudy identifies study-mode sessions in the shared sessions table;
// other modes (challenge, interview) land in later phases.
const ModeStudy = "study"

// ErrSessionContextLimitReached is returned when a turn is attempted on a
// session whose ContextUsage.State is already ContextStateBlocked. Checked
// inside application/study.Service against persisted state so a forged
// desktop call cannot bypass it. See
// specs/phases/phase-02-knowledge-engine/06-study-context-limits.md.
var ErrSessionContextLimitReached = errors.New("study session has reached its context limit")

// ContextState is how close a session's persisted conversation history is
// to the active model's context window.
type ContextState string

// The three ContextState values, in ascending order of severity: normal
// (used tokens < 80% of the context window), warning (>= 80%), blocked
// (>= 95%, sends are rejected).
const (
	ContextStateNormal  ContextState = "normal"
	ContextStateWarning ContextState = "warning"
	ContextStateBlocked ContextState = "blocked"
)

// ContextUsage is a session's last-measured occupancy of its model's
// context window, persisted so a warning/block can be restored on resume
// without another LLM call. See NextContextUsage for how it's updated.
type ContextUsage struct {
	State ContextState
	// Model is the exact resolved model ID this measurement was taken
	// against, or "" if unresolved.
	Model string
	// UsedTokens is the input+output token occupancy of the last completed
	// request/response for this session (not incremental across turns; see
	// NextContextUsage).
	UsedTokens int
	// ContextLength is the active model's context window, in tokens, or 0
	// if not yet resolved from the model catalog.
	ContextLength int
	// Estimated is true when UsedTokens came from the conservative
	// character-count formula rather than the provider's native usage.
	Estimated bool
}

// Session is a single study conversation about a topic.
type Session struct {
	ID        string
	Topic     string
	Mode      string
	FolderID  string // always populated; falls back to folder.DefaultFolderID
	StartedAt time.Time
	Context   ContextUsage
}
