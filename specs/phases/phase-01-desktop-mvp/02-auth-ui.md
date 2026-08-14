# Phase 1.2 — Auth UI (Desktop)

## Goal

User can create a local account and log in from the desktop app. Everything runs on-device (see [01-auth-backend.md](01-auth-backend.md)) — no email confirmation, no network call.

## Flow

```text
App opens → no local session → login screen
Successful login/register → session saved locally → OpenRouter key gate (see 04-onboarding.md) → main screen / onboarding
```

## UI Screens (React)

- [x] Login screen (email + password + submit)
- [x] Create account screen (email + password + confirm)
- [x] Reset local account screen ("forgot password" replacement — explains this deletes the local account so the user can register again; not a real recovery, since there is no email/server)

## Wails Bindings (Go)

```go
func (a *App) Login(email, password string) (LoginResult, error)
func (a *App) Register(email, password string) error
func (a *App) ResetLocalAccount(email string) error
func (a *App) HasLocalSession() bool
```

`HasLocalSession` was added beyond the original list above: the Flow section requires the frontend to know on startup whether a local session already exists so it can skip the login screen, and none of the other three bindings cover that.

## Core (`internal/application/auth/`)

- [x] Use cases: `Login`, `Register`, `ResetLocalAccount` (see [01-auth-backend.md](01-auth-backend.md))
- [x] Session stored locally at `~/.athena/session.json`

## Acceptance Criteria

- New user creates a local account and lands directly on the app (no "check your email" step)
- Existing user logs in and reaches the main screen
- Session marker is present in `~/.athena/session.json` after login
- Invalid credentials show an inline error message (no crash, no modal)
- "Reset local account" removes the local account and returns to the create-account screen
