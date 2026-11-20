package study

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	domainprofile "github.com/santaniello/athena/internal/domain/profile"
)

func TestBuildSystemPrompt_includesAllProfileFieldsTopicAndSessionGoal(t *testing.T) {
	// Given a fully filled profile, a topic, and the session's own goal
	profile := domainprofile.UserProfile{
		Name:            "Ana",
		AssistantName:   "Atena",
		Area:            "Engenharia de Software",
		ExperienceLevel: domainprofile.ExperienceLevelIntermediate,
		StudyStyle:      domainprofile.StudyStylePracticalExamples,
	}

	// When building the system prompt
	prompt := buildSystemPrompt(profile, "Distributed systems", "Ace the SQL interview")

	// Then it includes every profile field, the topic, and the session goal
	assert.Contains(t, prompt, "Atena")
	assert.Contains(t, prompt, "Ana")
	assert.Contains(t, prompt, "Engenharia de Software")
	assert.Contains(t, prompt, domainprofile.ExperienceLevelIntermediate)
	assert.Contains(t, prompt, domainprofile.StudyStylePracticalExamples)
	assert.Contains(t, prompt, "Goal: Ace the SQL interview")
	assert.Contains(t, prompt, "Distributed systems")
}

func TestBuildSystemPrompt_omitsGoalLine_whenSessionGoalIsBlank(t *testing.T) {
	// Given a session predating the Goal field (e.g. created before this
	// feature shipped), so it has no goal of its own and there is no
	// profile-level fallback left to render
	profile := domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}

	// When building the system prompt
	prompt := buildSystemPrompt(profile, "Distributed systems", "")

	// Then it never mentions "Goal:" at all
	assert.NotContains(t, prompt, "Goal:")
}

func TestBuildSystemPrompt_neverMentionsSpecialty(t *testing.T) {
	// Given any profile and topic (UserProfile has no Specialty field)
	profile := domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}

	// When building the system prompt
	prompt := buildSystemPrompt(profile, "Distributed systems", "Ace the SQL interview")

	// Then it never references a {Specialty} placeholder
	assert.NotContains(t, prompt, "Specialty")
}

func adaptivePrompt() string {
	profile := domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}
	return buildSystemPrompt(profile, "Distributed systems", "Ace the SQL interview")
}

func TestBuildSystemPrompt_namesAndDescribesTheSixStrategies(t *testing.T) {
	prompt := adaptivePrompt()

	for _, rule := range []string{
		"You are an adaptive tutor",
		"DIRECT_INSTRUCTION: explain the idea clearly and briefly",
		"SOCRATIC_QUESTIONING: ask one investigative question",
		"GUIDED_PRACTICE: give a small task or example for the user to apply",
		"COMPREHENSION_CHECK: after an explanation, ask a short recall question",
		"CORRECTIVE_FEEDBACK: name the misconception and fix it",
		"CONCEPT_REVIEW: revisit a missing prerequisite",
	} {
		assert.Contains(t, prompt, rule)
	}
}

func TestBuildSystemPrompt_neverLetsTheModelWriteStrategyIdentifiers(t *testing.T) {
	prompt := adaptivePrompt()

	assert.Contains(t, prompt, "internal labels. Never write them")
	assert.Contains(t, prompt, "describe your approach in plain words")
}

func TestBuildSystemPrompt_explicitRequestsWinOverAsking(t *testing.T) {
	prompt := adaptivePrompt()

	assert.Contains(t, prompt, "explicitly asks for an explanation, a hint, more depth or a simpler version")
	assert.Contains(t, prompt, "do that first")
	assert.Contains(t, prompt, "never answer that request with a question")
}

func TestBuildSystemPrompt_carriesSelectionHeuristics(t *testing.T) {
	prompt := adaptivePrompt()

	for _, rule := range []string{
		"Unknown concept: explain it",
		"After a correct answer with a justification, deepen at the same level by changing the angle",
		"what would change if a condition changed",
		"Recurring difficulty: simplify, give a hint or split the problem into smaller steps",
		"Right after explaining a concept, ask a short comprehension check, only when it helps",
		"Missing prerequisite: review it first",
	} {
		assert.Contains(t, prompt, rule)
	}
}

func TestBuildSystemPrompt_carriesAntiLoopRules(t *testing.T) {
	prompt := adaptivePrompt()

	for _, rule := range []string{
		"Do not turn every reply into a question; ask at most one main question per reply",
		"Never repeat a question already asked",
		"last two answers on the same point were wrong or empty, explain or give a hint instead of asking again",
		"Raise the difficulty only after consistent correct answers on the same point, never after a single one",
	} {
		assert.Contains(t, prompt, rule)
	}
}

