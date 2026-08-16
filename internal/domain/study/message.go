package study

import "time"

// Message roles persisted for a study session. The system prompt is never
// persisted as a message row — it is rebuilt from the profile every turn.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Message is a single user or assistant turn within a study session.
type Message struct {
	ID        string
	SessionID string
	Role      string
	Content   string
	CreatedAt time.Time
}
