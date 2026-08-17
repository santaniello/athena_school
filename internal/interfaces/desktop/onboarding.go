package desktop

import (
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
)

// UserProfileInput is the desktop-facing DTO for SaveProfile. It is
// deliberately decoupled from profile.UserProfile's on-disk JSON shape.
type UserProfileInput struct {
	Name              string   `json:"name"`
	AssistantName     string   `json:"assistantName"`
	Area              string   `json:"area"`
	ExperienceLevel   string   `json:"experienceLevel"`
	Goals             []string `json:"goals"`
	StudyStyle        string   `json:"studyStyle"`
	AssistantLanguage string   `json:"assistantLanguage"`
}

// HasOpenRouterKey reports whether an OpenRouter key is already configured,
// so the frontend can skip the key gate screen on subsequent launches.
func (a *App) HasOpenRouterKey() bool {
	cfg, err := a.config.Load()
	return err == nil && cfg.OpenRouterKey != ""
}

// SaveOpenRouterKey validates key against OpenRouter and, on success,
// persists it locally and updates the live LLM client so already-running
// study sessions pick it up without an app restart. See
// specs/phases/phase-01-desktop-mvp/04-onboarding.md and
// specs/phases/phase-01-desktop-mvp/08-settings.md.
func (a *App) SaveOpenRouterKey(key string) error {
	if err := a.onboarding.SaveOpenRouterKey(a.ctx, key); err != nil {
		return err
	}
	a.apiKeyUpdater.SetAPIKey(key)
	return nil
}

// HasUserProfile reports whether onboarding has already been completed, so
// the frontend can skip straight to the main screen on subsequent launches.
func (a *App) HasUserProfile() bool {
	_, err := a.profiles.Load()
	return err == nil
}

// GetProfile reads back the saved UserProfile, so the frontend can
// personalize the home screen greeting. See
// specs/phases/phase-01-desktop-mvp/03-home-screen.md.
func (a *App) GetProfile() (UserProfileInput, error) {
	profile, err := a.profiles.Load()
	if err != nil {
		return UserProfileInput{}, err
	}
	return UserProfileInput{
		Name:              profile.Name,
		AssistantName:     profile.AssistantName,
		Area:              profile.Area,
		ExperienceLevel:   profile.ExperienceLevel,
		Goals:             profile.Goals,
		StudyStyle:        profile.StudyStyle,
		AssistantLanguage: profile.AssistantLanguage,
	}, nil
}

// SaveProfile validates and persists the profile collected during
// onboarding.
func (a *App) SaveProfile(input UserProfileInput) error {
	_, err := a.onboarding.SaveProfile(domainprofile.UserProfile{
		Name:              input.Name,
		AssistantName:     input.AssistantName,
		Area:              input.Area,
		ExperienceLevel:   input.ExperienceLevel,
		Goals:             input.Goals,
		StudyStyle:        input.StudyStyle,
		AssistantLanguage: input.AssistantLanguage,
	})
	return err
}
