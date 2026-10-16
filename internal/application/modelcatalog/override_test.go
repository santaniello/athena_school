package modelcatalog

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/llm/mocks"
)

func TestParseContextWindowOverride(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{name: "unset is off", raw: "", want: 0},
		{name: "blank is off", raw: "   ", want: 0},
		{name: "zero is off", raw: "0", want: 0},
		{name: "off keyword is off", raw: "off", want: 0},
		{name: "off keyword ignores case and spaces", raw: " OFF ", want: 0},
		{name: "positive integer turns it on", raw: "20000", want: 20000},
		{name: "positive integer tolerates spaces", raw: " 20000 ", want: 20000},
		{name: "negative is rejected", raw: "-5", wantErr: true},
		{name: "non numeric is rejected", raw: "abc", wantErr: true},
		{name: "decimal is rejected", raw: "1.5", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given the raw value of the override setting

			// When parsing it
			got, err := ParseContextWindowOverride(tt.raw)

			// Then it yields the length (0 = off) or rejects an invalid value
			if tt.wantErr {
				require.Error(t, err)
				assert.Zero(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOverrideResolver_CachedContextLength_reportsOverrideForAnyModel(t *testing.T) {
	// Given an override of 20000 over an inner resolver that is never consulted
	inner := mocks.NewMockModelContextResolver(t)
	resolver := NewOverrideResolver(inner, 20000)

	// When looking up a model
	length, ok := resolver.CachedContextLength("anthropic/claude-sonnet-4.5")

	// Then the override is reported as a cache hit
	assert.True(t, ok)
	assert.Equal(t, 20000, length)
}

func TestOverrideResolver_CachedContextLength_blankModelDelegatesToInner(t *testing.T) {
	// Given an inner resolver that has no length for a blank model ID
	inner := mocks.NewMockModelContextResolver(t)
	inner.EXPECT().CachedContextLength("").Return(0, false).Once()
	resolver := NewOverrideResolver(inner, 20000)

	// When looking up the blank model
	length, ok := resolver.CachedContextLength("")

	// Then the override does not invent a length for an unresolved model
	assert.False(t, ok)
	assert.Zero(t, length)
}

func TestOverrideResolver_RefreshContextLength_returnsOverrideWithoutRefreshing(t *testing.T) {
	// Given an override over an inner resolver that must not be refreshed
	inner := mocks.NewMockModelContextResolver(t)
	resolver := NewOverrideResolver(inner, 20000)

	// When refreshing for a model
	length, err := resolver.RefreshContextLength(context.Background(), "anthropic/claude-sonnet-4.5")

	// Then the override is returned and no catalog request is made
	require.NoError(t, err)
	assert.Equal(t, 20000, length)
}

func TestOverrideResolver_RefreshContextLength_blankModelDelegatesToInner(t *testing.T) {
	// Given an inner resolver that fails for a blank model ID
	innerErr := errors.New("unknown model")
	inner := mocks.NewMockModelContextResolver(t)
	inner.EXPECT().RefreshContextLength(context.Background(), "").Return(0, innerErr).Once()
	resolver := NewOverrideResolver(inner, 20000)

	// When refreshing for the blank model
	length, err := resolver.RefreshContextLength(context.Background(), "")

	// Then the inner result is passed through untouched
	require.ErrorIs(t, err, innerErr)
	assert.Zero(t, length)
}
