package folder

import (
	"context"
	"fmt"
)

// SetArchLabEnabled switches the Architecture Lab on or off for the folder
// with the given id. It only gates generating new challenges from the
// folder's sessions; nothing that already exists is touched. See
// specs/phases/phase-07-improve-arch-lab/01-arch-lab-per-folder.md.
func (s *Service) SetArchLabEnabled(ctx context.Context, id string, enabled bool) error {
	if err := s.folders.SetArchLabEnabled(ctx, id, enabled); err != nil {
		return fmt.Errorf("folder: setting architecture lab flag: %w", err)
	}
	return nil
}
