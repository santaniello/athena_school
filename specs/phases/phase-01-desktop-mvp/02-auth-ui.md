# Phase 1.2 — Auth UI (Desktop)

## Goal

User can create a local account and log in from the desktop app. Everything runs on-device (see [01-auth-backend.md](01-auth-backend.md)) — no email confirmation, no network call.

## Flow

```text
App opens → no local session → login screen
Successful login/register → session saved locally → OpenRouter key gate (see 04-onboarding.md) → main screen / onboarding
```

## UI Screens (React)

- [ ] Login screen (email + password + submit)
- [ ] Create account screen (email + password + confirm)
- [ ] Reset local account screen ("forgot password" replacement — explains this deletes the local account so the user can register again; not a real recovery, since there is no email/server)

## Wails Bindings (Go)

```go
func (a *App) Login(email, password string) (LoginResult, error)
func (a *App) Register(email, password string) error
func (a *App) ResetLocalAccount(email string) error
```

## Core (`internal/application/auth/`)

- [ ] Use cases: `Login`, `Register`, `ResetLocalAccount` (see [01-auth-backend.md](01-auth-backend.md))
- [ ] Session stored locally at `~/.athena/session.json`

## Acceptance Criteria

- New user creates a local account and lands directly on the app (no "check your email" step)
- Existing user logs in and reaches the main screen
- Session marker is present in `~/.athena/session.json` after login
- Invalid credentials show an inline error message (no crash, no modal)
- "Reset local account" removes the local account and returns to the create-account screen
