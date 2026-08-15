package openrouter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/config"
)

func TestValidator_ValidateKey_returnsNil_whenOpenRouterAcceptsTheKey(t *testing.T) {
	// Given a fake OpenRouter that accepts any bearer token and echoes it back
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	validator := NewValidator(server.URL)

	// When validating a key
	err := validator.ValidateKey(context.Background(), "sk-or-valid")

	// Then it succeeds and the key was sent as a bearer token
	require.NoError(t, err)
	assert.Equal(t, "Bearer sk-or-valid", receivedAuth)
}

func TestValidator_ValidateKey_returnsErrKeyInvalid_whenOpenRouterRejectsTheKey(t *testing.T) {
	// Given a fake OpenRouter that rejects the key
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	validator := NewValidator(server.URL)

	// When validating the key
	err := validator.ValidateKey(context.Background(), "sk-or-invalid")

	// Then it fails with the invalid-key sentinel
	assert.ErrorIs(t, err, config.ErrKeyInvalid)
}

func TestValidator_ValidateKey_returnsGenericError_whenOpenRouterFails(t *testing.T) {
	// Given a fake OpenRouter that errors for an unrelated reason
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	validator := NewValidator(server.URL)

	// When validating a key
	err := validator.ValidateKey(context.Background(), "sk-or-whatever")

	// Then it fails, but not with the invalid-key sentinel, so callers can
	// tell "your key is wrong" apart from "OpenRouter is unreachable"
	assert.Error(t, err)
	assert.NotErrorIs(t, err, config.ErrKeyInvalid)
}

func TestValidator_ValidateKey_returnsError_whenHostIsUnreachable(t *testing.T) {
	// Given a validator pointed at a server that is not listening
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	unreachableURL := server.URL
	server.Close()
	validator := NewValidator(unreachableURL)

	// When validating a key
	err := validator.ValidateKey(context.Background(), "sk-or-whatever")

	// Then it fails, but not with the invalid-key sentinel
	assert.Error(t, err)
	assert.NotErrorIs(t, err, config.ErrKeyInvalid)
}

func TestValidator_ValidateKey_returnsError_whenContextIsAlreadyCanceled(t *testing.T) {
	// Given a fake OpenRouter and an already-canceled context
	requestReachedServer := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestReachedServer = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	validator := NewValidator(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When validating a key
	err := validator.ValidateKey(ctx, "sk-or-whatever")

	// Then it fails without ever reaching the server
	assert.Error(t, err)
	assert.False(t, requestReachedServer)
}
