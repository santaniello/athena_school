# Phase 1.10 — Study Folders

## Goal

Group study sessions into folders (like ChatGPT Projects), persist every session so the user can browse and
reopen it any time, and navigate that history from an explorer-style tree in the sidebar instead of losing the
transcript once a session ends.

## Domain

```go
// package folder
type Folder struct {
    ID        string
    Name      string
    IsDefault bool
    CreatedAt time.Time
}

// DefaultFolderID is the fixed ID of the auto-seeded default folder every
// session falls back to when no folder is chosen.
const DefaultFolderID = "default"
```

```go
// package study — extended
type Session struct {
    ID        string
    Topic     string
    Mode      string
    FolderID  string // always populated; falls back to folder.DefaultFolderID
    StartedAt time.Time
    EndedAt   time.Time
}
```

`Session.FolderID` references `folder.Folder` by ID only — no cross-package Go type coupling, mirroring how
`Message.SessionID` already relates to `Session` today.

## Design Decisions

- **`folder` is its own domain package**, not nested under `study`: it has its own lifecycle (create/rename/delete)
  and, since `Session.Mode` already anticipates non-study session types (`challenge`, `interview` in later phases)
  sharing the same table, folders are an organizational concept over sessions in general — not specific to `study`.
- **The default folder always exists** (seeded via migration, ID `default`, name "General") — every session always
  has a real `FolderID`, never null or a virtual "no folder" bucket. `NOT NULL` is enforced at the application
  layer (`Start` always populates `FolderID` before persisting), not the SQLite schema, because SQLite cannot add
  a `NOT NULL` column with a table rebuild to an existing table without one.
- **Reopening an ended session** clears `EndedAt` and lets the user keep chatting — not read-only. `SendMessage`
  already never checked `IsOpen()`, so this is purely a `Reopen` repository method plus UI/state changes, no new
  enforcement.
- **Folder CRUD is complete in this delivery**: create, rename (including the default folder), delete. Deleting a
  folder reassigns its sessions to the default folder first — sessions are never deleted.
- **The default folder cannot be deleted** (`ErrCannotDeleteDefaultFolder`), since it must always exist as the
  fallback target.
- **Moving an existing session to a different folder** is in scope for this delivery.
- **Navigation lives in the sidebar as an explorer-style tree**, not a separate list screen: clicking "Study" in
  the rail expands into folders; clicking a folder expands into its sessions; clicking a session opens it in the
  main pane. Validated with the user via an interactive mockup before implementation.
- **UI copy is in English**, matching the rest of the existing Study/Home/Settings screens (a pre-existing
  divergence from this document's own Portuguese-UI convention — see [AGENTS.md](../../../AGENTS.md) — kept
  intentional here for consistency with the surrounding screen rather than mixing languages).

## Tasks

- [ ] `internal/domain/folder/` — `Folder`, `Repository` port, sentinel errors
- [ ] `internal/infrastructure/sqlite/` — `folders` table + default-folder seed, `FolderRepository`
- [ ] `internal/domain/study/` — `Session.FolderID`; `SessionRepository` gains `GetByID`, `ListByFolder`,
      `Reopen`, `MoveToFolder`, `ReassignFolder`
- [ ] `internal/infrastructure/sqlite/` — `sessions.folder_id` column (conditional `ALTER TABLE` + backfill to
      `default`), `SessionRepository` method implementations
- [ ] `internal/application/folder/` — `CreateFolder`, `RenameFolder`, `DeleteFolder` (reassigns sessions first),
      `ListFolders`
- [ ] `internal/application/study/` — `Start` accepts `folderID` (falls back to `folder.DefaultFolderID`),
      `Resume` (reopens if needed + loads history), `MoveToFolder`, `ListSessionsByFolder`
- [ ] `internal/interfaces/desktop/` — `CreateFolder`/`RenameFolder`/`DeleteFolder`/`ListFolders` bindings;
      `StartStudySession(topic, folderID)`, `ResumeStudySession`, `MoveStudySession`, `ListStudySessionsByFolder`
- [ ] UI: `study-folder-tree.tsx` sidebar component (expand/collapse folders and sessions, create/rename/delete
      folder, move session, start a new session inline)
- [ ] UI: `StudyChatScreen.tsx` (renamed from `StudyScreen.tsx`) supports resuming a session, loading its history
- [ ] UI: `app-shell.tsx` owns the selected-session state, renders the tree in the rail and the chat/empty state
      in the main pane

## Acceptance Criteria

- A "General" default folder exists automatically — pre-existing sessions and any session started without
  explicitly choosing a folder land there
- User can create, rename, and delete folders from the sidebar tree; deleting a folder moves its sessions to
  "General" without deleting them; "General" itself cannot be deleted
- User can start a new session inside a specific folder from the tree
- User can move an existing session to a different folder
- Ended sessions remain visible in the tree and can be reopened — reopening clears `EndedAt` and lets the user
  keep chatting, loading the full prior transcript first
- Session history survives an app restart (already true at the SQLite layer; this phase makes it reachable from
  the UI for the first time)
