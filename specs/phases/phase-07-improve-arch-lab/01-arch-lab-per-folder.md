# Phase 7.1 — Architecture Lab per Folder

## Goal

Only the folders the user marks can generate Architecture Lab challenges. Today any study session,
in any folder, can: a session about "Java Streams" can be turned into a system-design exercise, and
the model forces a scenario out of it. A per-folder switch, **Enable Arch Lab**, makes the
user say which folders hold architecture study.

The switch is an explicit setting, not a folder name. Matching a name such as "System Design" was
considered and rejected: it breaks silently when the folder is renamed, needs case and spelling
rules, and ties a feature to a free-text string.

## Decisions

1. **A flag on the folder.** `Folder.ArchLabEnabled`, stored as
   `folders.arch_lab_enabled INTEGER NOT NULL DEFAULT 0`. A new folder is created disabled.
2. **Nothing is enabled for the user.** There is no seed and no backfill, not even by name: the
   user's existing folders come out of the migration disabled and the user enables the ones they
   want. After a reset of the local data ([1.13](../phase-01-desktop-mvp/13-reset-local-data.md))
   no folder exists, and a new one starts disabled. This matches
   [1.15](../phase-01-desktop-mvp/15-remove-default-folder.md): the app creates nothing for the
   user.
3. **What it gates: generating a challenge from a session.** A challenge can be generated from a
   session only when that session's folder has the flag on. This covers both entry points that
   exist today: the catalog's generation dialog ([3.7](../phase-03-architecture-lab/07-challenge-catalog-ui.md))
   and the exercise offer in the study chat ([3.14](../phase-03-architecture-lab/14-ai-challenge-generation.md)).
   [7.3](03-canonical-interviews.md) later makes interviews session-free; this gate then applies to
   practice challenges only.
4. **The backend enforces it.** `GenerationService.GenerateChallenge` fails with
   `ErrArchLabNotEnabled` and stores nothing when the session's folder is off. The screens also
   hide what cannot work, but they are not the control.
5. **The flag is read when generating, never remembered.** Sessions can be dragged between folders,
   and a folder can be switched off later. The check always reads the session's folder as it is at
   that moment. Moving a session or switching a folder off changes nothing that already exists.
6. **Switching off blocks only new generation.** Challenges already created, their attempts,
   submissions, evaluations and history stay as they are, and an existing challenge can still be
   started. Challenge content is immutable ([3.1](../phase-03-architecture-lab/01-challenge-domain-and-persistence.md))
   and history is permanent ([3.13](../phase-03-architecture-lab/13-evaluation-and-history-screen.md)).
7. **Catalog dialog.** The session list of the generation dialog contains only sessions of enabled
   folders. With none, the dialog explains: "Enable the Architecture Lab on a folder to generate
   challenges from its sessions." and offers no Generate button. The existing "no study sessions"
   message stays for the case where there are no sessions at all.
8. **Chat.** In a session of a disabled folder the tutor does not offer exercises:
   - the study prompt leaves out the exercise-offer instruction (`exerciseOfferInstruction`), so the
     model is never told to emit the marker, and a request for an exercise gets a normal reply;
   - if the marker still arrives, it is stripped from the text, as today, and **no offer is
     created**;
   - quiz and mind map offers are not affected by the flag.
9. **A pending offer whose folder was switched off.** `Accept` fails with `ErrArchLabNotEnabled`.
   The offer stays pending with the error shown and **Decline** still works, the same behavior as
   any failed accept ([3.14](../phase-03-architecture-lab/14-ai-challenge-generation.md) decision 9).
   This also covers the reuse path of `Accept`, which does not call the model: the check comes
   first.
10. **Where the user flips it.** The folder's actions menu in the study tree, next to rename and
    delete, gets **Enable Arch Lab** or **Disable Arch Lab**, according to the
    current value, and the folder row shows a small indicator when it is on. There is no checkbox
    in the new-folder dialog: one extra step after creating the folder keeps the create flow
    untouched.
11. **No confirmation on disable.** Disabling is harmless and reversible (decision 6).
12. **Language.** Folder and study screens stay in English and do not use i18next. The catalog
    dialog's new message is Architecture Lab text: it lives in the `lab` namespace, in
    `en.ts` and `pt.ts` ([3.20](../phase-03-architecture-lab/20-lab-language.md)).
13. **No data compatibility to keep.** The column is added with its default.

## Domain

`internal/domain/folder/`:

