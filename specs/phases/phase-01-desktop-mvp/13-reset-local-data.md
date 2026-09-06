# Phase 1.13 — Reset Local Data

## Goal

User can permanently clear their study history, folders, and knowledge
base from the Settings screen, without closing the app or losing their
OpenRouter key and onboarding profile — a live "start fresh" action, not
the account-deletion flow removed in
[12-remove-local-login.md](12-remove-local-login.md).

## Scope

Deletes: every study session and message, every folder except the seeded
`default` one, and every knowledge-domain row (items, chunks, evidence,
reconciliation proposals, ingested-file dedup records), plus the
in-memory vector index built from them.

Does **not** delete: `~/.athena/config.yaml` (OpenRouter key, extraction
limit), `~/.athena/profile.json` (onboarding profile), or `usage` (LLM
cost/token history) — `usage.session_id` is set `NULL` by its own
foreign key when sessions are deleted, matching how deleting a single
session already behaves (see
`TestOpen_migratesLegacyForeignKeysAndDetachesUsageWithoutRemovingIt`).

## Design

`internal/domain/reset.Resetter` is a single port (`Reset(ctx) error`),
implemented in `internal/infrastructure/sqlite/reset.go` as one
transaction: `sessions` → `folders WHERE id <> 'default'` →
`knowledge_items` → `knowledge_reconciliation_proposals` →
`knowledge_evidence` → `knowledge_chunks` → `ingested_files`. This order
matters — `knowledge_items` and `knowledge_reconciliation_proposals` are
deleted before `knowledge_evidence` because their junction tables
(`knowledge_item_evidence`, `knowledge_reconciliation_evidence`) only
cascade from the item/proposal side, not from evidence.

`internal/application/reset.Service.ResetLocalData` calls the `Resetter`
then `knowledge.VectorStore.ReplaceAll(ctx, nil)`, keeping the in-memory
index coherent with SQLite the same way every other knowledge mutation
already does (ADR-004). The Wails binding
(`internal/interfaces/desktop/reset.go`) calls the use case and, only on
success, `wailsruntime.WindowReloadApp` — the window and Go process never
close; the frontend remounts fresh, so every piece of UI state (folder
tree, topic tree, review badges) reflects the empty state without each
being reset individually.

The reset lives on the sqlite package rather than as a method added to
each of the affected repositories — the same "one place knows the whole
schema" role `migrations.go` already has, just for DML instead of DDL.
Adding a table later that should also be cleared means updating
`reset.go`, the same maintenance cost `migrations.go` already carries for
schema changes.

## UI

A "Danger zone" section at the bottom of the Settings screen (the same
screen edits the profile — see [08-settings.md](08-settings.md)), with a
destructive "Reset local data" button that opens a confirmation dialog
(`ResetLocalDataDialog`) naming what is deleted and what survives. On
confirm, the dialog shows "Resetting..." until the binding resolves; on
error, an inline message is shown and the dialog stays open for retry.

## Known risks

- Resetting during an in-flight study session stream can make that
  stream error out mid-response (its session row disappears under it).
  Accepted — no guard added; the resulting error is handled by the
  existing streaming error paths.
- A future table that should also be cleared by this reset must be added
  to `reset.go` by hand; nothing enforces that automatically.

## Acceptance Criteria

- Confirming the dialog deletes every session, message, non-default
  folder, and knowledge-domain row; `default` folder and `usage` survive
- `config.yaml` and `profile.json` are untouched — no re-onboarding or
  re-entering the OpenRouter key is required after a reset
- The app window and process never close; the frontend reloads and lands
  on Home
- A failed reset shows an inline error and does not reload the app
