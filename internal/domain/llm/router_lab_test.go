package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTierFor_architectureLabIsMedium(t *testing.T) {
	assert.Equal(t, TierMedium, TierFor(TaskArchitectureLab))
	assert.Equal(t, "architecture_lab", string(TaskArchitectureLab))
}
