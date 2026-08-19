package desktop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/application/onboarding"
	domainconfig "github.com/santaniello/athena/internal/domain/config"
	configmocks "github.com/santaniello/athena/internal/domain/config/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
)

func newTestOnboardingApp(
	t *testing.T,
	profiles domainprofile.Store,
	config domainconfig.Store,
	validator domainconfig.KeyValidator,
) *App {
	t.Helper()
	return newTestOnboardingAppWithKeyUpdater(t, profiles, config, validator, llmmocks.NewMockAPIKeyUpdater(t))
}

func newTestOnboardingAppWithKeyUpdater(
	t *testing.T,
	profiles domainprofile.Store,
	config domainconfig.Store,
	validator domainconfig.KeyValidator,
	apiKeyUpdater domainllm.APIKeyUpdater,
) *App {
	t.Helper()
	app := NewApp(nil, nil, onboarding.NewService(profiles, config, validator), profiles, config, nil, nil, nil, nil, apiKeyUpdater)
	app.Startup(context.Background())
	return app
}

func TestApp_HasOpenRouterKey_returnsTrue_whenConfigHasKey(t *testing.T) {
	// Given an App backed by a config store with a saved key
	config := configmocks.NewMockStore(t)
	config.EXPECT().Load().Return(domainconfig.Config{OpenRouterKey: "sk-or-abc"}, nil).Once()
	app := newTestOnboardingApp(t, profilemocks.NewMockStore(t), config, configmocks.NewMockKeyValidator(t))

	// When checking whether an OpenRouter key is configured
	has := app.HasOpenRouterKey()

	// Then it reports true
	assert.True(t, has)
}

func TestApp_HasOpenRouterKey_returnsFalse_whenConfigHasNoKey(t *testing.T) {
	// Given an App backed by a config store with no saved config
	config := configmocks.NewMockStore(t)
	config.EXPECT().Load().Return(domainconfig.Config{}, assert.AnError).Once()
	app := newTestOnboardingApp(t, profilemocks.NewMockStore(t), config, configmocks.NewMockKeyValidator(t))

	// When checking whether an OpenRouter key is configured
	has := app.HasOpenRouterKey()

	// Then it reports false
	assert.False(t, has)
}

func TestApp_SaveOpenRouterKey_savesKey_whenValid(t *testing.T) {
	// Given an App backed by a validator that accepts the key
	config := configmocks.NewMockStore(t)
	validator := configmocks.NewMockKeyValidator(t)
	apiKeyUpdater := llmmocks.NewMockAPIKeyUpdater(t)
	const key = "sk-or-valid"
	validator.EXPECT().ValidateKey(context.Background(), key).Return(nil).Once()
	config.EXPECT().Load().Return(domainconfig.Config{MaxKnowledgeExtractionItems: 8}, nil).Once()
	config.EXPECT().Save(domainconfig.Config{OpenRouterKey: key, MaxKnowledgeExtractionItems: 8}).Return(nil).Once()
	apiKeyUpdater.EXPECT().SetAPIKey(key).Once()
	app := newTestOnboardingAppWithKeyUpdater(t, profilemocks.NewMockStore(t), config, validator, apiKeyUpdater)

	// When saving the key
	err := app.SaveOpenRouterKey(key)

	// Then it succeeds and the live client picks up the new key immediately
	require.NoError(t, err)
}

func TestApp_SaveOpenRouterKey_propagatesInvalidKeyError(t *testing.T) {
	// Given an App backed by a validator that rejects the key, and an
	// APIKeyUpdater that must never be called
	validator := configmocks.NewMockKeyValidator(t)
	apiKeyUpdater := llmmocks.NewMockAPIKeyUpdater(t)
	const key = "sk-or-invalid"
	validator.EXPECT().ValidateKey(context.Background(), key).Return(domainconfig.ErrKeyInvalid).Once()
	app := newTestOnboardingAppWithKeyUpdater(t, profilemocks.NewMockStore(t), configmocks.NewMockStore(t), validator, apiKeyUpdater)

	// When saving the key
	err := app.SaveOpenRouterKey(key)

	// Then the invalid-key sentinel is surfaced unchanged, and the live
	// client is left untouched
	assert.ErrorIs(t, err, domainconfig.ErrKeyInvalid)
}

