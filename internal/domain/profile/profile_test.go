package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func validProfile() UserProfile {
	return UserProfile{
		Name:              "Ana",
		AssistantName:     "Atena",
		Area:              "Engenharia de Software",
		ExperienceLevel:   ExperienceLevelIntermediate,
		StudyStyle:        StudyStylePracticalExamples,
		AssistantLanguage: AssistantLanguageEnglish,
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

func TestUserProfile_Validate_returnsError_whenStudyStyleIsNotOneOfTheAllowedValues(t *testing.T) {
	// Given a profile with a study style outside the allowed values
	p := validProfile()
	p.StudyStyle = "whatever"

	// When validating it
	err := p.Validate()

	// Then it fails with the invalid-study-style error
	assert.ErrorIs(t, err, ErrInvalidStudyStyle)
}

func TestUserProfile_Validate_returnsError_whenAssistantLanguageIsNotOneOfTheAllowedValues(t *testing.T) {
	// Given a profile with an assistant language outside pt/en
	p := validProfile()
	p.AssistantLanguage = "fr"

	// When validating it
	err := p.Validate()

	// Then it fails with the invalid-assistant-language error
	assert.ErrorIs(t, err, ErrInvalidAssistantLanguage)
}
