package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTierFor_progressInsightIsMedium(t *testing.T) {
	// Given the Progress insight task
	task := TaskProgressInsight

	// When its tier is looked up
	tier := TierFor(task)

	// Then it is medium and the task keeps its stable name
	assert.Equal(t, TierMedium, tier)
	assert.Equal(t, "progress_insight", string(task))
}