func TestBuildSystemPrompt_openingRule(t *testing.T) {
	prompt := adaptivePrompt()

	assert.Contains(t, prompt, "a single focused question")
	assert.Contains(t, prompt, "open instead with a short explanation")
	assert.Contains(t, prompt, "from scratch")
	assert.Contains(t, prompt, "or when in doubt, ask the question")
	assert.Contains(t, prompt, "Never lecture")
}

func TestBuildSystemPrompt_dropsTheRigidQuestioningRules(t *testing.T) {
	prompt := adaptivePrompt()

	assert.NotContains(t, prompt, "Wait for the user's answer before introducing any new material")
	assert.NotContains(t, prompt, "After each answer, give brief feedback, then ask a follow-up question")
	assert.NotContains(t, prompt, "Do not explain or teach anything yet")
}

func TestBuildSystemPrompt_instructsConciseFollowUps(t *testing.T) {
	// Given any profile and topic
	profile := domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}

	// When building the system prompt
	prompt := buildSystemPrompt(profile, "Distributed systems", "Ace the SQL interview")

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
	prompt := buildSystemPrompt(profile, "Distributed systems", "Ace the SQL interview")

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
	prompt := buildSystemPrompt(profile, "Distributed systems", "Ace the SQL interview")

	// Then it explicitly instructs the model to reply in English
	assert.Contains(t, prompt, "Respond in English")
}

func TestBuildSystemPrompt_omitsLanguageInstructionWhenUnset(t *testing.T) {
	// Given a profile without AssistantLanguage set (e.g. partial profile)
	profile := domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}

	// When building the system prompt
	prompt := buildSystemPrompt(profile, "Distributed systems", "Ace the SQL interview")

	// Then it adds no language instruction, preserving prior behavior
	assert.NotContains(t, prompt, "Respond in")
}

func TestBuildSystemPrompt_instructsToNeverTranslateTheUserName(t *testing.T) {
	// Given any user name or nickname, including ones that are common words
	// in another language (the nickname "Santa" was rendered as "Papai Noel")
	for _, name := range []string{"Santa", "Rei", "Gato", "Ana"} {
		t.Run(name, func(t *testing.T) {
			profile := domainprofile.UserProfile{
				Name:              name,
				AssistantName:     "Atena",
				AssistantLanguage: domainprofile.AssistantLanguagePortuguese,
			}

			// When building the system prompt
			prompt := buildSystemPrompt(profile, "Cache", "")

			// Then it tells the model the name is a proper noun to be used
			// verbatim, never translated
			assert.Contains(t, prompt, fmt.Sprintf("The user's name is %q", name))
			assert.Contains(t, prompt, "never translate")
		})
	}
}

func TestBuildSystemPrompt_asksForTheTutorStateBlock(t *testing.T) {
	prompt := adaptivePrompt()

	for _, rule := range []string{
		tutorStateOpenTag,
		tutorStateCloseTag,
		`{"concept":"Race Condition","status":"PARTIALLY_UNDERSTOOD","strategy":"CORRECTIVE_FEEDBACK","userEvidence":true,"assisted":true}`,
		"on its own last lines, after all the text, with nothing after it",
		"at most once per reply",
		"only when you actually worked on a concept in this reply",
		"NOT_ASSESSED, INTRODUCED, PARTIALLY_UNDERSTOOD, DEMONSTRATED or NEEDS_REVIEW",
		"one of the six strategy names above",
		"The user never sees this block",
	} {
		assert.Contains(t, prompt, rule)
	}
}

func TestBuildSystemPrompt_tutorStateEvidenceRule(t *testing.T) {
	prompt := adaptivePrompt()

	for _, rule := range []string{
		"Set userEvidence to true only when the user's own latest message shows the behavior behind the status",
		"Use DEMONSTRATED only when userEvidence is true, never for a concept you only explained",
		"A concept you explain is at most INTRODUCED",
		"Set assisted to true when you had to give a hint or explain to get there",
	} {
		assert.Contains(t, prompt, rule)
	}
}

func TestBuildSystemPrompt_tutorStateBlockIsTheOnlyPlaceForStrategyNames(t *testing.T) {
	prompt := adaptivePrompt()

	assert.Contains(t, prompt, "The strategy field of this block is the only place you may write a strategy name")
}

func TestBuildSystemPrompt_tutorStateInstructionComesAfterTheOfferRules(t *testing.T) {
	prompt := adaptivePrompt()

	assert.Greater(t, strings.Index(prompt, tutorStateOpenTag), strings.Index(prompt, mindMapOfferMarker))
}
