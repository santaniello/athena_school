# Phase 1.6 — Study Mode

## Goal

User selects a topic, starts a study session, and receives streaming personalized responses from the LLM.

## Domain

```go
// package study
type Session struct {
    ID        string
    Topic     string
    Mode      string // "study"; other modes (challenge, interview) land in later phases
    StartedAt time.Time
    EndedAt   time.Time
}

type Message struct {
    ID        string
    SessionID string
    Role      string // user | assistant — the system prompt is never persisted as a row
    Content   string
    CreatedAt time.Time
}
```

## Prompt Template

`Specialty` is intentionally absent: `UserProfile` has no such field (dropped in
[04-onboarding.md](04-onboarding.md)).

```text
System: You are {AssistantName}, the learning assistant of {Name}.
        Area: {Area}. Level: {ExperienceLevel}.
        Style: {StudyStyle}. Goal: {Goals}.
        Topic for this session: {Topic}.
        Adapt all explanations to the user's context.
```

## Tasks

- [x] `internal/domain/study/` — `Session`, `Message`, session rules
- [x] `internal/application/study/` — `Service.Start`/`SendMessage`/`End`
- [x] `UserProfile` injected into every prompt (rebuilt fresh each turn, since the system prompt itself is never persisted)
- [x] UI: chat interface with streaming response display (`StudyScreen`, via Wails `study:chunk`/`study:done`/`study:error` events)
- [x] Topic selectable via UI (free text input — no topic list exists until the Knowledge Base ships in Phase 2)
- [x] LLM generates questions; user answers; LLM gives feedback (purely conversational, driven by the system prompt — no explicit question/answer state machine)

## Acceptance Criteria

- User selects a topic and starts a session from the UI
- Response streams in real time (text appears incrementally)
- System prompt includes all fields from `UserProfile`
- Session and messages are persisted to SQLite incrementally — the user's message is written before the LLM call, and the assistant's reply only once its stream completes, so a mid-stream failure never loses the user's side of the conversation (a stricter guarantee than "on completion")
- "End session" button closes the session gracefully
