package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	domainauth "github.com/santaniello/athena/internal/domain/auth"
)

// Login validates credentials against the local account and, on success,
// writes a local session marker. Both an unknown email and a wrong password
// fail with the same ErrInvalidCredentials, to avoid leaking whether an
// account exists.
func (s *Service) Login(ctx context.Context, email, password string) (domainauth.Account, error) {
	account, err := s.accounts.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domainauth.ErrAccountNotFound) {
			return domainauth.Account{}, ErrInvalidCredentials
		}
		return domainauth.Account{}, fmt.Errorf("auth: finding account: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)); err != nil {
		return domainauth.Account{}, ErrInvalidCredentials
	}

	session := domainauth.Session{AccountID: account.ID, CreatedAt: time.Now().UTC()}
	if err := s.sessions.Save(session); err != nil {
		return domainauth.Account{}, fmt.Errorf("auth: saving session: %w", err)
	}

	return account, nil
}
