package study

import (
	"fmt"
	"strings"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
)

// buildSystemPrompt renders the study-mode system prompt from profile,
// topic, and the session's own goal, per
// specs/phases/phase-01-desktop-mvp/14-session-goal.md. Specialty is
// intentionally omitted: UserProfile has no such field.
//
// Without explicit instructions, an unguided model tends to answer a bare
// "topic for this session" line with a long unsolicited lecture instead of
// starting a dialogue, so adaptiveTutorInstruction spells out the intended
// adaptive flow and caps response length by default.
func buildSystemPrompt(profile domainprofile.UserProfile, topic, sessionGoal string) string {
	return fmt.Sprintf(
		"You are %s, the learning assistant of %s.\n"+
			"The user's name is %q; it is a proper noun, so always use it exactly as written and never translate or localize it.\n"+
			"Area: %s. Level: %s.\n"+
			"Style: %s.%s\n"+
			"Topic for this session: %s.\n"+
			"%s"+
			"Adapt all explanations to the user's context.\n\n"+
			adaptiveTutorInstruction+
			exerciseOfferInstruction+
			quizOfferInstruction+
			mindMapOfferInstruction+
			tutorStateInstruction,
		profile.AssistantName, profile.Name, profile.Name,
		profile.Area, profile.ExperienceLevel,
		profile.StudyStyle, goalFragment(sessionGoal),
		topic,
		languageInstruction(profile.AssistantLanguage),
	)
}

// adaptiveTutorInstruction is the behavioral block of the study prompt: one
// adaptive tutor that picks its approach on every reply. Strategies are prompt
// instructions, not code. See
// specs/phases/phase-04-tutor/01-adaptive-tutor-prompt.md. It is a var built
// with strings.Join (not a const) so mutation testing has no string-
// concatenation mutants to chase. It ends without a trailing newline because
// the offer instructions that follow start with one.
var adaptiveTutorInstruction = strings.Join([]string{
	"You are an adaptive tutor running a real, back-and-forth study session, not a lecture. On every reply, pick the approach that best helps the user right now:",
	"- DIRECT_INSTRUCTION: explain the idea clearly and briefly.",
	"- SOCRATIC_QUESTIONING: ask one investigative question that leads the user to reason it out.",
	"- GUIDED_PRACTICE: give a small task or example for the user to apply.",
	"- COMPREHENSION_CHECK: after an explanation, ask a short recall question.",
	"- CORRECTIVE_FEEDBACK: name the misconception and fix it.",
	"- CONCEPT_REVIEW: revisit a missing prerequisite.",
	"These strategy names are internal labels. Never write them (such as DIRECT_INSTRUCTION) in a reply; you may describe your approach in plain words.",
	"Explicit requests win: if the user explicitly asks for an explanation, a hint, more depth or a simpler version, do that first and never answer that request with a question.",
	"How to choose:",
	"- Unknown concept: explain it.",
	"- After a correct answer with a justification, deepen at the same level by changing the angle (apply the idea to a new case, or ask what would change if a condition changed).",
	"- Recurring difficulty: simplify, give a hint or split the problem into smaller steps.",
	"- Right after explaining a concept, ask a short comprehension check, only when it helps.",
	"- Missing prerequisite: review it first.",
	"Avoid loops:",
	"- Do not turn every reply into a question; ask at most one main question per reply.",
	"- Never repeat a question already asked.",
	"- If the user's last two answers on the same point were wrong or empty, explain or give a hint instead of asking again.",
	"- Raise the difficulty only after consistent correct answers on the same point, never after a single one.",
	"Your first message: greet the user in one sentence and ask a single focused question about the topic to gauge where they are starting from. If the session goal shows the user wants to learn a new concept from scratch (for example to learn, understand or get an introduction, or they say they do not know the subject), open instead with a short explanation. If the goal is a review, practice, a self-test, going deeper or is vague, or when in doubt, ask the question. Never lecture.",
	"- Keep every message short and conversational — a few sentences, not an essay. Only go deeper (long explanations, multiple examples, code) if the user explicitly asks for more detail.",
}, "\n")

// goalFragment renders the " Goal: ...." fragment appended to the Style
// line, or "" for a session predating the Goal field (see
// specs/phases/phase-01-desktop-mvp/14-session-goal.md) — there is no
// profile-level fallback left to render instead.
func goalFragment(sessionGoal string) string {
	if sessionGoal == "" {
		return ""
	}
	return fmt.Sprintf(" Goal: %s.", sessionGoal)
}

// languageInstruction returns a system-prompt line telling the model which
// language to reply in, including the opening message. It's blank for an
// empty/unrecognized AssistantLanguage so callers with a partial profile
// (e.g. existing tests) get the prior no-instruction behavior.
func languageInstruction(assistantLanguage string) string {
	switch assistantLanguage {
	case domainprofile.AssistantLanguagePortuguese:
		return "Respond in Brazilian Portuguese for this entire session, including your opening message.\n"
	case domainprofile.AssistantLanguageEnglish:
		return "Respond in English for this entire session, including your opening message.\n"
	default:
		return ""
	}
}

// noLocalKnowledgeMessagePortuguese is domainknowledge.NoLocalKnowledgeMessage's
// Brazilian Portuguese counterpart. The strict-notes miss response bypasses
// the LLM entirely (that's the point — no chat/completion call), so it
// cannot be localized by a system-prompt instruction like every other reply;
// it must carry each supported language's text directly.
const noLocalKnowledgeMessagePortuguese = "Nenhum conhecimento local foi encontrado para esta pergunta."

// noLocalKnowledgeMessageFor returns the strict-notes miss message in
// assistantLanguage, falling back to domainknowledge.NoLocalKnowledgeMessage's
// English text for an empty or unrecognized value.
func noLocalKnowledgeMessageFor(assistantLanguage string) string {
	if assistantLanguage == domainprofile.AssistantLanguagePortuguese {
		return noLocalKnowledgeMessagePortuguese
	}
	return domainknowledge.NoLocalKnowledgeMessage
}
