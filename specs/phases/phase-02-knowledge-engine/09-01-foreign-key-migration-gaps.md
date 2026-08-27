# Phase 2.9.1 — Foreign Key Migration Gaps

## Goal

Two related data-integrity gaps in `internal/infrastructure/sqlite`'s foreign-key
enforcement, both surfaced by CodeRabbit while reviewing PR #54 (2.9 Knowledge Item
Evidence) and deliberately deferred rather than fixed as a drive-by inside an
unrelated feature PR. This spec documents both problems and their candidate
solutions; it intentionally stops short of picking one for Problem 1 and stops
short of a full implementation plan for either — that decision and the plan need
to be made before implementation starts, the same way `08-01-vectorstore-orphan-chunk-recovery.md`
did before `08-02-deleteitem-orphan-chunk-risk.md`'s decision.

## Why this is a separate spec

Both gaps live in code introduced by commit `0970781 fix(sqlite): enforce foreign
key integrity` — the prerequisite that made 2.9's Knowledge Item Evidence work
possible, committed directly without ever going through its own PR/CI/review
cycle. This is the first time that code has been exposed to a real review. Fixing
either gap safely means reordering `migrateSessionForeignKeyActions`'s
foreign-key-disabled cleanup window (see "Why the two problems are coupled"
below), which is delicate, migration-ordering-sensitive code that deserves its
own focused change with full test coverage against legacy fixtures — not a
rushed addition to an already-large, unrelated feature PR.

The project is pre-release (no shipped installs with real user databases yet), so
neither gap is an active incident. Both are worth closing before release, because
`internal/infrastructure/sqlite/db_test.go`'s own legacy fixtures already prove
the exact scenario each gap mishandles.

## Problem 1 — `PRAGMA foreign_keys` is not guaranteed per-connection

### The problem

`internal/infrastructure/sqlite/db.go`, `Open`:

```go
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	...
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil { ... }
	for _, migration := range migrations {
		if err := migration(db); err != nil { ... }
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil { ... }
	if err := checkForeignKeys(db); err != nil { ... }
	return db, nil
}
```

`PRAGMA foreign_keys = ON` is executed exactly once, via `db.Exec`, after
migrations run. In SQLite, every `PRAGMA` is a *per-connection* setting — it does
not persist on the `*sql.DB` handle, only on whichever physical connection
happened to service that one `Exec` call.

`db.SetMaxOpenConns(1)` caps the pool at one connection *at a time*, and the
existing doc comment on `Open` explains why: SQLite allows only one writer, and
capping the pool routes every query through a single connection so
`database/sql` serializes access instead of opening a second, colliding one.
That comment is correct about concurrent connections, but it does not guarantee
the connection stays the *same* one for the process's lifetime. `database/sql`
transparently discards a connection and opens a fresh one whenever the driver
returns `driver.ErrBadConn` (a disk I/O error, a corrupted page, the process
being resource-constrained, or any other condition the driver reports as
"this connection is no longer usable"). A freshly opened replacement connection
never received the `PRAGMA foreign_keys = ON` call — that `Exec` already
returned once, on the connection it happened to run on — so it silently reverts
to SQLite's default of foreign keys **off**.

The consequence is not cosmetic: `messages.session_id` and
`knowledge_item_evidence.evidence_id`/`item_id` rely on `ON DELETE
CASCADE`/`SET NULL` foreign-key actions to stay consistent. If enforcement is
silently off on the connection handling a delete:

- `messages` rows are not cascade-deleted when their `sessions` row is deleted
  (though the application code also explicitly manages this elsewhere; the FK
  is the last-resort guarantee, not the only one).
- More concretely for 2.9: `internal/application/knowledge/delete.go`'s
  `DeleteItem` deletes a `knowledge_items` row directly. Its
  `knowledge_item_evidence` links are supposed to cascade-delete via `ON DELETE
  CASCADE`. `DeleteItem` then calls `EvidenceRepository.DeleteUnreferenced`
  (`internal/infrastructure/sqlite/evidence_repository.go`), which removes an
  `Evidence` row only if `NOT EXISTS (SELECT 1 FROM knowledge_item_evidence
  WHERE evidence_id = ...)`. If the cascade silently didn't fire, the dangling
  `knowledge_item_evidence` row (pointing at a now-nonexistent item) still
  counts as a reference, so `DeleteUnreferenced` incorrectly *keeps* an Evidence
  snapshot that should have been cleaned up — a slow, silent leak, not a crash.

`checkForeignKeys` (`PRAGMA foreign_key_check`, run once at `Open` time) would
not catch this: it inspects data already written under whatever enforcement was
active at write time, not future writes on a later, differently-configured
connection.

### Options considered

