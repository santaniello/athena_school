// Package auth holds the local auth use cases: Register, Login and
// ResetLocalAccount. See specs/phases/phase-01-desktop-mvp/01-auth-backend.md.
package auth

import (
	domainauth "github.com/santaniello/athena/internal/domain/auth"
)

// Service implements the local auth use cases against an
// domainauth.AccountRepository and a domainauth.SessionStore.
type Service struct {
	accounts domainauth.AccountRepository
	sessions domainauth.SessionStore
}

// NewService creates a Service backed by the given repository and session
// store.
func NewService(accounts domainauth.AccountRepository, sessions domainauth.SessionStore) *Service {
	return &Service{accounts: accounts, sessions: sessions}
}
