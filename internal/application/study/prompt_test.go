package study

import (
	"testing"

	"github.com/stretchr/testify/assert"

	domainprofile "github.com/santaniello/athena/internal/domain/profile"
)

func TestBuildSystemPrompt_includesAllProfileFieldsAndTopic(t *testing.T) {
	// Given a fully filled profile and a topic
	profile := domainprofile.UserProfile{
		Name:            "Ana",
		AssistantName:   "Atena",
		Area:            "Engenharia de Software",
		ExperienceLevel: domainprofile.ExperienceLevelIntermediate,
		Goals:           []string{"SQL", "System Design"},
		StudyStyle:      domainprofile.StudyStylePracticalExamples,
	}

	// When building the system prompt
	prompt := buildSystemPrompt(profile, "Distributed systems")

	// Then it includes every profile field and the topic
	assert.Contains(t, prompt, "Atena")
	assert.Contains(t, prompt, "Ana")
	assert.Contains(t, prompt, "Engenharia de Software")
	assert.Contains(t, prompt, domainprofile.ExperienceLevelIntermediate)
	assert.Contains(t, prompt, domainprofile.StudyStylePracticalExamples)
	assert.Contains(t, prompt, "SQL, System Design")
	assert.Contains(t, prompt, "Distributed systems")
}

func TestBuildSystemPrompt_neverMentionsSpecialty(t *testing.T) {
	// Given any profile and topic (UserProfile has no Specialty field)
	profile := domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}

	// When building the system prompt
	prompt := buildSystemPrompt(profile, "Distributed systems")

	// Then it never references a {Specialty} placeholder
	assert.NotContains(t, prompt, "Specialty")
}
