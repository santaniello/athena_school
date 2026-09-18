# Phase 1.15 — Remove the default folder

## Goal

Remove the hardcoded "General" default folder introduced in
[10-study-folders.md](10-study-folders.md). Folders are always created by the user for a real
topic — there is no auto-seeded fallback, and no folder is protected from deletion.

## Domain

```go
// package folder
type Folder struct {
    ID        string
    Name      string
    CreatedAt time.Time
}
```

`DefaultFolderID`, `ErrCannotDeleteDefaultFolder`, and `Folder.IsDefault` are all removed.
`study.Start` now requires a non-blank `folderID`, returning the new `ErrFolderRequired`
otherwise — there is no fallback folder to default into.

## Design Decisions

- **Deleting a folder deletes its sessions.** `DeleteFolder` calls a new
  `SessionRepository.DeleteByFolder` before `folders.Delete`, and `sessions.folder_id` now
  declares `ON DELETE CASCADE` against `folders(id)` at the schema level too — deleting a
  session cascades to its messages and persisted sources (`messages`, `message_sources`), same
  as deleting a session directly already did. `knowledge_items` are untouched: they are not
  folder-scoped in this delivery — linking knowledge to folders is a future spec.
- **Zero folders is a normal state**, not an error condition — on first install, or after
  deleting the last folder. The Study section shows an empty state ("No folders yet") instead
  of assuming a folder already exists to browse or start a session in. `StudyFolderTree` reports
  its folder count up to `AppShell` (`onFolderCountChange`) so the main pane can tell "no
  folders yet" apart from "folders exist but nothing is selected."
- **Deleting the last folder uses the same confirmation dialog as any other folder** — no
  extra warning for the destructive edge case of losing every session at once.
- **No destructive migration for existing installs.** The migration that seeded `('default',
  'General', 1, ...)` is removed outright; nothing replaces it. If a `folders.is_default` column
  and a `default` row already exist from before this change, `dropFoldersIsDefaultColumn` only
  strips the column — the row and its sessions survive as an ordinary folder, renameable and
  deletable like any other.

## Migration notes

Two destructive-schema migrations were needed (`dropFoldersIsDefaultColumn`,
`addSessionsFolderIDCascade`), both rebuilt via
create-under-a-temporary-name → copy → drop-old → rename-into-place, not the more obvious
rename-old-away → create → copy → drop-old order used elsewhere in this file
(`migrateSessionForeignKeyActions`): renaming a table that other tables reference by foreign key
makes SQLite silently rewrite those other tables' `REFERENCES` clauses to the temporary name,
leaving them permanently dangling once that temporary table is dropped — breaking every later
query against them. `folders` is referenced by `sessions.folder_id`; `sessions` is referenced by
`messages.session_id` and `usage.session_id`. Renaming the fresh temporary table into the
now-free final name instead avoids the rewrite entirely, since nothing ever references the
temporary name. `PRAGMA foreign_keys` is also toggled off for the duration of each rebuild:
dropping the old table while another table's `NO ACTION`/`CASCADE` foreign key still points at
it can otherwise block the migration outright or silently cascade-delete unrelated rows.

`repairSessionsWithInvalidFolder` (the deletion equivalent of the old reassign-to-default
repair) was moved to run after `migrateSessionForeignKeyActions` rather than staying next to
`addSessionsFolderIDColumn`: deleting a session whose `usage` row still predates the SET NULL
foreign-key upgrade would otherwise fail the same way.

## Acceptance Criteria

- No folder is seeded automatically; a fresh install starts with zero folders.
- The user must create a folder before starting a session — `Start` rejects a blank `folderID`.
- Deleting a folder permanently deletes every session inside it (and their messages/sources);
  nothing is reassigned elsewhere.
- Any folder, including the last one remaining, can be deleted through the same confirmation
  flow.
- Existing installs that already have a `default`/`General` folder keep it, and its sessions,
  across the upgrade — it just stops being protected or auto-recreated.