**Option A — DSN `_pragma=foreign_keys(1)`.** `modernc.org/sqlite` (pinned at
v1.57.0 in `go.mod`) supports a `_pragma` DSN query parameter, executed verbatim
against every new connection the driver opens:

```go
sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
```

- Pro: enforcement becomes a property of every connection this `*sql.DB` will
  ever open, including one opened to replace a discarded bad connection — no
  code path can forget it.
- Con: enforcement is now active from the very first connection, including
  during migrations — see "Why the two problems are coupled" below.

**Option B — `sqlite.RegisterConnectionHook`.** The driver exposes a
package-level hook invoked after each new connection is set up:

```go
sqlite.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, dsn string) error {
	_, err := conn.ExecContext(context.Background(), "PRAGMA foreign_keys = ON", nil)
	return err
})
```

- Pro: same guarantee as Option A (every connection, including replacements),
  without embedding driver-specific query-string syntax in the DSN.
- Con: it is a *global*, driver-level registration — it affects every `*sql.DB`
  opened against `modernc.org/sqlite` in the process, not just this one. For
  this codebase that is a non-issue today (one process, one database), but it
  is a more surprising mechanism to reason about than a DSN parameter scoped to
  one `sql.Open` call.

**Option C — accept the risk.** `driver.ErrBadConn` requires the driver to
detect its own connection is broken; for a local, single-process SQLite file
with no network involved, this is rare. Leave `Open` as-is and revisit only if
it is ever observed in practice.

- Pro: zero change, zero risk of a migration-ordering regression (see below).
- Con: the gap is real and silent — if it ever fires, nothing signals it; it
  just quietly stops enforcing the one thing `0970781` was written to guarantee.

### Why the two problems are coupled

Whichever of Option A/B is chosen, foreign-key enforcement becomes active from
the connection `migrations` itself runs on — not just the connection `Open`
returns to callers. `migrateSessionForeignKeyActions` (see Problem 2) currently
relies on enforcement being off during its own cleanup: it issues `DELETE FROM
sessions WHERE ...` for rows whose children (`messages`, `usage`) still exist at
that point, then renames/rebuilds `messages` and `usage` with the new FK
declarations and re-inserts only the surviving rows. With enforcement on
throughout, that `DELETE FROM sessions` would need every dependent row already
gone first — child before parent — or the delete would fail outright once the
child tables also declare enforced foreign keys (today they don't yet, at that
point in the migration; the old, pre-enforcement `messages`/`usage` tables have
no declared FK to fail against, only the *new* ones created moments later do).

`PRAGMA foreign_keys` cannot be changed inside an active transaction (SQLite
rejects it) — `migrateSessionForeignKeyActions` currently runs its entire
cleanup-and-rebuild sequence inside one `db.Begin()`/`tx.Commit()`. Enabling
enforcement earlier in `Open` means this migration needs one of:

- Run its cleanup with `PRAGMA foreign_keys = OFF` explicitly issued on the
  same connection before the transaction begins, then rely on the *next*
  connection (or an explicit `PRAGMA foreign_keys = ON` after commit) to
  restore enforcement — fragile if `SetMaxOpenConns(1)` ever changes.
- Reorder so no row is ever deleted while its children still reference it —
  i.e. fix Problem 2 (reassign instead of delete) and delete children before
  parents wherever a delete is still needed, so the sequence never needs FK
  enforcement suspended at all.

The second path is the one Problem 2's proposed fix below happens to produce
for free: once orphaned-folder sessions are reassigned instead of deleted,
`migrateSessionForeignKeyActions` no longer deletes any session that still has
child rows during the transition (the "$0$ children" cases — messages/usage
that are truly orphaned from a missing session — already delete children before
any parent is touched, which is already the correct order). This is why
Problem 2 should be resolved as part of, or before, Problem 1 — not
independently.

## Problem 2 — legacy migration deletes orphaned-folder sessions instead of reassigning them

### The problem

`internal/infrastructure/sqlite/migrations.go`:

```go
// addSessionsFolderIDColumn adds sessions.folder_id if it does not already
// exist ... and backfills any existing rows to the default folder ...
func addSessionsFolderIDColumn(db *sql.DB) error {
	...
	_, err = db.Exec(`UPDATE sessions SET folder_id = 'default' WHERE folder_id IS NULL`)
	return err
}
```

```go
func migrateSessionForeignKeyActions(db *sql.DB) error {
	...
	statements := []string{
		`DELETE FROM sessions
		 WHERE folder_id IS NULL
		    OR NOT EXISTS (SELECT 1 FROM folders WHERE folders.id = sessions.folder_id)`,
		`DELETE FROM messages
		 WHERE session_id IS NULL
		    OR session_id = ''
		    OR NOT EXISTS (SELECT 1 FROM sessions WHERE sessions.id = messages.session_id)`,
		...
	}
	...
}
```

