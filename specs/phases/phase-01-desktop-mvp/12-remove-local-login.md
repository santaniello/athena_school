# Phase 1.12 — Remove Local Login

## Goal

Athena is a single-user local install: no table other than `accounts`
itself ever referenced an account by foreign key, so the local
login/register/reset-account system (see
[01-auth-backend.md](01-auth-backend.md),
[02-auth-ui.md](02-auth-ui.md)) protected no data and added a screen the
user has to get through before reaching the app. It is removed. Being
"set up" is now defined purely by `~/.athena/config.yaml` (OpenRouter key)
and `~/.athena/profile.json` (onboarding profile) — the same two files
that already drove routing past login.

## What was removed

- `internal/domain/auth/`, `internal/application/auth/` — the `Account`/
  `Session` domain model and the `Register`/`Login`/`ResetLocalAccount`
  use cases
- `internal/infrastructure/sqlite` — `AccountRepository`; the `accounts`
  table itself, via a `DROP TABLE IF EXISTS accounts` migration (see
  `internal/infrastructure/sqlite/migrations.go`)
- `internal/infrastructure/session/` — the `~/.athena/session.json`
  session-marker store
- `internal/interfaces/desktop/auth.go` — the `Register`/`Login`/
  `ResetLocalAccount`/`HasLocalSession`/`Logout` Wails bindings
- `LoginScreen`, `RegisterScreen`, `ResetAccountScreen` and the "Log out"
  button in `AppShell`'s sidebar footer

## Design

`App.tsx`'s view state machine drops `'login' | 'register' | 'reset'`
entirely; `resolveInitialView`'s `HasLocalSession` check is gone, so
launch resolves straight to `HasOpenRouterKey → HasUserProfile → app`
(see [04-onboarding.md](04-onboarding.md)). `main.go` no longer wires an
`auth.Service`, session store, or `AccountRepository`; `desktop.NewApp`
drops both corresponding parameters. Any pre-existing `accounts` table
from before this change is dropped once, silently, the first time the
new binary opens the database — there is no migration path *for* account
data, since none of it was ever referenced by anything else.

Any `~/.athena/session.json` left over from a prior install is not
cleaned up: nothing reads it anymore, so it is an inert, harmless file.

## Known risk

`DROP TABLE IF EXISTS accounts` is one-way — there is no migration that
recreates a dropped account row. Accepted: the table only ever held
local login credentials, and this codebase's only install at the time of
this change had test data only, no accounts anyone depended on.

## Out of scope

- What (if anything) replaces the sidebar footer's "Log out" button —
  left empty here; a later spec (a local-data reset action) decides what
  goes there.
- Any change to `KeyGateScreen`, `OnboardingScreen`, `profile.json`, or
  `config.yaml` — none of them ever depended on an account.

## Acceptance Criteria

- Launching the app with no OpenRouter key configured shows the key gate,
  never a login screen
- Launching the app with a key and profile already configured goes
  straight to the app shell
- No table named `accounts` exists in `~/.athena/athena.db` after
  opening it with the new binary, even if it existed before
- No source file references `Account`, `Session` (auth), `Register`,
  `Login`, `ResetLocalAccount`, `HasLocalSession`, or `Logout`
