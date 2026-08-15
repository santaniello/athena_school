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
	app := NewApp(nil, nil, onboarding.NewService(profiles, config, validator), profiles, config)
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
	const key = "sk-or-valid"
	validator.EXPECT().ValidateKey(context.Background(), key).Return(nil).Once()
	config.EXPECT().Save(domainconfig.Config{OpenRouterKey: key}).Return(nil).Once()
	app := newTestOnboardingApp(t, profilemocks.NewMockStore(t), config, validator)

	// When saving the key
	err := app.SaveOpenRouterKey(key)

	// Then it succeeds
	require.NoError(t, err)
}

func TestApp_SaveOpenRouterKey_propagatesInvalidKeyError(t *testing.T) {
	// Given an App backed by a validator that rejects the key
	validator := configmocks.NewMockKeyValidator(t)
	const key = "sk-or-invalid"
	validator.EXPECT().ValidateKey(context.Background(), key).Return(domainconfig.ErrKeyInvalid).Once()
	app := newTestOnboardingApp(t, profilemocks.NewMockStore(t), configmocks.NewMockStore(t), validator)

	// When saving the key
	err := app.SaveOpenRouterKey(key)

	// Then the invalid-key sentinel is surfaced unchanged
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
		Name:            "Ana",
		AssistantName:   "Atena",
		Area:            "Engenharia de Software",
		ExperienceLevel: domainprofile.ExperienceLevelIntermediate,
		Goals:           []string{"SQL", "System Design"},
		StudyStyle:      "Prática com exercícios",
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
