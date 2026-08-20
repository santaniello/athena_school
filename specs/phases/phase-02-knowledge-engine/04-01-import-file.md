# Phase 2.4.1 — Single-File Import

## Goal

Today `ImportFolder` (2.3) is the only entry point into the notes pipeline: the
user must pick a directory, even to bring in exactly one note. This spec adds a
second entry point — importing a single `.md`/`.txt` file directly — without
duplicating any pipeline logic (chunking, embedding, skip-if-unchanged,
`IndexWarnings`). Folder import is unchanged; this is strictly additive.

## Dependencies

- 2.3 supplies the entire pipeline this spec reuses: chunking, shadow Items,
  `IngestedFile` dedup, and the transactional replace-per-file logic inside
  `ingestFile`.
- 2.4 supplies the vector-store reconciliation (`IndexingWarning`,
  `IndexWarnings`) that a single-file import surfaces exactly like a folder
  import does.

## Use case

`internal/application/ingest/import_folder.go` extracts the per-candidate loop
body of `ImportFolder` (guard already checked, candidates already collected)
into an unexported helper shared by both entry points:

```go
func (s *Service) ImportFolder(ctx context.Context, root fs.FS, onProgress func(Progress) error) (Summary, error) {
    if err := s.index.CheckMutationAllowed(); err != nil {
        return Summary{}, err
    }
    candidates, err := collectCandidates(root)
    if err != nil {
        return Summary{}, fmt.Errorf("ingest: scanning folder: %w", err)
    }
    return s.importCandidates(ctx, root, candidates, onProgress)
}

// ImportFile imports exactly one .md/.txt file — filePath is relative to
// root (the desktop binding opens the file's parent directory via
// os.OpenRoot and passes the file's base name, the same confinement 2.3
// already relies on for folder import). Shares every rule with ImportFolder
// (skip-if-unchanged, chunking, embedding, IndexWarnings) via
// importCandidates — the only difference from a folder import is a
// one-entry candidate list instead of a full directory walk.
func (s *Service) ImportFile(ctx context.Context, root fs.FS, filePath string, onProgress func(Progress) error) (Summary, error) {
    if err := s.index.CheckMutationAllowed(); err != nil {
        return Summary{}, err
    }
    switch strings.ToLower(path.Ext(filePath)) {
    case ".md", ".txt":
    default:
        return Summary{}, fmt.Errorf("ingest: unsupported file type %q", path.Ext(filePath))
    }
    return s.importCandidates(ctx, root, []string{filePath}, onProgress)
}

// importCandidates runs the mtime skip check, ingestFile, and Summary/
// Progress bookkeeping shared by ImportFolder and ImportFile. It never
// touches s.index — both public entry points already checked the guard
// before collecting their own candidate list.
func (s *Service) importCandidates(ctx context.Context, root fs.FS, candidates []string, onProgress func(Progress) error) (Summary, error)
```

`Progress`, `Summary`, and `FileFailure` are unchanged — a single-file import
reports through the exact same shapes, just with `FilesTotal == 1`.

## Desktop binding

`internal/interfaces/desktop/app.go` gains an injectable file-picker field,
mirroring the existing `openDirectory` field (the real
`wailsruntime.OpenFileDialog`, like `OpenDirectoryDialog`, calls
`log.Fatal`/`os.Exit` outside a real Wails runtime context, which would abort
the test binary):

```go
openFile func(ctx context.Context, options wailsruntime.OpenDialogOptions) (string, error)
```

wired in `NewApp` to `wailsruntime.OpenFileDialog`.

`internal/interfaces/desktop/ingest.go` gains two bindings, next to
`PickNotesFolder`/`ImportNotes`:

```go
// PickNotesFile opens the OS file picker restricted to .md/.txt, and
// returns the chosen path, or "" if the user cancelled.
func (a *App) PickNotesFile() (string, error) {
    return a.openFile(a.ctx, wailsruntime.OpenDialogOptions{
        Title: "Select a note file",
        Filters: []wailsruntime.FileFilter{
            {DisplayName: "Notes (*.md, *.txt)", Pattern: "*.md;*.txt"},
        },
    })
}

// ImportFile imports exactly the single file at path, streaming progress
// via the same "ingest:progress"/"ingest:done"/"ingest:error" events as
// ImportNotes — the UI only ever has one import active at a time, so
// reusing the channel keeps IngestProgressDialog identical for both flows.
func (a *App) ImportFile(path string) error {
    dir := filepath.Dir(path)
    root, err := os.OpenRoot(dir)
    if err != nil {
        a.emit(a.ctx, eventIngestError, err.Error())
        return err
    }
    defer func() {
        if closeErr := root.Close(); closeErr != nil {
            log.Printf("closing notes import root %q: %v", dir, closeErr)
        }
    }()

    summary, err := a.ingest.ImportFile(a.ctx, root.FS(), filepath.Base(path), func(p ingest.Progress) error {
        a.emit(a.ctx, eventIngestProgress, toIngestProgressResult(p))
        return nil
    })
    if err != nil {
        a.emit(a.ctx, eventIngestError, err.Error())
        return err
    }
    a.emit(a.ctx, eventIngestDone, toIngestSummaryResult(summary))
    return nil
}
```

