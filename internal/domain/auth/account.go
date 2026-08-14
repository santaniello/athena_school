// Package auth holds the local authentication core: the Account domain
// model and the ports (AccountRepository, SessionStore) that infrastructure
// adapters implement. See specs/phases/phase-01-desktop-mvp/01-auth-backend.md.
package auth

import "time"

// Account is a local user account. PasswordHash is always a bcrypt hash,
// never a plaintext password.
type Account struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}
