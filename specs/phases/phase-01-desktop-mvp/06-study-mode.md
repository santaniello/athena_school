# Phase 1.6 — Study Mode

## Goal

User selects a topic, starts a study session, and receives streaming personalized responses from the LLM.

## Domain

```go
type StudySession struct {
    ID        string
    Topic     string
    StartedAt time.Time
    EndedAt   time.Time
}
```

## Prompt Template

```text
System: You are {AssistantName}, the learning assistant of {Name}.
        Area: {Area}. Focus: {Specialty}. Level: {ExperienceLevel}.
        Style: {StudyStyle}. Goal: {Goals}.
        Adapt all explanations to the user's context.
```

## Tasks

- [ ] `internal/domain/study/` — `StudySession`, session rules
- [ ] `internal/application/study/` — `StudyService.Start(topic string)`
- [ ] `UserProfile` injected into every prompt
- [ ] UI: chat interface with streaming response display
- [ ] Topic selectable via UI (text input or list)
- [ ] LLM generates questions; user answers; LLM gives feedback

## Acceptance Criteria

- User selects a topic and starts a session from the UI
- Response streams in real time (text appears incrementally)
- System prompt includes all fields from `UserProfile`
- Session and messages are persisted to SQLite on completion
- "End session" button closes the session gracefully
