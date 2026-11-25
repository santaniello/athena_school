// Package progresssource adapts the profile store to the ports of the Progress
// use case, so application/progress never imports the profile domain.
package progresssource

import (
	"context"
	"fmt"

	domainprofile "github.com/santaniello/athena/internal/domain/profile"
)

// Profiles reads the assistant language of the user's profile.
type Profiles struct{ profiles domainprofile.Store }

// NewProfiles creates a Profiles reading from profiles.
func NewProfiles(profiles domainprofile.Store) *Profiles { return &Profiles{profiles: profiles} }

// AssistantLanguage returns the assistant language code of the profile.
func (r *Profiles) AssistantLanguage(context.Context) (string, error) {
	profile, err := r.profiles.Load()
	if err != nil {
		return "", fmt.Errorf("progresssource: loading profile: %w", err)
	}
	return profile.AssistantLanguage, nil
}
