package onboarding

import (
	"fmt"
	"strings"

	domainprofile "github.com/santaniello/athena/internal/domain/profile"
)

// UpdateProfile normalizes and validates profile, then persists it while
// preserving the original CreatedAt from the currently saved profile. Used
// by Settings, where editing an existing profile is an update, not a
// re-creation — unlike SaveProfile, which always stamps a fresh CreatedAt.
// See specs/phases/phase-01-desktop-mvp/08-settings.md.
func (s *Service) UpdateProfile(profile domainprofile.UserProfile) (domainprofile.UserProfile, error) {
	existing, err := s.profiles.Load()
	if err != nil {
		return domainprofile.UserProfile{}, fmt.Errorf("onboarding: loading existing profile: %w", err)
	}

	profile.Name = strings.TrimSpace(profile.Name)
	profile.AssistantName = strings.TrimSpace(profile.AssistantName)
	profile.Area = strings.TrimSpace(profile.Area)
	profile.StudyStyle = strings.TrimSpace(profile.StudyStyle)
	profile.CreatedAt = existing.CreatedAt

	if err := profile.Validate(); err != nil {
		return domainprofile.UserProfile{}, err
	}

	if err := s.profiles.Save(profile); err != nil {
		return domainprofile.UserProfile{}, fmt.Errorf("onboarding: saving profile: %w", err)
	}

	return profile, nil
}
