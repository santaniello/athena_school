package onboarding

import (
	"fmt"
	"strings"
	"time"

	domainprofile "github.com/santaniello/athena/internal/domain/profile"
)

// SaveProfile normalizes and validates profile, stamps its CreatedAt, and
// persists it. On validation failure it never reaches the store.
func (s *Service) SaveProfile(profile domainprofile.UserProfile) (domainprofile.UserProfile, error) {
	profile.Name = strings.TrimSpace(profile.Name)
	profile.AssistantName = strings.TrimSpace(profile.AssistantName)
	profile.Area = strings.TrimSpace(profile.Area)
	profile.Specialty = strings.TrimSpace(profile.Specialty)
	profile.StudyStyle = strings.TrimSpace(profile.StudyStyle)
	profile.CreatedAt = time.Now().UTC()

	if err := profile.Validate(); err != nil {
		return domainprofile.UserProfile{}, err
	}

	if err := s.profiles.Save(profile); err != nil {
		return domainprofile.UserProfile{}, fmt.Errorf("onboarding: saving profile: %w", err)
	}

	return profile, nil
}
