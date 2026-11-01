package exercise

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeTopic_trimsAndLowerCases(t *testing.T) {
	// Given a topic with surrounding whitespace and mixed case
	topic := "  Distributed SYSTEMS \n"

	// When normalizing it
	normalized := NormalizeTopic(topic)

	// Then it is trimmed and lower-cased
	assert.Equal(t, "distributed systems", normalized)
}

func TestPage_Normalize(t *testing.T) {
	tests := []struct {
		name string
		page Page
		want Page
	}{
		{"zero limit uses the default", Page{Limit: 0, Offset: 5}, Page{Limit: DefaultPageLimit, Offset: 5}},
		{"negative limit uses the default", Page{Limit: -3}, Page{Limit: DefaultPageLimit}},
		{"negative offset becomes zero", Page{Limit: 10, Offset: -1}, Page{Limit: 10}},
		{"limit above the maximum is capped", Page{Limit: MaxPageLimit + 1}, Page{Limit: MaxPageLimit}},
		{"limit at the maximum is kept", Page{Limit: MaxPageLimit}, Page{Limit: MaxPageLimit}},
		{"valid page is unchanged", Page{Limit: 20, Offset: 40}, Page{Limit: 20, Offset: 40}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a page request
			// When normalizing it
			got := tt.page.Normalize()

			// Then it is within the allowed bounds
			assert.Equal(t, tt.want, got)
		})
	}
}
