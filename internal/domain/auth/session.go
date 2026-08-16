package auth

import "time"

// Session is the local session marker written on successful login. There is
// no token and no expiry tied to a server.
type Session struct {
	AccountID string
	CreatedAt time.Time
}

// SessionStore persists the local session marker.
type SessionStore interface {
	Save(session Session) error
	Load() (Session, error)
	Clear() error
}
