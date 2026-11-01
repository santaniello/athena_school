package exercise

import (
	"fmt"
	"strings"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// generationTask routes the generation call to the Architecture Lab model.
const generationTask = domainllm.TaskArchitectureLab

const generationSystemPrompt = `You write system-design exam challenges for a learner, based on what the learner studied.

Write one challenge and the rubric criteria it is graded on.

Rules:
- Do not include a solution, an architecture, or the names of the components the learner should choose. State the problem, the load and the requirements only.
- The scenario describes a concrete product, its users and its scale.
- Requirements, constraints and objectives are short, testable sentences.
- Write 1 to 6 criteria specific to this challenge. Each criterion has a relative weight (a positive whole number), a mandatory flag, and exactly 5 levels, from level 0 (absent) to level 4 (excellent), each describing what that level looks like.
- Write every text in %s.
- Difficulty: %s.

Answer with only the JSON object below: no Markdown, no code fence, no comments.
{
  "title": "string",
  "scenario": "string",
  "functionalRequirements": ["string"],
  "nonFunctionalRequirements": ["string"],
  "constraints": ["string"],
  "objectives": ["string"],
  "criteria": [
    {
      "name": "string",
      "description": "string",
      "weight": 10,
      "mandatory": false,
      "levels": ["level 0", "level 1", "level 2", "level 3", "level 4"]
    }
  ]
}`

// buildGenerationPrompt returns the messages that ask the model for a
// challenge. It is a pure function of the context, so its content is unit
// tested.
func buildGenerationPrompt(gc generationContext, difficulty domainexercise.Difficulty) []domainllm.Message {
	return []domainllm.Message{
		{Role: "system", Content: fmt.Sprintf(generationSystemPrompt, languageName(gc.Profile.Language), difficulty)},
		{Role: "user", Content: buildGenerationUserPrompt(gc)},
	}
}

func languageName(code string) string {
	if code == "pt" {
		return "Brazilian Portuguese"
	}
	return "English"
}

// buildGenerationUserPrompt lists the context in a fixed order, skipping what
// is missing: topic, goal, summary, profile, notes.
func buildGenerationUserPrompt(gc generationContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Topic: %s\n", gc.Topic)
	if gc.Goal != "" {
		fmt.Fprintf(&b, "\nGoal: %s\n", gc.Goal)
	}
	if gc.Summary != "" {
		fmt.Fprintf(&b, "\nConversation summary:\n%s\n", gc.Summary)
	}
	fmt.Fprintf(&b, "\nLearner: focus area %s, experience level %s\n", gc.Profile.Area, gc.Profile.ExperienceLevel)
	if len(gc.Notes) > 0 {
		b.WriteString("\nNotes:\n")
		for _, note := range gc.Notes {
			fmt.Fprintf(&b, "[%s]\n%s\n", note.Source, note.Content)
		}
	}
	return b.String()
}
