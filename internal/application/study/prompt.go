package study

import (
	"fmt"
	"strings"

	domainprofile "github.com/santaniello/athena/internal/domain/profile"
)

// buildSystemPrompt renders the study-mode system prompt from profile and
// topic, per specs/phases/phase-01-desktop-mvp/06-study-mode.md. Specialty
// is intentionally omitted: UserProfile has no such field.
func buildSystemPrompt(profile domainprofile.UserProfile, topic string) string {
	return fmt.Sprintf(
		"You are %s, the learning assistant of %s.\n"+
			"Area: %s. Level: %s.\n"+
			"Style: %s. Goal: %s.\n"+
			"Topic for this session: %s.\n"+
			"Adapt all explanations to the user's context.",
		profile.AssistantName, profile.Name,
		profile.Area, profile.ExperienceLevel,
		profile.StudyStyle, strings.Join(profile.Goals, ", "),
		topic,
	)
}
