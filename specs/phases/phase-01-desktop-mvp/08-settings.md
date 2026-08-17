# Phase 1.8 — Settings Screen

## Goal

User can configure the OpenRouter API key and update their profile fields without re-running onboarding. This is also where the user edits the key first entered at the onboarding gate (see [04-onboarding.md](04-onboarding.md)) — both screens share the same test-call validation logic; do not duplicate it.

## Config File

`~/.athena/config.yaml`

```yaml
openrouter_key: "sk-or-..."
```

`default_model_tier` was dropped from this file: model routing (see
[05-llm-service.md](05-llm-service.md)) is driven entirely by `TaskType` via
`TierFor`, with no user-facing override, so the field had no consumer.

## UI Fields

Every `UserProfile` field, not just the four originally listed here, since
they're all part of the same profile the user edited during onboarding:

- OpenRouter API key (masked input)
- Name
- Assistant name
- Area / focus
- Experience level (dropdown: beginner / intermediate / advanced)
- Goals
- Preferred study style
- Assistant language

## Tasks

- [x] Settings screen reachable from the main navigation
- [x] Saves to `~/.athena/config.yaml` and `~/.athena/profile.json` on confirm
- [x] API key validated by making a test call; error shown inline if invalid
- [x] Changes take effect immediately (no restart required)

Editing the profile preserves its original `CreatedAt` (a new
`onboarding.Service.UpdateProfile` use case, distinct from onboarding's
`SaveProfile` which always stamps a fresh one). The OpenRouter key's "no
restart required" requirement needed a backend fix beyond the UI: the running
`openrouter.Client` now exposes a concurrency-safe `SetAPIKey`, called right
after a successful key save, instead of only picking up a new key at the next
app launch.

## Acceptance Criteria

- User opens settings, changes the assistant name, saves, and the new name appears in subsequent study sessions
- Entering an invalid API key shows an error before saving
- Config file is updated on disk after saving
