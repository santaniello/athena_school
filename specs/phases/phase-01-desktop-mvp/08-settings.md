# Phase 1.8 — Settings Screen

## Goal

User can configure the OpenRouter API key and update their profile fields without re-running onboarding. This is also where the user edits the key first entered at the onboarding gate (see [04-onboarding.md](04-onboarding.md)) — both screens share the same test-call validation logic; do not duplicate it.

## Config File

`~/.athena/config.yaml`

```yaml
openrouter_key: "sk-or-..."
default_model_tier: medium
```

## UI Fields

- OpenRouter API key (masked input)
- Assistant name
- Area / focus
- Experience level (dropdown: beginner / intermediate / advanced)

## Tasks

- [ ] Settings screen reachable from the main navigation
- [ ] Saves to `~/.athena/config.yaml` and `~/.athena/profile.json` on confirm
- [ ] API key validated by making a test call; error shown inline if invalid
- [ ] Changes take effect immediately (no restart required)

## Acceptance Criteria

- User opens settings, changes the assistant name, saves, and the new name appears in subsequent study sessions
- Entering an invalid API key shows an error before saving
- Config file is updated on disk after saving
