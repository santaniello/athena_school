package study

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConversationSummary_returnsSummary_whenInputIsValid(t *testing.T) {
	// Given
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	// When
	summary, err := NewConversationSummary("s1", "## Progress\n- pointers", "m9", 42, now)

	// Then
	require.NoError(t, err)
	assert.Equal(t, ConversationSummary{
		SessionID:            "s1",
		Content:              "## Progress\n- pointers",
		CoversUntilMessageID: "m9",
		TokenEstimate:        42,
		UpdatedAt:            now,
	}, summary)
}

func TestNewConversationSummary_acceptsZeroTokenEstimate(t *testing.T) {
	// When
	_, err := NewConversationSummary("s1", "x", "m1", 0, time.Time{})

	// Then
	require.NoError(t, err)
}

func TestNewConversationSummary_returnsError_whenInputIsInvalid(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		content   string
		coversID  string
		estimate  int
	}{
		{"empty session id", "", "x", "m1", 1},
		{"empty covers id", "s1", "x", "", 1},
		{"empty content", "s1", "", "m1", 1},
		{"blank content", "s1", " \n\t ", "m1", 1},
		{"negative estimate", "s1", "x", "m1", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			summary, err := NewConversationSummary(tt.sessionID, tt.content, tt.coversID, tt.estimate, time.Time{})

			// Then
			assert.ErrorIs(t, err, ErrInvalidConversationSummary)
			assert.Equal(t, ConversationSummary{}, summary)
		})
	}
}

func TestSummaryTargetTokens_isClampedFractionOfWindow(t *testing.T) {
	tests := []struct {
		name   string
		window int
		want   int
	}{
		{"below floor", 5_000, SummaryMinTargetTokens},
		{"within range", 20_000, 2_000},
		{"above ceiling", 128_000, SummaryMaxTargetTokens},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SummaryTargetTokens(tt.window))
		})
	}
}

func TestSummaryHardCapTokens_addsTolerance(t *testing.T) {
	// 4000 target + 25% tolerance
	assert.Equal(t, 5_000, SummaryHardCapTokens(4_000))
}