```go
type Folder struct {
    ID             string
    Name           string
    ArchLabEnabled bool
    CreatedAt      time.Time
}
```

`Repository` gains `SetArchLabEnabled(ctx, id string, enabled bool) error`, returning
`ErrFolderNotFound` for an unknown id. `Create` always stores the flag as off.

`internal/domain/exercise/`: `ErrArchLabNotEnabled`.

## Application

- `folder.Service.SetArchLabEnabled(ctx, id, enabled)`; `ListFolders` returns the flag.
- `exercise.SessionTopicReader` ([3.1](../phase-03-architecture-lab/01-challenge-domain-and-persistence.md))
  also returns whether the session's folder has the lab enabled. The `exercise` package still
  imports neither `study` nor `folder`.
- `GenerationService.GenerateChallenge` and `OfferService.Accept` check it first (decisions 4 and
  9) and return `ErrArchLabNotEnabled`.
- `SessionService.ListSessionsForGeneration` returns only sessions of enabled folders.
- `study.buildSystemPrompt` takes the flag and includes `exerciseOfferInstruction` only when it is
  on. The offer detector skips `CreateOffer` for the exercise marker when it is off, still
  stripping the marker.

## SQLite

Guarded migration in `internal/infrastructure/sqlite/migrations.go`, in the existing style
(`hasColumn` check, runs once):

```sql
ALTER TABLE folders ADD COLUMN arch_lab_enabled INTEGER NOT NULL DEFAULT 0;
```

The folder repository reads and writes the column. `session_option_repository.go` joins the folder
and filters on the flag.

## Desktop bindings and frontend

- `FolderResult` gains `archLabEnabled`; new binding `SetFolderArchLabEnabled(id, enabled)`, thin,
  an error code per sentinel (`arch_lab_not_enabled` for the generation and accept bindings).
  `wailsjs` regenerated.
- `frontend/src/lib/folder.ts`: the `Folder` type and `setFolderArchLabEnabled`.
- `study-folder-tree.tsx`: the menu item and the indicator (decision 10).
- `generate-challenge-dialog.tsx`: the empty state of decision 7, from the `lab` namespace.
- The offer card shows the accept error through the existing error path.

## Tasks

Each slice is TDD (Go with testify and Mockery mocks, Vitest for the frontend), keeps the build
green and is committed on its own.

- [ ] Domain: `ArchLabEnabled`, `Repository.SetArchLabEnabled`, `ErrArchLabNotEnabled`
- [ ] SQLite: guarded migration, repository read and write, `ErrFolderNotFound`
- [ ] Application: `folder.Service.SetArchLabEnabled`, flag in `ListFolders`
- [ ] SQLite and application: `ListSessionsForGeneration` filters by the flag
- [ ] Application: `GenerateChallenge` and `Accept` refuse a disabled folder, nothing stored
- [ ] Application: study prompt omits the exercise-offer instruction when off; detector creates no
      offer but strips the marker
- [ ] Bindings, `wailsjs` and mocks regenerated
- [ ] Frontend: folder menu item and indicator; generation dialog empty state (`en` and `pt`)
- [ ] Docs: CHANGELOG, README and `lib/documentation.ts`

## Acceptance Criteria

- A new folder is disabled. After the migration every existing folder is disabled.
- Enabling and disabling from the folder menu persists and shows the indicator.
- `GenerateChallenge` for a session in a disabled folder fails with `ErrArchLabNotEnabled`, calls
  no LLM and stores nothing; in an enabled folder it works as before.
- The catalog dialog lists only sessions of enabled folders; with none enabled it shows the
  explanation and no Generate button.
- In a disabled folder, asking the tutor for an exercise gets a normal reply with no offer card,
  and the marker text is never visible; in an enabled folder the offer appears as before.
- A pending offer in a folder switched off fails on Accept with the error, stays pending and can be
  declined.
- Switching a folder off leaves its challenges, attempts and history intact, and an existing
  challenge can still be started.
- Dragging a session to another folder changes whether it can generate, from the next call on.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  application/domain code, lint and vulnerability checks, and the frontend tests, coverage, lint,
  typecheck, `npx prettier --check .` and Stryker on the changed files pass.

## Out of scope

- A folder-name rule, a checkbox in the new-folder dialog, and any automatic enabling.
- Gating quiz, mind map or tutor behavior by the flag.
- Hiding the Architecture Lab screen from the sidebar when no folder is enabled.
- Classifying folders or sessions by topic.
- Interviews without a session ([7.3](03-canonical-interviews.md)).
