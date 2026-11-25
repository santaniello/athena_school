package progresssource

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
)

func TestProfiles_AssistantLanguage_returnsTheLanguageOfTheProfile(t *testing.T) {
	// Given a profile with a Portuguese assistant
	store := profilemocks.NewMockStore(t)
	store.EXPECT().Load().Return(domainprofile.UserProfile{AssistantLanguage: "pt"}, nil)

	// When the language is read
	language, err := NewProfiles(store).AssistantLanguage(context.Background())

	// Then it is the profile's
	require.NoError(t, err)
	assert.Equal(t, "pt", language)
}

func TestProfiles_AssistantLanguage_returnsTheErrorOfTheStore(t *testing.T) {
	// Given a profile that cannot be loaded
	store := profilemocks.NewMockStore(t)
	boom := errors.New("no profile")
	store.EXPECT().Load().Return(domainprofile.UserProfile{}, boom)

	// When the language is read
	_, err := NewProfiles(store).AssistantLanguage(context.Background())

	// Then the cause is returned
	require.ErrorIs(t, err, boom)
}
