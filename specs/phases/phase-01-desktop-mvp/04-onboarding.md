# Phase 1.4 — Onboarding Interview

## Goal

First-time users are guided through a conversational interview that produces a `UserProfile` used to personalize every future session.

## Flow

```text
First login → no local UserProfile → onboarding screen
LLM conducts conversational interview (3–5 turns)
User answers → profile generated → confirmation screen → saved locally
```

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
- [ ] UI: conversational chat (not a form)
- [ ] Confirmation screen with inline editing before saving

## Acceptance Criteria

- User who has never run the app sees the onboarding screen after first login
- The LLM collects all required fields through natural conversation
- Confirmation screen shows the collected profile; user can edit before saving
- After saving, subsequent launches skip onboarding and go directly to the main screen
- `~/.athena/profile.json` exists and is valid JSON after onboarding completes
