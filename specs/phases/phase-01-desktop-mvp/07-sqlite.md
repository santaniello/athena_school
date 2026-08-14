# Phase 1.7 — Local Persistence (SQLite)

## Goal

All session data is stored locally in SQLite using a pure-Go driver (no CGO). The `accounts` table also backs local auth (see [01-auth-backend.md](01-auth-backend.md)) — there is no remote database in this phase.

## Driver

`modernc.org/sqlite` — pure Go, no CGO, cross-platform.

## Schema

```sql
CREATE TABLE accounts (
    id            TEXT PRIMARY KEY,
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    DATETIME
);

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    topic      TEXT,
    mode       TEXT, -- study | challenge | interview
    started_at DATETIME,
    ended_at   DATETIME
);

CREATE TABLE messages (
    id         TEXT PRIMARY KEY,
    session_id TEXT REFERENCES sessions(id),
    role       TEXT, -- user | assistant
    content    TEXT,
    created_at DATETIME
);

CREATE TABLE usage (
    id            TEXT PRIMARY KEY,
    session_id    TEXT REFERENCES sessions(id),
    model         TEXT,
    input_tokens  INTEGER,
    output_tokens INTEGER,
    cost          REAL,
    created_at    DATETIME
);
```

## Tasks

- [ ] `internal/infrastructure/sqlite/` — repository implementations
- [ ] Migration runner: applies schema on first run, no-op on subsequent runs
- [ ] Database file: `~/.athena/athena.db`
- [ ] Repository interfaces defined in `internal/domain/` (not in infrastructure)

## Acceptance Criteria

- `~/.athena/athena.db` is created on first launch
- A registered local account has a row in `accounts` with a bcrypt `password_hash`, never a plaintext password
- A completed study session has a row in `sessions` and rows in `messages`
- Token usage from each LLM call has a row in `usage`
- Running the app a second time does not re-run migrations or corrupt the database
