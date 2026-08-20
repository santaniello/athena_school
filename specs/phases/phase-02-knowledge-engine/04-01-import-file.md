# Phase 2.4.1 — Single-File Import

## Goal

Today `ImportFolder` (2.3) is the only entry point into the notes pipeline: the
user must pick a directory even to bring in exactly one note. This spec adds a
second entry point for importing one `.md`/`.txt` file directly. Both entry
points share candidate processing, chunking, embedding, transactional
replacement, progress reporting, and `IndexWarnings`.

The feature also closes two pre-existing correctness gaps that become unsafe
once arbitrary single files can be selected:

- a root-relative path cannot be the global identity of an imported file —
  `/course-a/notes.md` and `/course-b/notes.md` must not collide, and the same
  file imported individually or through different folder roots must not be
  duplicated;
- modification times stored with one-second precision can miss a rapid edit.

Folder import therefore keeps its user-visible behavior, but its persisted
source identity changes to a canonical absolute path and its mtime precision
changes to nanoseconds. This spec is not strictly additive internally.

## Dependencies

- 2.3 supplies the pipeline this spec reuses: chunking, shadow Items,
  `IngestedFile` dedup, and transactional replace-per-file behavior inside
  `ingestFile`.
- 2.4 supplies the full-operation mutation reservation and vector-store
  reconciliation (`BeginMutation`/`EndMutation`, `IndexingWarning`, and
  `IndexWarnings`).

## Source identity and stored path

An imported source has two distinct paths:

- **`SourcePath`** is its internal identity: the absolute path normalized by
  the desktop adapter with `filepath.Abs`, `filepath.Clean`, and
  `filepath.ToSlash`. It is never shown in progress, failures, or index issue
  UI. Case is preserved and symlinks are not resolved: two selected symlink
  aliases are distinct locations. Editing or replacing content at the same
  location updates the same Item; moving, renaming, or copying a file creates
  a new source.
- **`FilePath`** is the root-relative `/`-separated path captured on the first
  import. It remains the display path and the input to `BuildShadowItem` on
  every re-import, even when the same source is later reached through another
  folder root. This keeps its Topic stable instead of moving the Item according
  to whichever entry point happened to process it most recently.

For example, the source `/notes/go/concurrency.md` has one `SourcePath`
regardless of whether it is selected directly, found while importing `/notes`,
or found while importing `/notes/go`. Its first import fixes `FilePath`:

- imported directly (equivalent to importing its immediate parent),
  `FilePath = "concurrency.md"`, so Topic falls back to the H1 and then the
  basename;
- first imported from `/notes`, `FilePath = "go/concurrency.md"`, so Topic is
  `go`.

A later import through the other entry point keeps that first `FilePath` and
Topic. `Concept` and `Definition` still follow the existing source-authoritative
rule: when the file changes, they are derived again and can overwrite manual
edits to the shadow Item.

The desktop boundary normalizes the selected root because it owns OS-specific
path handling. The application service continues to accept `fs.FS` and never
opens arbitrary OS paths. It receives the normalized root as an identity
prefix and combines it with each valid relative candidate using `path.Join`.

## Pre-release schema correction

There are no deployed or local knowledge records that must survive this
change. No data cleanup, heuristic backfill, archive, or backup is required.
The pre-release schema becomes:

```sql
CREATE TABLE knowledge_chunks (
    -- existing columns omitted
    source_path TEXT, -- canonical absolute identity; set for imported_doc
    file_path   TEXT  -- stable first-import relative/display path
);

CREATE INDEX idx_knowledge_chunks_source_path
    ON knowledge_chunks(source_path);

CREATE TABLE ingested_files (
    source_path      TEXT PRIMARY KEY,
    file_path        TEXT NOT NULL,
    mtime_unix_nano  INTEGER NOT NULL,
    embedding_model TEXT NOT NULL,
    chunk_count      INTEGER NOT NULL,
    item_id          TEXT NOT NULL,
    ingested_at      DATETIME
);
```

`domainknowledge.Chunk` gains `SourcePath`; `FilePath` retains its current
display/provenance meaning. `ChunkRepository.DeleteByFilePath` becomes
`DeleteBySourcePath`, so replacement cannot delete an unrelated same-named
file. `domainknowledge.IngestedFile` gains `SourcePath`, keeps `Path` as the
stable first-import relative path, and replaces ambiguous `MTime` with
`MTimeUnixNano`. `IngestedFileRepository.ListAll` returns a map keyed by
`SourcePath`.

