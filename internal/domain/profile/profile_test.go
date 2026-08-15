package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func validProfile() UserProfile {
	return UserProfile{
		Name:            "Ana",
		AssistantName:   "Atena",
		Area:            "Engenharia de Software",
		ExperienceLevel: ExperienceLevelIntermediate,
		Goals:           []string{"SQL", "System Design"},
		StudyStyle:      "Prática com exercícios",
	}
}

func TestUserProfile_Validate_returnsNil_whenAllFieldsArePresent(t *testing.T) {
	// Given a profile with every required field filled
	p := validProfile()

	// When validating it
	err := p.Validate()

	// Then it passes
	assert.NoError(t, err)
}

func TestUserProfile_Validate_returnsError_whenNameIsEmpty(t *testing.T) {
	// Given a profile with a blank name
	p := validProfile()
	p.Name = "   "

	// When validating it
	err := p.Validate()

	// Then it fails with the name-required error
	assert.ErrorIs(t, err, ErrNameRequired)
}

func TestUserProfile_Validate_returnsError_whenAssistantNameIsEmpty(t *testing.T) {
	// Given a profile with a blank assistant name
	p := validProfile()
	p.AssistantName = ""

	// When validating it
	err := p.Validate()

	// Then it fails with the assistant-name-required error
	assert.ErrorIs(t, err, ErrAssistantNameRequired)
}

func TestUserProfile_Validate_returnsError_whenAreaIsEmpty(t *testing.T) {
	// Given a profile with a blank area
	p := validProfile()
	p.Area = ""

	// When validating it
	err := p.Validate()

	// Then it fails with the area-required error
	assert.ErrorIs(t, err, ErrAreaRequired)
}

func TestUserProfile_Validate_returnsError_whenExperienceLevelIsNotOneOfTheAllowedValues(t *testing.T) {
	// Given a profile with an experience level outside beginner/intermediate/advanced
	p := validProfile()
	p.ExperienceLevel = "expert"

	// When validating it
	err := p.Validate()

	// Then it fails with the invalid-experience-level error
	assert.ErrorIs(t, err, ErrInvalidExperienceLevel)
}

func TestUserProfile_Validate_returnsError_whenGoalsIsEmpty(t *testing.T) {
	// Given a profile with no goals
	p := validProfile()
	p.Goals = nil

	// When validating it
	err := p.Validate()

	// Then it fails with the goals-required error
	assert.ErrorIs(t, err, ErrGoalsRequired)
}

func TestUserProfile_Validate_returnsError_whenGoalsOnlyContainsBlankEntries(t *testing.T) {
	// Given a profile whose goals list has entries that are blank after trimming
	p := validProfile()
	p.Goals = []string{"   ", ""}

	// When validating it
	err := p.Validate()

	// Then it fails with the goals-required error
	assert.ErrorIs(t, err, ErrGoalsRequired)
}

func TestUserProfile_Validate_returnsError_whenStudyStyleIsEmpty(t *testing.T) {
	// Given a profile with a blank study style
	p := validProfile()
	p.StudyStyle = ""

	// When validating it
	err := p.Validate()

	// Then it fails with the study-style-required error
	assert.ErrorIs(t, err, ErrStudyStyleRequired)
}
