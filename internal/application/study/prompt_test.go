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

func TestBuildSystemPrompt_instructsAShortSocraticOpening(t *testing.T) {
	// Given any profile and topic
	profile := domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}

	// When building the system prompt
	prompt := buildSystemPrompt(profile, "Distributed systems")

	// Then it explicitly tells the model to open with one short question,
	// not a lecture — without this instruction, an unguided model tends to
	// dump a long unsolicited explanation instead of starting a dialogue
	assert.Contains(t, prompt, "a single focused question")
	assert.Contains(t, prompt, "Do not explain or teach anything yet")
}

func TestBuildSystemPrompt_instructsConciseFollowUps(t *testing.T) {
	// Given any profile and topic
	profile := domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}

	// When building the system prompt
	prompt := buildSystemPrompt(profile, "Distributed systems")

	// Then it tells the model to keep every message short unless the user
	// explicitly asks for more depth
	assert.Contains(t, prompt, "Keep every message short")
	assert.Contains(t, prompt, "Only go deeper")
	assert.Contains(t, prompt, "explicitly asks")
}

func TestBuildSystemPrompt_instructsPortugueseWhenProfileWantsPortuguese(t *testing.T) {
	// Given a profile with AssistantLanguage set to Portuguese
	profile := domainprofile.UserProfile{
		Name:              "Ana",
		AssistantName:     "Atena",
		AssistantLanguage: domainprofile.AssistantLanguagePortuguese,
	}

	// When building the system prompt
	prompt := buildSystemPrompt(profile, "Distributed systems")

	// Then it explicitly instructs the model to reply in Portuguese,
	// including the opening message
	assert.Contains(t, prompt, "Respond in Brazilian Portuguese")
	assert.Contains(t, prompt, "opening message")
}

func TestBuildSystemPrompt_instructsEnglishWhenProfileWantsEnglish(t *testing.T) {
	// Given a profile with AssistantLanguage set to English
	profile := domainprofile.UserProfile{
		Name:              "Ana",
		AssistantName:     "Atena",
		AssistantLanguage: domainprofile.AssistantLanguageEnglish,
	}

	// When building the system prompt
	prompt := buildSystemPrompt(profile, "Distributed systems")

	// Then it explicitly instructs the model to reply in English
	assert.Contains(t, prompt, "Respond in English")
}

func TestBuildSystemPrompt_omitsLanguageInstructionWhenUnset(t *testing.T) {
	// Given a profile without AssistantLanguage set (e.g. partial profile)
	profile := domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}

	// When building the system prompt
	prompt := buildSystemPrompt(profile, "Distributed systems")

	// Then it adds no language instruction, preserving prior behavior
	assert.NotContains(t, prompt, "Respond in")
}
