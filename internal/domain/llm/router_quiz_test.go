package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTierFor_quizIsMedium(t *testing.T) {
	// Given the Interactive Quiz task
	task := TaskQuiz

	// When its tier is looked up
	tier := TierFor(task)

	// Then it is medium and the task keeps its stable name
	assert.Equal(t, TierMedium, tier)
	assert.Equal(t, "quiz", string(task))
}