func TestApp_HasUserProfile_returnsTrue_whenProfileExists(t *testing.T) {
	// Given an App backed by a profile store with a saved profile
	profiles := profilemocks.NewMockStore(t)
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana"}, nil).Once()
	app := newTestOnboardingApp(t, profiles, configmocks.NewMockStore(t), configmocks.NewMockKeyValidator(t))

	// When checking whether onboarding was already completed
	has := app.HasUserProfile()

	// Then it reports true
	assert.True(t, has)
}

func TestApp_HasUserProfile_returnsFalse_whenNoProfileExists(t *testing.T) {
	// Given an App backed by a profile store with no saved profile
	profiles := profilemocks.NewMockStore(t)
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{}, assert.AnError).Once()
	app := newTestOnboardingApp(t, profiles, configmocks.NewMockStore(t), configmocks.NewMockKeyValidator(t))

	// When checking whether onboarding was already completed
	has := app.HasUserProfile()

	// Then it reports false
	assert.False(t, has)
}

func mockMatchesProfileNamed(name string) interface{} {
	return mock.MatchedBy(func(p domainprofile.UserProfile) bool {
		return p.Name == name
	})
}

func validProfileInput() UserProfileInput {
	return UserProfileInput{
		Name:              "Ana",
		AssistantName:     "Atena",
		Area:              "Engenharia de Software",
		ExperienceLevel:   domainprofile.ExperienceLevelIntermediate,
		Goals:             []string{"SQL", "System Design"},
		StudyStyle:        domainprofile.StudyStylePracticalExamples,
		AssistantLanguage: domainprofile.AssistantLanguageEnglish,
	}
}

func TestApp_SaveProfile_savesProfile_whenValid(t *testing.T) {
	// Given an App backed by a profile store that accepts the save
	profiles := profilemocks.NewMockStore(t)
	profiles.EXPECT().Save(mockMatchesProfileNamed("Ana")).Return(nil).Once()
	app := newTestOnboardingApp(t, profiles, configmocks.NewMockStore(t), configmocks.NewMockKeyValidator(t))

	// When saving a valid profile
	err := app.SaveProfile(validProfileInput())

	// Then it succeeds
	require.NoError(t, err)
}

func TestApp_GetProfile_returnsProfile_whenItExists(t *testing.T) {
	// Given an App backed by a profile store with a saved profile
	profiles := profilemocks.NewMockStore(t)
	input := validProfileInput()
	saved := domainprofile.UserProfile{
		Name:              input.Name,
		AssistantName:     input.AssistantName,
		Area:              input.Area,
		ExperienceLevel:   input.ExperienceLevel,
		Goals:             input.Goals,
		StudyStyle:        input.StudyStyle,
		AssistantLanguage: input.AssistantLanguage,
	}
	profiles.EXPECT().Load().Return(saved, nil).Once()
	app := newTestOnboardingApp(t, profiles, configmocks.NewMockStore(t), configmocks.NewMockKeyValidator(t))

	// When reading the profile back
	got, err := app.GetProfile()

	// Then it returns the saved fields
	require.NoError(t, err)
	assert.Equal(t, input, got)
}

func TestApp_GetProfile_propagatesLoadError_whenNoProfileExists(t *testing.T) {
	// Given an App backed by a profile store with no saved profile
	profiles := profilemocks.NewMockStore(t)
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{}, assert.AnError).Once()
	app := newTestOnboardingApp(t, profiles, configmocks.NewMockStore(t), configmocks.NewMockKeyValidator(t))

	// When reading the profile back
	got, err := app.GetProfile()

	// Then the error is surfaced unchanged and no profile is returned
	assert.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, UserProfileInput{}, got)
}

func TestApp_SaveProfile_propagatesValidationError_whenGoalsIsMissing(t *testing.T) {
	// Given an App backed by a profile store that must never be called
	profiles := profilemocks.NewMockStore(t)
	app := newTestOnboardingApp(t, profiles, configmocks.NewMockStore(t), configmocks.NewMockKeyValidator(t))
	input := validProfileInput()
	input.Goals = nil

	// When saving a profile with no goals
	err := app.SaveProfile(input)

	// Then the domain validation error is surfaced unchanged
	assert.ErrorIs(t, err, domainprofile.ErrGoalsRequired)
}
