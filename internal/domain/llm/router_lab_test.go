package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTierFor_architectureLabIsMedium(t *testing.T) {
	// Given the Architecture Lab task
	task := TaskArchitectureLab

	// When its tier is looked up
	tier := TierFor(task)

	// Then it is medium and the task keeps its stable name
	assert.Equal(t, TierMedium, tier)
	assert.Equal(t, "architecture_lab", string(task))
}
