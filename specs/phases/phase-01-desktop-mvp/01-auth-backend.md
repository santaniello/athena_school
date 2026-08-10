# Phase 1.1 — Auth & License Backend

## Goal

Lightweight HTTP server (Go) responsible solely for accounts and licenses.

## Domain

```go
type Account struct {
    ID        string
    Email     string
    Password  string // bcrypt
    Plan      string // trial | essencial | pro | expert
    TrialEnds time.Time
    CreatedAt time.Time
}
```

## Endpoints

```text
POST /auth/register      → create account, send confirmation email
POST /auth/login         → return JWT + refresh token
POST /auth/refresh       → renew access token
POST /auth/forgot        → initiate password recovery
GET  /account/plan       → return plan and trial status
POST /webhooks/paddle    → receive payment events
```

## Tasks

- [ ] Separate Go module or `server/` subdirectory
- [ ] JWT with 24h expiry + 30-day refresh token
- [ ] Email dispatch via SMTP (confirmation + recovery)
- [ ] Storage: SQLite for development; PostgreSQL-ready interface
- [ ] Deploy target: Railway, Fly.io, or VPS

## Acceptance Criteria

- `POST /auth/register` creates an account and sends confirmation email
- `POST /auth/login` with valid credentials returns a JWT
- `GET /account/plan` returns `{ plan: "trial", trial_ends: "..." }` for a new account
- All endpoints return structured JSON errors on failure
