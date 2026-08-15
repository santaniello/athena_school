package onboarding

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainconfig "github.com/santaniello/athena/internal/domain/config"
	"github.com/santaniello/athena/internal/domain/config/mocks"
)

func TestSaveOpenRouterKey_savesConfig_whenKeyIsValid(t *testing.T) {
	// Given a validator that accepts the key and a config store
	validator := mocks.NewMockKeyValidator(t)
	store := mocks.NewMockStore(t)
	ctx := context.Background()
	const key = "sk-or-valid"

	validator.EXPECT().ValidateKey(ctx, key).Return(nil).Once()
	store.EXPECT().Save(domainconfig.Config{OpenRouterKey: key}).Return(nil).Once()

	service := NewService(nil, store, validator)

	// When saving the key
	err := service.SaveOpenRouterKey(ctx, key)

	// Then it succeeds
	require.NoError(t, err)
}

func TestSaveOpenRouterKey_returnsErrOpenRouterKeyRequired_whenKeyIsBlank(t *testing.T) {
	// Given a validator and a config store that must never be called
	validator := mocks.NewMockKeyValidator(t)
	store := mocks.NewMockStore(t)
	service := NewService(nil, store, validator)

	// When saving a blank key
	err := service.SaveOpenRouterKey(context.Background(), "   ")

	// Then it fails with the key-required error, without validating or saving
	assert.ErrorIs(t, err, ErrOpenRouterKeyRequired)
}

func TestSaveOpenRouterKey_propagatesErrKeyInvalid_andNeverSaves(t *testing.T) {
	// Given a validator that rejects the key
	validator := mocks.NewMockKeyValidator(t)
	store := mocks.NewMockStore(t)
	ctx := context.Background()
	const key = "sk-or-invalid"

	validator.EXPECT().ValidateKey(ctx, key).Return(domainconfig.ErrKeyInvalid).Once()

	service := NewService(nil, store, validator)

	// When saving the key
	err := service.SaveOpenRouterKey(ctx, key)

	// Then the invalid-key sentinel is propagated and the config is never saved
	assert.ErrorIs(t, err, domainconfig.ErrKeyInvalid)
}

func TestSaveOpenRouterKey_propagatesUnexpectedValidatorError(t *testing.T) {
	// Given a validator that fails for a reason unrelated to key validity
	validator := mocks.NewMockKeyValidator(t)
	store := mocks.NewMockStore(t)
	ctx := context.Background()
	const key = "sk-or-whatever"
	networkErr := errors.New("openrouter unreachable")

	validator.EXPECT().ValidateKey(ctx, key).Return(networkErr).Once()

	service := NewService(nil, store, validator)

	// When saving the key
	err := service.SaveOpenRouterKey(ctx, key)

	// Then the unexpected error is surfaced, not masked as an invalid key
	assert.ErrorIs(t, err, networkErr)
	assert.NotErrorIs(t, err, domainconfig.ErrKeyInvalid)
}
