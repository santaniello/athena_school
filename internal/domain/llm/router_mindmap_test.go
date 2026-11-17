package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTierFor_mindMapIsMedium(t *testing.T) {
	// Given the Mind Map task
	task := TaskMindMap

	// When its tier is looked up
	tier := TierFor(task)

	// Then it is medium and the task keeps its stable name
	assert.Equal(t, TierMedium, tier)
	assert.Equal(t, "mind_map", string(task))
}
