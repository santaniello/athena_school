# Phase 1.14 — Per-Session Goal

## Goal

Replace the profile-wide `Goals` field ([04-onboarding.md](04-onboarding.md)) with a
per-session `Goal`, set once when the session is created, so the system prompt's
`Goal:` line reflects what the user is actually trying to accomplish in *that*
session instead of one static, profile-wide objective repeated across every
topic.

## Domain

`UserProfile.Goals` is removed entirely — Goal is no longer collected during
onboarding or editable in Settings.

```go
// package profile
type UserProfile struct {
    Name              string
    AssistantName     string
    Area              string
    ExperienceLevel   string
    StudyStyle        string
    AssistantLanguage string
    CreatedAt         time.Time
    // Goals removed — see phase-01-desktop-mvp/14-session-goal.md
}
```

```go
// package study
type Session struct {
    ID        string
    Topic     string
    Mode      string
    FolderID  string
    Goal      string // required at creation; '' only for sessions predating this feature
    StartedAt time.Time
    Context   ContextUsage
}
```

`Service.Start(ctx, topic, folderID, goal string)` trims and requires `goal`
exactly like it already requires `topic` (`ErrGoalRequired`, mirroring
`ErrTopicRequired` in `internal/application/study/errors.go`).

## Prompt Template

`buildSystemPrompt` (`internal/application/study/prompt.go`) takes the
session's `Goal` instead of `profile.Goals`:

```text
System: You are {AssistantName}, the learning assistant of {Name}.
        Area: {Area}. Level: {ExperienceLevel}.
        Style: {StudyStyle}. Goal: {Session.Goal}.
        Topic for this session: {Topic}.
        Adapt all explanations to the user's context.
```

A session created before this feature shipped has `Goal == ""` (backfilled by
the `goal` column migration's `NOT NULL DEFAULT ''`); for those, the "Goal:
..." fragment is omitted from the "Style/Goal" line entirely rather than
rendering an empty value — there is no profile-level fallback left to use.

## UI

Session creation moves from the sidebar's inline topic-only `<Input>`
(`study-folder-tree.tsx`) to a modal dialog — same `Dialog`/`DialogContent`
pattern as the existing "New folder" dialog in the same file — collecting two
required fields: session name (topic) and Goal. "Create" stays disabled until
both are non-blank.

Onboarding (`OnboardingFormScreen`, `OnboardingConfirmScreen`) and Settings
(`SettingsScreen`) drop their Goals `TagInput` field.

## Tasks

- [ ] `internal/domain/profile/profile.go` — remove `Goals`, `ErrGoalsRequired`, `hasNonBlankGoal`
- [ ] `internal/domain/study/session.go` — add `Goal string`
- [ ] `internal/application/study/errors.go` — add `ErrGoalRequired`
- [ ] `internal/application/study/start.go` — validate + persist `goal`
- [ ] `internal/application/study/prompt.go` — build the Goal line from `session.Goal`, omitted when blank
- [ ] `internal/application/study/opening_turn.go`, `send_message.go` — thread `session.Goal` into `buildSystemPrompt`
- [ ] `internal/infrastructure/sqlite/migrations.go` — `addSessionsGoalColumn` (`ALTER TABLE sessions ADD COLUMN goal TEXT NOT NULL DEFAULT ''`)
- [ ] `internal/infrastructure/sqlite/session_repository.go` — `Create`/`GetByID`/`ListByFolder` include `goal`
- [ ] `internal/interfaces/desktop/study.go` — `StartStudySession` gains `goal`; `StudySessionResult.Goal`
- [ ] `internal/interfaces/desktop/onboarding.go`, `settings.go` — drop `Goals` from `UserProfileInput`
- [ ] `frontend/src/lib/profile.ts` — drop `goals` from `ProfileDraft`
- [ ] `frontend/src/screens/OnboardingFormScreen.tsx`, `OnboardingConfirmScreen.tsx`, `SettingsScreen.tsx` — remove Goals `TagInput`
- [ ] `frontend/src/lib/study.ts` — `StudySession.goal`, `startStudySession(topic, goal, folderId)`
- [ ] `frontend/src/components/study-folder-tree.tsx` — "New session" modal (topic + Goal, both required)
- [ ] `frontend/src/components/app-shell.tsx` — `ActiveStudySession.goal`; `handleStartNewSession` reuses it

## Acceptance Criteria

- Onboarding and Settings no longer ask for or display Goals
- Creating a session requires both a name and a Goal — "Create" is disabled until both are filled
- Two sessions in the same folder with different Goals produce visibly different opening turns (different stated objective)
- A session created before this change (no `goal` column value) still starts and streams normally, with no "Goal: ..." fragment in its system prompt
- The "Start new session" context-limit recovery action carries over the prior session's Goal without prompting again
