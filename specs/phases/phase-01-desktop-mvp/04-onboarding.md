# Phase 1.4 — Onboarding Interview

## Goal

First-time users are guided through a conversational interview that produces a `UserProfile` used to personalize every future session.

## Flow

```text
First login → no local UserProfile → onboarding screen
No openrouter_key in ~/.athena/config.yaml → mandatory "Connect your OpenRouter key" screen
  (masked input, validated with a test call — same validation as 08-settings.md) → key saved
LLM conducts conversational interview (3–5 turns)
User answers → profile generated → confirmation screen → saved locally
```

The interview cannot start without a valid OpenRouter key, since it is itself an LLM call (see [05-llm-service.md](05-llm-service.md)). The key gate only appears once — subsequent app launches skip it if `openrouter_key` is already present, same as the settings screen ([08-settings.md](08-settings.md)) which lets the user change it later.

## Information Collected

- Name
- Area of work / what they study
- Specific focus
- Experience level: `beginner | intermediate | advanced`
- Main goal
- Preferred study style
- What they want to call the assistant

## Domain

```go
type UserProfile struct {
    Name            string    `json:"name"`
    AssistantName   string    `json:"assistant_name"`
    Area            string    `json:"area"`
    Specialty       string    `json:"specialty"`
    ExperienceLevel string    `json:"experience_level"` // beginner | intermediate | advanced
    Goals           []string  `json:"goals"`
    StudyStyle      string    `json:"study_style"`
    CreatedAt       time.Time `json:"created_at"`
}
```

## Tasks

- [ ] `internal/domain/profile/` — `UserProfile` struct and validation
- [ ] `internal/application/onboarding/` — interview conductor logic
- [ ] `UserProfile` persisted to `~/.athena/profile.json`
- [ ] UI: "Connect your OpenRouter key" gate screen, shown before the interview when `openrouter_key` is missing (reuses the test-call validation from [08-settings.md](08-settings.md))
- [ ] UI: conversational chat (not a form)
- [ ] Confirmation screen with inline editing before saving

## Acceptance Criteria

- User who has never run the app sees the onboarding screen after first login
- If no OpenRouter key is configured, the key gate screen appears and blocks the interview until a valid key is saved
- The LLM collects all required fields through natural conversation
- Confirmation screen shows the collected profile; user can edit before saving
- After saving, subsequent launches skip onboarding and go directly to the main screen
- `~/.athena/profile.json` exists and is valid JSON after onboarding completes
