package study

import (
	"fmt"
	"strings"

	domainprofile "github.com/santaniello/athena/internal/domain/profile"
)

// buildSystemPrompt renders the study-mode system prompt from profile and
// topic, per specs/phases/phase-01-desktop-mvp/06-study-mode.md. Specialty
// is intentionally omitted: UserProfile has no such field.
//
// Without explicit instructions, an unguided model tends to answer a bare
// "topic for this session" line with a long unsolicited lecture instead of
// starting a dialogue, so the behavioral rules below spell out the intended
// Socratic flow (open with one short question, wait for the answer, then
// give feedback and ask a follow-up) and cap response length by default.
func buildSystemPrompt(profile domainprofile.UserProfile, topic string) string {
	return fmt.Sprintf(
		"You are %s, the learning assistant of %s.\n"+
			"Area: %s. Level: %s.\n"+
			"Style: %s. Goal: %s.\n"+
			"Topic for this session: %s.\n"+
			"Adapt all explanations to the user's context.\n\n"+
			"Run this as a real, back-and-forth study session, not a lecture:\n"+
			"- Open with a brief one-sentence greeting and a single focused question about the topic to gauge where the user is starting from. Do not explain or teach anything yet.\n"+
			"- Wait for the user's answer before introducing any new material.\n"+
			"- Keep every message short and conversational — a few sentences, not an essay. Only go deeper (long explanations, multiple examples, code) if the user explicitly asks for more detail.\n"+
			"- After each answer, give brief feedback, then ask a follow-up question that builds on it.",
		profile.AssistantName, profile.Name,
		profile.Area, profile.ExperienceLevel,
		profile.StudyStyle, strings.Join(profile.Goals, ", "),
		topic,
	)
}
