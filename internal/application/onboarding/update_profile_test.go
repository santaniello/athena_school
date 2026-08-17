package onboarding

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	"github.com/santaniello/athena/internal/domain/profile/mocks"
)

func TestUpdateProfile_preservesOriginalCreatedAt_whenProfileIsValid(t *testing.T) {
	// Given an existing profile with a fixed CreatedAt and a valid edit to it
	store := mocks.NewMockStore(t)
	originalCreatedAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	existing := validInput()
	existing.CreatedAt = originalCreatedAt

	edit := validInput()
	edit.AssistantName = "Nova Atena"

	store.EXPECT().Load().Return(existing, nil).Once()
	store.EXPECT().
		Save(mock.MatchedBy(func(p domainprofile.UserProfile) bool {
			return p.AssistantName == "Nova Atena" && p.CreatedAt.Equal(originalCreatedAt)
		})).
		Return(nil).
		Once()

	service := NewService(store, nil, nil)

	// When updating the profile
	saved, err := service.UpdateProfile(edit)

	// Then it succeeds and keeps the original CreatedAt, not a fresh one
	require.NoError(t, err)
	assert.True(t, saved.CreatedAt.Equal(originalCreatedAt))
}

func TestUpdateProfile_returnsValidationError_andNeverSaves_whenProfileIsInvalid(t *testing.T) {
	// Given an existing profile and an invalid edit (no goals)
	store := mocks.NewMockStore(t)
	existing := validInput()
	existing.CreatedAt = time.Now().UTC()

	edit := validInput()
	edit.Goals = nil

	store.EXPECT().Load().Return(existing, nil).Once()

	service := NewService(store, nil, nil)

	// When updating the profile
	_, err := service.UpdateProfile(edit)

	// Then it fails with the domain validation error, without saving
	assert.ErrorIs(t, err, domainprofile.ErrGoalsRequired)
}

func TestUpdateProfile_propagatesLoadError_whenNoExistingProfile(t *testing.T) {
	// Given a profile store with no existing profile to load
	store := mocks.NewMockStore(t)
	loadErr := errors.New("no such file")
	store.EXPECT().Load().Return(domainprofile.UserProfile{}, loadErr).Once()

	service := NewService(store, nil, nil)

	// When updating the profile
	_, err := service.UpdateProfile(validInput())

	// Then the load error is surfaced, without attempting to save
	assert.ErrorIs(t, err, loadErr)
}

func TestUpdateProfile_propagatesStoreError_onSave(t *testing.T) {
	// Given an existing profile and a store that fails to save the edit
	store := mocks.NewMockStore(t)
	existing := validInput()
	existing.CreatedAt = time.Now().UTC()
	saveErr := errors.New("disk full")

	store.EXPECT().Load().Return(existing, nil).Once()
	store.EXPECT().Save(mock.AnythingOfType("profile.UserProfile")).Return(saveErr).Once()

	service := NewService(store, nil, nil)

	// When updating the profile
	_, err := service.UpdateProfile(validInput())

	// Then the store error is surfaced
	assert.ErrorIs(t, err, saveErr)
}