Fresh databases use the definitions above. A structural migration may rebuild
the empty pre-release `ingested_files` table and add `source_path` to the empty
`knowledge_chunks` table when an older schema is detected. It must first verify
that the affected tables contain no rows and fail rather than delete data if
that premise is violated. The migration is idempotent on the next `Open`.

## Use case

`internal/application/ingest/import_folder.go` introduces a private candidate
that separates the path used to read the current `fs.FS` from the source key:

```go
type importCandidate struct {
    ReadPath   string // relative to the fs.FS used for this invocation
    SourcePath string // canonical absolute identity, never displayed
}
```

Both public entry points own the full 2.4 mutation reservation. The shared
helper assumes that its caller holds it:

```go
func (s *Service) ImportFolder(
    ctx context.Context,
    root fs.FS,
    sourceRoot string,
    onProgress func(Progress) error,
) (Summary, error) {
    if err := s.index.BeginMutation(); err != nil {
        return Summary{}, err
    }
    defer s.index.EndMutation()

    paths, err := collectCandidates(root)
    if err != nil {
        return Summary{}, fmt.Errorf("ingest: scanning folder: %w", err)
    }
    candidates := makeImportCandidates(sourceRoot, paths)
    return s.importCandidates(ctx, root, candidates, false, onProgress)
}

func (s *Service) ImportFile(
    ctx context.Context,
    root fs.FS,
    sourceRoot string,
    filePath string,
    onProgress func(Progress) error,
) (Summary, error) {
    // Input validation deliberately precedes index reservation: an invalid
    // request always reports its own error, even while the index is busy.
    if !fs.ValidPath(filePath) {
        return Summary{}, fmt.Errorf("ingest: invalid file path %q", filePath)
    }
    switch strings.ToLower(path.Ext(filePath)) {
    case ".md", ".txt":
    default:
        return Summary{}, fmt.Errorf("ingest: unsupported file type %q", path.Ext(filePath))
    }

    if err := s.index.BeginMutation(); err != nil {
        return Summary{}, err
    }
    defer s.index.EndMutation()

    candidate := makeImportCandidate(sourceRoot, filePath)
    return s.importCandidates(ctx, root, []importCandidate{candidate}, true, onProgress)
}

func (s *Service) importCandidates(
    ctx context.Context,
    root fs.FS,
    candidates []importCandidate,
    restoreDeletedItem bool,
    onProgress func(Progress) error,
) (Summary, error)
```

`restoreDeletedItem` is the one intentional processing difference between the
entry points:

- folder import preserves 2.3 behavior: an unchanged `IngestedFile` is skipped
  without resurrecting a shadow Item deleted in the Explorer;
- single-file import is an explicit restoration request. Before skipping an
  unchanged source, it checks the recorded `ItemID`. `ErrItemNotFound` forces
  the ordinary `ingestFile` replacement path, which recreates the Item under
  the same ID and rebuilds its chunks. Another repository error is recorded as
  that candidate's failure.

For a source already present in `IngestedFile`, `ingestFile` reads the current
candidate through `ReadPath` but uses the stored `IngestedFile.Path` for
`BuildShadowItem`, `Chunk.FilePath`, progress-independent provenance, and the
next upsert. It uses `SourcePath` for lookup, chunk deletion, and replacement.

`modTime` returns `info.ModTime().UnixNano()`. New, changed, restored, skipped,
failed, and post-commit warning bookkeeping otherwise stays shared. `Progress`,
`Summary`, and `FileFailure` keep their existing shapes, and every displayed
path remains relative. A single-file import reports `FilesTotal == 1`.

Input failures (invalid path/extension or a rejected mutation reservation) are
top-level errors. Once a valid candidate is accepted, stat/read/embed/write
failures appear in the ordinary finished `Summary` with `FilesFailed == 1`.
`IndexingWarning` remains a successful durable import under `IndexWarnings`.

No file-size limit or UTF-8 validation is added here; both entry points retain
the existing pipeline behavior. No symlink resolution, content-hash identity,
operation queue, or operation ID is introduced.

## Desktop binding

`internal/interfaces/desktop/app.go` gains an injectable file-picker field,
mirroring `openDirectory` (the real Wails dialog functions require a Wails
runtime context and cannot be called directly from unit tests):

```go
openFile func(ctx context.Context, options wailsruntime.OpenDialogOptions) (string, error)
```

`NewApp` wires it to `wailsruntime.OpenFileDialog`.

`internal/interfaces/desktop/ingest.go` gains `PickNotesFile` and `ImportFile`.
The filter exposes every casing of `.md` and `.txt` because GTK glob matching
is case-sensitive even though the application rule is not:

