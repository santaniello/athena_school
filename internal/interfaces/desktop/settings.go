package desktop

import (
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
)

// UpdateProfile validates and persists changes to the already-saved
// profile, preserving its original CreatedAt. It returns the saved
// (trimmed) profile so the frontend can reflect exactly what was
// persisted without a second GetProfile round trip. See
// specs/phases/phase-01-desktop-mvp/08-settings.md.
func (a *App) UpdateProfile(input UserProfileInput) (UserProfileInput, error) {
	saved, err := a.onboarding.UpdateProfile(domainprofile.UserProfile{
		Name:              input.Name,
		AssistantName:     input.AssistantName,
		Area:              input.Area,
		ExperienceLevel:   input.ExperienceLevel,
		Goals:             input.Goals,
		StudyStyle:        input.StudyStyle,
		AssistantLanguage: input.AssistantLanguage,
	})
	if err != nil {
		return UserProfileInput{}, err
	}
	return UserProfileInput{
		Name:              saved.Name,
		AssistantName:     saved.AssistantName,
		Area:              saved.Area,
		ExperienceLevel:   saved.ExperienceLevel,
		Goals:             saved.Goals,
		StudyStyle:        saved.StudyStyle,
		AssistantLanguage: saved.AssistantLanguage,
	}, nil
}
