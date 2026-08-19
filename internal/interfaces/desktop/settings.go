package desktop

import (
	"math"

	domainconfig "github.com/santaniello/athena/internal/domain/config"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
)

// KnowledgeExtractionSettings is the desktop-facing extraction configuration.
type KnowledgeExtractionSettings struct {
	MaxKnowledgeExtractionItems int `json:"maxKnowledgeExtractionItems"`
}

// GetKnowledgeExtractionSettings reads the current extraction configuration.
func (a *App) GetKnowledgeExtractionSettings() (KnowledgeExtractionSettings, error) {
	cfg, err := a.config.Load()
	if err != nil {
		return KnowledgeExtractionSettings{}, err
	}
	cfg = cfg.WithDefaults()
	return KnowledgeExtractionSettings{MaxKnowledgeExtractionItems: cfg.MaxKnowledgeExtractionItems}, nil
}

// UpdateKnowledgeExtractionSettings validates and persists the extraction maximum.
func (a *App) UpdateKnowledgeExtractionSettings(maxItems float64) error {
	if math.Trunc(maxItems) != maxItems {
		return domainconfig.ErrMaxKnowledgeExtractionItemsOutOfRange
	}
	cfg, err := a.config.Load()
	if err != nil {
		return err
	}
	cfg.MaxKnowledgeExtractionItems = int(maxItems)
	if err := cfg.Validate(); err != nil {
		return err
	}
	return a.config.Save(domainconfig.Config{
		OpenRouterKey:               cfg.OpenRouterKey,
		MaxKnowledgeExtractionItems: cfg.MaxKnowledgeExtractionItems,
	})
}

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