`addSessionsFolderIDColumn` (registered earlier in the `migrations` slice, and
which also guarantees the `'default'` folder row exists via an `INSERT OR
IGNORE` a few statements before it) only backfills `folder_id IS NULL`. It does
not — and given SQLite has no declared FK on that column at that point in the
migration history, cannot easily — detect a `folder_id` that is *non-NULL but
points at a folder that no longer exists* (e.g. a folder deleted through some
earlier code path that didn't reassign its sessions first).

`migrateSessionForeignKeyActions` runs later and re-checks the same condition
(`folder_id IS NULL OR NOT EXISTS (... folders ...)`), catching both the
NULL case (already meant to be impossible by then) and the orphaned-reference
case `addSessionsFolderIDColumn` cannot fix. Instead of repairing it the same
way — reassigning to `'default'` — it `DELETE`s the session outright, which
cascades into deleting every message that session ever had, through the very
next statement in the same list. This is real conversation history loss for
whatever session hit this path, framed only as "invalid" by a comment that
calls it "pre-release test data" without the code actually distinguishing test
data from a legitimately orphaned real session — the query only tests "does
the referenced folder still exist," not "is this synthetic test data."

### Proposed solution

Mirror `addSessionsFolderIDColumn`'s own backfill pattern, extended to also
cover the orphaned (non-existent-folder) case it misses:

```sql
UPDATE sessions SET folder_id = 'default'
WHERE folder_id IS NULL
   OR folder_id = ''
   OR NOT EXISTS (SELECT 1 FROM folders WHERE folders.id = sessions.folder_id)
```

...in place of the current `DELETE FROM sessions ...` statement. The `'default'`
folder row is guaranteed to exist by this point (`INSERT OR IGNORE INTO folders
... VALUES ('default', ...)` runs immediately after the `folders` table is
created, before `addSessionsFolderIDColumn`, before
`migrateSessionForeignKeyActions`, in the fixed `migrations` slice order).

The following `DELETE FROM messages WHERE session_id IS NULL OR session_id =
'' OR NOT EXISTS (... sessions ...)` statement is unaffected and stays: it
already only removes messages that never had a valid owning session at all,
which is a different, legitimate case (an orphaned message, not an orphaned
session) — untouched by this fix.

### Test plan (for whichever problem is implemented)

- Extend `internal/infrastructure/sqlite/db_test.go`'s legacy-migration fixture
  (`TestOpen_migratesLegacyForeignKeysAndDetachesUsageWithoutRemovingIt`, or a
  new sibling test) with a session whose `folder_id` points at a folder that
  was never created (or a folder id that doesn't exist), owning at least one
  message. Assert after `Open`: the session still exists, its `folder_id` is
  now `'default'`, and its message(s) still exist — the opposite of today's
  assertion that this session and its message are gone.
- If Problem 1 is implemented alongside: a test proving
  `migrateSessionForeignKeyActions` still succeeds end-to-end with foreign-key
  enforcement active throughout `Open` (not just after), against a legacy
  fixture containing every case `db_test.go` already covers (valid session,
  orphaned-folder session, orphaned message, orphaned usage).
- If Problem 1 is implemented: a targeted test that a second, freshly opened
  connection against the same `*sql.DB` (or driver-level hook registration)
  still enforces foreign keys — e.g. by forcing a connection reset (driver
  hooks/mocking may be needed here, since `driver.ErrBadConn` is not easily
  triggerable through the public `database/sql` API against a real SQLite
  file) or by directly asserting the DSN/hook configuration is in place rather
  than trying to simulate the failure.

## References

- Raised by CodeRabbit on PR #54, against `09-persistent-provenance.md`
  (Knowledge Item Evidence) — both findings were about pre-existing code from
  commit `0970781`, exposed to review for the first time by an unrelated diff
  touching nearby lines.
- `internal/infrastructure/sqlite/db.go` — `Open`, `checkForeignKeys`.
- `internal/infrastructure/sqlite/migrations.go` — `migrateSessionForeignKeyActions`,
  `addSessionsFolderIDColumn`, `hasForeignKeyDeleteAction`.
- `internal/infrastructure/sqlite/db_test.go` — the legacy-migration fixture to
  extend.
- `internal/application/knowledge/delete.go`,
  `internal/infrastructure/sqlite/evidence_repository.go` — `DeleteItem`/
  `DeleteUnreferenced`, the concrete 2.9 consequence Problem 1 describes.
- commit `0970781 fix(sqlite): enforce foreign key integrity` — introduced both
  pieces of code this spec concerns; never went through its own PR/review.
