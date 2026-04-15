package study_test

import (
	"testing"

	"github.com/fsantaniello/athena_school/internal/study"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GivenTopic_WhenBuildExplanationPrompt_ThenContainsTopicAndAthena(t *testing.T) {
	// Given: a topic with no subtopic
	topic := "system-design"

	// When: building the explanation prompt
	result, err := study.BuildExplanationPrompt(topic, "")

	// Then: the prompt contains the topic and identifies as Athena
	require.NoError(t, err)
	assert.Contains(t, result, "system-design")
	assert.Contains(t, result, "Athena")
}

func Test_GivenTopicAndSubtopic_WhenBuildExplanationPrompt_ThenContainsCombinedLabel(t *testing.T) {
	// Given: a topic and a subtopic
	topic := "system-design"
	subtopic := "caching"

	// When: building the explanation prompt
	result, err := study.BuildExplanationPrompt(topic, subtopic)

	// Then: the prompt contains the combined "topic › subtopic" label
	require.NoError(t, err)
	assert.Contains(t, result, "system-design › caching")
}

func Test_GivenTopic_WhenBuildQuestionPrompt_ThenContainsTopicAndInstruction(t *testing.T) {
	// Given: a topic
	topic := "concurrency"

	// When: building the question prompt
	result, err := study.BuildQuestionPrompt(topic)

	// Then: the prompt contains the topic and the ask-only-question instruction
	require.NoError(t, err)
	assert.Contains(t, result, "concurrency")
	assert.Contains(t, result, "Ask only the question")
}

func Test_GivenTopicQuestionAnswer_WhenBuildEvaluationPrompt_ThenContainsAllFields(t *testing.T) {
	// Given: a topic, question, and answer
	topic := "caching"
	question := "What is LRU eviction?"
	answer := "Least Recently Used eviction removes the least recently accessed item"

	// When: building the evaluation prompt
	result, err := study.BuildEvaluationPrompt(topic, question, answer)

	// Then: the prompt contains all three values plus the evaluation format markers
	require.NoError(t, err)
	assert.Contains(t, result, "caching")
	assert.Contains(t, result, "What is LRU eviction?")
	assert.Contains(t, result, "Least Recently Used eviction removes the least recently accessed item")
	assert.Contains(t, result, "Strengths")
	assert.Contains(t, result, "Score")
}
