package auth

import "context"

// ResetLocalAccount deletes the local account for the given email so a new
// one can be registered with the same address. There is no email-based
// recovery in this phase — this is a destructive local reset, not a real
// recovery flow.
func (s *Service) ResetLocalAccount(ctx context.Context, email string) error {
	account, err := s.accounts.FindByEmail(ctx, email)
	if err != nil {
		return err
	}
	return s.accounts.Delete(ctx, account.ID)
}