No new events, no new DTOs — `IngestProgressResult`/`IngestSummaryResult` are
reused unchanged.

## Frontend

`frontend/src/lib/ingest.ts` adds thin wrappers, same shape as
`pickNotesFolder`/`importNotes`:

```ts
export async function pickNotesFile(): Promise<string> {
  return PickNotesFile()
}

export async function importFile(path: string): Promise<void> {
  await ImportFile(path)
}
```

`frontend/src/components/knowledge-section.tsx`'s single **"Import notes"**
button becomes a `DropdownMenu` trigger (same component already used in
`study-folder-tree.tsx`), with two items:

```text
[ Import notes ▾ ]
   ├─ Import folder...
   └─ Import file...
```

State moves from `importFolderPath: string | null` to a unified target:

```ts
const [importTarget, setImportTarget] = useState<{ kind: 'folder' | 'file'; path: string } | null>(null)
```

`frontend/src/components/ingest-progress-dialog.tsx` generalizes its props
from `folderPath` to `kind: 'folder' | 'file'` + `path: string`, and calls
`importNotes(path)` or `importFile(path)` accordingly inside the same
open/cleanup effect. The finished-state summary, the failures list, and the
`IndexWarnings` notice are all unchanged — they already work per-file. The
in-progress description text branches on `kind`:

- folder: "Processing files in the selected folder."
- file: "Processing the selected file."

No cancel affordance, exactly as 2.3 — same rationale (no operation in the app
is cancellable today, and each file replace is already an isolated
transaction).

## Tasks

- [ ] `internal/application/ingest/import_folder.go` — extract
      `importCandidates`, add `ImportFile`
- [ ] `internal/application/ingest/import_folder_test.go` (or a new
      `import_file_test.go`) — new file ingested, unchanged file skipped,
      changed file re-ingested, invalid extension rejected,
      `CheckMutationAllowed` guard rejects, `IndexingWarning` surfaced
- [ ] `internal/interfaces/desktop/app.go` — `openFile` field, wired to
      `wailsruntime.OpenFileDialog` in `NewApp`
- [ ] `internal/interfaces/desktop/ingest.go` — `PickNotesFile`, `ImportFile`
- [ ] `internal/interfaces/desktop/ingest_test.go` — mirrors the existing
      `PickNotesFolder`/`ImportNotes` test pairs, stubbing `app.openFile`
      and using a real `t.TempDir()` file
- [ ] `frontend/wailsjs/go/desktop/App.d.ts` / `App.js` — regenerated via
      `wails build`/`wails dev` after the new `App` methods exist
- [ ] `frontend/src/lib/ingest.ts` — `pickNotesFile`, `importFile`
- [ ] `frontend/src/components/knowledge-section.tsx` — `DropdownMenu`
      replacing the single "Import notes" button; unified `importTarget` state
- [ ] `frontend/src/components/knowledge-section.test.tsx` — clicking
      "Import file..." opens the dialog with `kind="file"`; cancelling either
      picker does nothing
- [ ] `frontend/src/components/ingest-progress-dialog.tsx` — `kind`/`path`
      props replacing `folderPath`
- [ ] `frontend/src/components/ingest-progress-dialog.test.tsx` — existing
      cases updated with `kind="folder"`; core cases (start-on-open, done,
      error, listener cleanup, stale-rejection on reopen) duplicated for
      `kind="file"`
- [ ] `CHANGELOG.md` — `[Unreleased] > Added` entry for single-file import

## Acceptance Criteria

- Picking a single `.md` or `.txt` file imports exactly that file: it is
  chunked, embedded, and produces one shadow `knowledge.Item`, identically to
  how that same file would be processed inside a folder import
- Re-importing the same unchanged file reports it as skipped; editing it and
  re-importing replaces its chunks and updates its existing shadow Item in
  place, exactly as 2.3 already guarantees for a folder import
- Picking a file with an unsupported extension is rejected before any
  chunking/embedding happens
- A single-file import is blocked by the same `CheckMutationAllowed` guard as
  a folder import, and surfaces `IndexingWarning`/`IndexWarnings` the same way
  when vector-store reconciliation fails after a durable write
- The "Import notes" toolbar control offers both "Import folder..." and
  "Import file..." as a dropdown, and cancelling either OS picker leaves the
  Explorer state untouched
- The same progress dialog drives both flows end to end (progress events,
  result summary, failures list, manual close) — a single-file import shows
  `1 of 1 files` rather than a separate UI