```go
func (a *App) PickNotesFile() (string, error) {
    return a.openFile(a.ctx, wailsruntime.OpenDialogOptions{
        Title: "Select a note file",
        Filters: []wailsruntime.FileFilter{
            {
                DisplayName: "Notes (*.md, *.txt)",
                Pattern: "*.md;*.mD;*.Md;*.MD;*.txt;*.txT;*.tXt;*.tXT;*.Txt;*.TxT;*.TXt;*.TXT",
            },
        },
    })
}
```

`ImportNotes` and `ImportFile` normalize their selected directory before
calling `os.OpenRoot`; `filepath.ToSlash` supplies the application-facing
`sourceRoot`. They do not call `filepath.EvalSymlinks`.

```go
func (a *App) ImportFile(selectedPath string) error {
    absolutePath, err := filepath.Abs(filepath.Clean(selectedPath))
    if err != nil {
        a.emit(a.ctx, eventIngestError, err.Error())
        return err
    }

    dir := filepath.Dir(absolutePath)
    root, err := os.OpenRoot(dir)
    if err != nil {
        a.emit(a.ctx, eventIngestError, err.Error())
        return err
    }
    defer func() {
        if closeErr := root.Close(); closeErr != nil {
            log.Printf("closing note import root %q: %v", dir, closeErr)
        }
    }()

    summary, err := a.ingest.ImportFile(
        a.ctx,
        root.FS(),
        filepath.ToSlash(dir),
        filepath.Base(absolutePath),
        func(p ingest.Progress) error {
            a.emit(a.ctx, eventIngestProgress, toIngestProgressResult(p))
            return nil
        },
    )
    if err != nil {
        a.emit(a.ctx, eventIngestError, err.Error())
        return err
    }
    a.emit(a.ctx, eventIngestDone, toIngestSummaryResult(summary))
    return nil
}
```

`ImportNotes` passes the picked folder's normalized absolute path as
`sourceRoot` to `ImportFolder`. Empty picker results never reach either import
binding through the normal UI.

No new events or DTOs are added. `IngestProgressResult` and
`IngestSummaryResult` are shared unchanged. The global
`ingest:progress`/`ingest:done`/`ingest:error` channels deliberately support one
active import only. The modal enforces that invariant in normal UI use, and
`BeginMutation` rejects overlapping backend operations; operation IDs and an
import queue remain out of scope.

## Frontend

`frontend/src/lib/ingest.ts` adds thin wrappers matching
`pickNotesFolder`/`importNotes`:

```ts
export async function pickNotesFile(): Promise<string> {
  return PickNotesFile()
}

export async function importFile(path: string): Promise<void> {
  await ImportFile(path)
}
```

`frontend/src/components/knowledge-section.tsx` replaces the single
**"Import notes"** button with the existing `DropdownMenu` component:

```text
[ Import notes ▾ ]
   ├─ Import folder...
   └─ Import file...
```

Its target state becomes:

```ts
const [importTarget, setImportTarget] = useState<{
  kind: 'folder' | 'file'
  path: string
} | null>(null)
```

The section also owns one inline picker-error state. Starting either picker
clears the preceding picker error; a rejection shows
`"Failed to open the notes picker. Please try again."`; cancellation (`""`)
does nothing to the Explorer and starts no import. Both menu items remain
disabled while `mutationsDisabled` is true.

`frontend/src/components/ingest-progress-dialog.tsx` generalizes its props from
`folderPath` to `kind: 'folder' | 'file'` plus `path`, then calls
`importNotes(path)` or `importFile(path)` inside the existing open/cleanup
effect. Finished summary, failures, `IndexWarnings`, stale-rejection defense,
listener cleanup, and manual close are shared. The in-progress description is:

- folder: `"Processing files in the selected folder."`
- file: `"Processing the selected file."`

There is no cancel affordance, as in 2.3. A single-file import uses the same
progress copy and shows `1 of 1 files`; no singular-only dialog is introduced.
`KnowledgeExplorerScreen` and `KnowledgeTopicTree` already reload from the
shared `ingest:done` event, so no new refresh mechanism is needed.

## Implementation order (TDD)

Each item below is one Red → Green → Refactor cycle; do not implement the next
behavior before the current cycle is green.

1. [ ] Canonical imported-source persistence
   - tests first in `internal/infrastructure/sqlite/db_test.go`,
     `chunk_repository_test.go`, and `ingested_file_repository_test.go`;
   - update `knowledge_chunks`/`ingested_files`, `knowledge.Chunk`,
     `IngestedFile`, repository methods, validation, and generated mocks;
   - prove two identical relative names under different absolute sources
     coexist and deletion targets only one `SourcePath`.
