# Phase 1.2 — Auth UI (Desktop)

## Goal

User can log in, create an account, and recover a forgotten password from the desktop app.

## Flow

```text
App opens → no local token → login screen
Successful login → token saved locally → main screen
```

## UI Screens (React)

- [ ] Login screen (email + password + submit)
- [ ] Create account screen (name + email + password + confirm)
- [ ] Pending confirmation screen ("check your email")
- [ ] Password recovery screen

## Wails Bindings (Go)

```go
func (a *App) Login(email, password string) (LoginResult, error)
func (a *App) Register(name, email, password string) error
func (a *App) ForgotPassword(email string) error
```

## Core (`internal/application/auth/`)

- [ ] Use cases: `Login`, `Register`, `RefreshToken`, `ForgotPassword`
- [ ] Token stored locally at `~/.athena/session.json`
- [ ] Grace period: 7 days offline with cached token

## Acceptance Criteria

- New user creates an account and lands on the confirmation screen
- Existing user logs in and reaches the main screen
- Token is present in `~/.athena/session.json` after login
- Invalid credentials show an inline error message (no crash, no modal)
