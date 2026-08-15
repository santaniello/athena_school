package onboarding

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	"github.com/santaniello/athena/internal/domain/profile/mocks"
)

func validInput() domainprofile.UserProfile {
	return domainprofile.UserProfile{
		Name:            "Ana",
		AssistantName:   "Atena",
		Area:            "Engenharia de Software",
		Specialty:       "Backend",
		ExperienceLevel: domainprofile.ExperienceLevelIntermediate,
		Goals:           []string{"SQL", "System Design"},
		StudyStyle:      "Prática com exercícios",
	}
}

func TestSaveProfile_savesProfileWithCreatedAt_whenProfileIsValid(t *testing.T) {
	// Given a profile store and a valid profile with no CreatedAt set
	store := mocks.NewMockStore(t)
	input := validInput()

	store.EXPECT().
		Save(mock.MatchedBy(func(p domainprofile.UserProfile) bool {
			return p.Name == input.Name && !p.CreatedAt.IsZero()
		})).
		Return(nil).
		Once()

	service := NewService(store, nil, nil)

	// When saving the profile
	saved, err := service.SaveProfile(input)

	// Then it succeeds, stamps CreatedAt, and returns the saved profile
	require.NoError(t, err)
	assert.False(t, saved.CreatedAt.IsZero())
}

func TestSaveProfile_returnsValidationError_andNeverSaves_whenProfileIsInvalid(t *testing.T) {
	// Given a profile store that must never be called and an invalid profile (no goals)
	store := mocks.NewMockStore(t)
	input := validInput()
	input.Goals = nil

	service := NewService(store, nil, nil)

	// When saving the profile
	_, err := service.SaveProfile(input)

	// Then it fails with the domain validation error, without saving
	assert.ErrorIs(t, err, domainprofile.ErrGoalsRequired)
}

func TestSaveProfile_propagatesStoreError(t *testing.T) {
	// Given a profile store that fails to save
	store := mocks.NewMockStore(t)
	input := validInput()
	storeErr := errors.New("disk full")

	store.EXPECT().
		Save(mock.MatchedBy(func(p domainprofile.UserProfile) bool { return p.Name == input.Name })).
		Return(storeErr).
		Once()

	service := NewService(store, nil, nil)

	// When saving the profile
	_, err := service.SaveProfile(input)

	// Then the store error is surfaced
	assert.ErrorIs(t, err, storeErr)
}
