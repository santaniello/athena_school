// Package profile holds the UserProfile domain model collected during
// onboarding and the ports infrastructure adapters implement to persist it.
// See specs/phases/phase-01-desktop-mvp/04-onboarding.md.
package profile

import (
	"errors"
	"strings"
	"time"
)

// Allowed values for UserProfile.ExperienceLevel.
const (
	ExperienceLevelBeginner     = "beginner"
	ExperienceLevelIntermediate = "intermediate"
	ExperienceLevelAdvanced     = "advanced"
)

// Sentinel errors returned by UserProfile.Validate, one per required field.
var (
	ErrNameRequired           = errors.New("name is required")
	ErrAssistantNameRequired  = errors.New("assistant name is required")
	ErrAreaRequired           = errors.New("area is required")
	ErrInvalidExperienceLevel = errors.New("experience level must be beginner, intermediate or advanced")
	ErrGoalsRequired          = errors.New("at least one goal is required")
	ErrStudyStyleRequired     = errors.New("study style is required")
)

// UserProfile is generated from the onboarding form and used to personalize
// every future study session.
type UserProfile struct {
	Name            string    `json:"name"`
	AssistantName   string    `json:"assistant_name"`
	Area            string    `json:"area"`
	ExperienceLevel string    `json:"experience_level"` // beginner | intermediate | advanced
	Goals           []string  `json:"goals"`
	StudyStyle      string    `json:"study_style"`
	CreatedAt       time.Time `json:"created_at"`
}

// Validate checks that every required field is present and that
// ExperienceLevel is one of the allowed values. It returns the first
// violated sentinel error.
func (p UserProfile) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return ErrNameRequired
	}
	if strings.TrimSpace(p.AssistantName) == "" {
		return ErrAssistantNameRequired
	}
	if strings.TrimSpace(p.Area) == "" {
		return ErrAreaRequired
	}
	switch p.ExperienceLevel {
	case ExperienceLevelBeginner, ExperienceLevelIntermediate, ExperienceLevelAdvanced:
	default:
		return ErrInvalidExperienceLevel
	}
	if !hasNonBlankGoal(p.Goals) {
		return ErrGoalsRequired
	}
	if strings.TrimSpace(p.StudyStyle) == "" {
		return ErrStudyStyleRequired
	}
	return nil
}

func hasNonBlankGoal(goals []string) bool {
	for _, goal := range goals {
		if strings.TrimSpace(goal) != "" {
			return true
		}
	}
	return false
}
