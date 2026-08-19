package desktop

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainconfig "github.com/santaniello/athena/internal/domain/config"
	configmocks "github.com/santaniello/athena/internal/domain/config/mocks"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
)

func TestApp_UpdateProfile_savesProfile_andReturnsSavedFields_whenValid(t *testing.T) {
	// Given an App backed by a profile store with an existing profile
	profiles := profilemocks.NewMockStore(t)
	originalCreatedAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	existing := domainprofile.UserProfile{
		Name:              "Ana",
		AssistantName:     "Atena",
		Area:              "Engenharia de Software",
		ExperienceLevel:   domainprofile.ExperienceLevelIntermediate,
		Goals:             []string{"SQL"},
		StudyStyle:        domainprofile.StudyStylePracticalExamples,
		AssistantLanguage: domainprofile.AssistantLanguageEnglish,
		CreatedAt:         originalCreatedAt,
	}
	profiles.EXPECT().Load().Return(existing, nil).Once()
	profiles.EXPECT().
		Save(mock.MatchedBy(func(p domainprofile.UserProfile) bool {
			return p.AssistantName == "Nova Atena" && p.CreatedAt.Equal(originalCreatedAt)
		})).
		Return(nil).
		Once()
	app := newTestOnboardingApp(t, profiles, configmocks.NewMockStore(t), configmocks.NewMockKeyValidator(t))

	input := validProfileInput()
	input.AssistantName = "Nova Atena"

	// When updating the profile
	got, err := app.UpdateProfile(input)

	// Then it succeeds and returns the saved fields
	require.NoError(t, err)
	assert.Equal(t, "Nova Atena", got.AssistantName)
}

func TestApp_GetKnowledgeExtractionSettings_returnsConfiguredMaximum(t *testing.T) {
	// Given an App with a configured extraction maximum
	configs := configmocks.NewMockStore(t)
	configs.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 12}, nil).Once()
	app := NewApp(nil, nil, nil, nil, configs, nil, nil, nil, nil, nil)

	// When reading knowledge extraction settings
	result, err := app.GetKnowledgeExtractionSettings()

	// Then the configured maximum is returned
	require.NoError(t, err)
	assert.Equal(t, 12, result.MaxKnowledgeExtractionItems)
}

func TestApp_UpdateKnowledgeExtractionSettings_validatesAndPreservesOpenRouterKey(t *testing.T) {
	// Given an App with an existing OpenRouter key
	configs := configmocks.NewMockStore(t)
	configs.EXPECT().Load().Return(domainconfig.Config{OpenRouterKey: "sk-or-existing", MaxKnowledgeExtractionItems: 8}, nil).Once()
	configs.EXPECT().Save(domainconfig.Config{OpenRouterKey: "sk-or-existing", MaxKnowledgeExtractionItems: 12}).Return(nil).Once()
	app := NewApp(nil, nil, nil, nil, configs, nil, nil, nil, nil, nil)

	// When changing the extraction maximum
	err := app.UpdateKnowledgeExtractionSettings(12)

	// Then the setting is saved without replacing the key
	require.NoError(t, err)
}

func TestApp_UpdateKnowledgeExtractionSettings_rejectsOutOfRangeMaximum(t *testing.T) {
	// Given an App and an out-of-range maximum
	configs := configmocks.NewMockStore(t)
	configs.EXPECT().Load().Return(domainconfig.Config{OpenRouterKey: "sk-or-existing", MaxKnowledgeExtractionItems: 8}, nil).Once()
	app := NewApp(nil, nil, nil, nil, configs, nil, nil, nil, nil, nil)

	// When changing the setting to an invalid value
	err := app.UpdateKnowledgeExtractionSettings(21)

	// Then validation rejects it and Save is never called
	assert.ErrorIs(t, err, domainconfig.ErrMaxKnowledgeExtractionItemsOutOfRange)
}

func TestApp_UpdateKnowledgeExtractionSettings_rejectsFractionalMaximum(t *testing.T) {
	// Given an App and a fractional extraction maximum
	configs := configmocks.NewMockStore(t)
	app := NewApp(nil, nil, nil, nil, configs, nil, nil, nil, nil, nil)

	// When changing the setting to a fractional value
	err := app.UpdateKnowledgeExtractionSettings(1.5)

	// Then validation rejects it before loading or saving the configuration
	assert.ErrorIs(t, err, domainconfig.ErrMaxKnowledgeExtractionItemsOutOfRange)
	assert.EqualError(t, err, "maximum knowledge extraction items must be an integer between 1 and 20")
}

func TestApp_UpdateProfile_propagatesValidationError_whenGoalsIsMissing(t *testing.T) {
	// Given an App backed by a profile store with an existing profile, and a
	// store that must never be called to save
	profiles := profilemocks.NewMockStore(t)
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{CreatedAt: time.Now().UTC()}, nil).Once()
	app := newTestOnboardingApp(t, profiles, configmocks.NewMockStore(t), configmocks.NewMockKeyValidator(t))

	input := validProfileInput()
	input.Goals = nil

	// When updating the profile with no goals
	_, err := app.UpdateProfile(input)

	// Then the domain validation error is surfaced unchanged
	assert.ErrorIs(t, err, domainprofile.ErrGoalsRequired)
}

func TestApp_UpdateProfile_propagatesLoadError_whenNoExistingProfile(t *testing.T) {
	// Given an App backed by a profile store with no existing profile
	profiles := profilemocks.NewMockStore(t)
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{}, assert.AnError).Once()
	app := newTestOnboardingApp(t, profiles, configmocks.NewMockStore(t), configmocks.NewMockKeyValidator(t))

	// When updating the profile
	got, err := app.UpdateProfile(validProfileInput())

	// Then the load error is surfaced and no profile is returned
	assert.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, UserProfileInput{}, got)
}
