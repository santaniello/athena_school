# Phase 1.1 — Local Auth Core

## Goal

Account creation and login run entirely on-device — no remote server in this phase. The design is a hexagonal port so a remote implementation can replace the local one later without touching use cases or UI.

## Domain

```go
type Account struct {
    ID           string
    Email        string
    PasswordHash string // bcrypt
    CreatedAt    time.Time
}
```

## Port (`internal/domain/auth/`)

```go
type AccountRepository interface {
    Create(ctx context.Context, account Account) error
    FindByEmail(ctx context.Context, email string) (Account, error)
    UpdatePassword(ctx context.Context, id string, passwordHash string) error
    Delete(ctx context.Context, id string) error
}
```

Today the only implementation is local (`internal/infrastructure/sqlite`, `accounts` table — see [07-sqlite.md](07-sqlite.md)). A future remote implementation (e.g. `internal/infrastructure/httpapi`) satisfies the same interface; use cases in `internal/application/auth/` never depend on which one is wired in.

## Use Cases (`internal/application/auth/`)

- `Register(email, password string) error` — hashes the password (bcrypt), creates the local `Account`
- `Login(email, password string) (Account, error)` — validates credentials against the local hash
- `ResetLocalAccount(email string) error` — deletes and allows recreating the local account; there is no email-based recovery (no remote server, no SMTP), so this is a destructive local reset, not a real recovery flow

## Local Session

- On successful login, write a session marker to `~/.athena/session.json` (account ID + timestamp)
- On subsequent launches, a valid local session skips the login screen and goes straight to the main flow
- No token, no expiry tied to a server — this replaces the old "offline grace period" concept, which only made sense with a remote-issued JWT

## Tasks

- [ ] `internal/domain/auth/` — `Account` struct, `AccountRepository` port
- [ ] `internal/application/auth/` — `Register`, `Login`, `ResetLocalAccount` use cases
- [ ] `internal/infrastructure/sqlite/` — `AccountRepository` implementation (see [07-sqlite.md](07-sqlite.md))
- [ ] Local session read/write at `~/.athena/session.json`

## Acceptance Criteria

- `Register` creates a local account with a bcrypt-hashed password; a duplicate email is rejected
- `Login` with valid local credentials succeeds and writes `~/.athena/session.json`
- `Login` with invalid credentials fails with a descriptive error, no panic
- `ResetLocalAccount` removes the existing local account so a new one can be registered with the same email
- No network call is made at any point in this flow