2. [ ] Existing folder import adopts canonical identity and nanosecond mtime
   - tests first in `internal/application/ingest/import_folder_test.go` and
     `internal/interfaces/desktop/ingest_test.go`;
   - pass normalized `sourceRoot`, preserve first-import `FilePath`, and prove
     the same physical path reached from two roots is one source;
   - prove two mtimes within one second but with different nanoseconds trigger
     replacement.
3. [ ] Application single-file import
   - tests first in a new `internal/application/ingest/import_file_test.go`;
   - cover new file, unchanged skip, changed replacement, case-insensitive
     extension, validation-before-reservation, full reservation release,
     one-entry progress/summary, per-file failure, and `IndexingWarning`;
   - cover explicit restoration of an unchanged deleted Item while folder
     import continues to skip that case;
   - extract only the candidate-processing code required by both flows.
4. [ ] Desktop picker and binding
   - tests first in `internal/interfaces/desktop/ingest_test.go` and
     `app_test.go`;
   - assert the exact title and case-complete filter, cancellation/error
     passthrough, normalized source root, one real `t.TempDir()` file, emitted
     progress/done/error events, and injected `openFile` wiring;
   - add `PickNotesFile`/`ImportFile`, then regenerate
     `frontend/wailsjs/go/desktop/App.d.ts` and `App.js` via Wails.
5. [ ] Frontend binding wrappers
   - tests first in `frontend/src/lib/ingest.test.ts`;
   - add `pickNotesFile` and `importFile` with exact binding delegation.
6. [ ] Import dropdown and picker errors
   - tests first in `frontend/src/components/knowledge-section.test.tsx`;
   - cover both menu actions, both cancellations, both picker failures, error
     clearing, disabled state, and the unified target.
7. [ ] Shared progress dialog
   - update existing folder tests first, then add file-kind cases in
     `frontend/src/components/ingest-progress-dialog.test.tsx`;
   - cover start-on-open, description, done, processing error, binding
     rejection fallback, warnings, listener cleanup, manual close, and stale
     rejection after reopening for both kinds without redundant assertions.
8. [ ] Documentation
   - add the single-file import feature under `[Unreleased] > Added` in
     `CHANGELOG.md`;
   - update `README.md` Phase 2 wording from Markdown-only folder import to
     Markdown/plain-text folder or single-file import.

After the cycles are green, run the complete quality gate required by
`AGENTS.md`: coverage at least 80%, `make mutation-go` for the changed domain
and application code (and vector-store scope if touched), frontend mutation
testing for changed frontend logic, lint, vulnerability checks, and then the
Conventional Commit.

## Acceptance Criteria

- Picking one `.md`/`.txt` file, with any extension casing supported by the
  platform picker, imports exactly that file and produces the same chunks,
  embeddings, shadow Item fields, progress shape, and index reconciliation as
  the shared pipeline.
- `/course-a/notes.md` and `/course-b/notes.md` coexist as distinct sources;
  importing `/notes/go.md` individually and through any folder root resolves to
  one source and one stable Item ID.
- The first import's relative `FilePath` remains the display/provenance and
  topic-derivation path on later imports through another entry point. Absolute
  `SourcePath` is never exposed in progress, failures, or index issue UI.
- Re-importing an unchanged source with an existing Item reports it as skipped.
  A nanosecond-level mtime change replaces its chunks and updates that Item in
  place.
- If an unchanged source's Item was deleted, direct single-file import rebuilds
  it under the recorded ID and counts it as ingested; folder import continues
  to skip it until the file changes.
- Invalid paths and unsupported extensions are rejected before
  `BeginMutation`, repository access, chunking, or embedding. Both public entry
  points hold `BeginMutation`/`EndMutation` for their complete operations.
- Failure after accepting the candidate produces an `ingest:done` summary with
  one failed file. A request-level failure emits `ingest:error`.
  Post-commit vector reconciliation failure remains a successful ingest with
  one `IndexWarning`.
- The toolbar dropdown offers **Import folder...** and **Import file...**.
  Cancellation leaves Explorer state untouched; failure to open either picker
  produces the shared inline error.
- The same progress dialog drives both flows end to end. A single-file import
  displays `1 of 1 files`, and the existing Explorer and topic tree refresh on
  `ingest:done`.
- Only one import is active through the UI. No operation IDs, queue, cancel
  affordance, file-size limit, UTF-8 validation, symlink resolution, or content
  hashing are introduced.
