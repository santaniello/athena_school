# Phase 2.19 — Example Note Template Download

## Goal

Give users a concrete, downloadable example of the note structure Athena's
chunking pipeline works best with — one paragraph per sub-topic inside a
heading section — so future imported notes are less likely to hit the
topic-dilution problem chunking-by-paragraph (2.3.1) cannot fix on its own:
a note whose author bundles several ideas into one continuous paragraph.

## Why this is a separate spec

This is a standalone, self-contained UI increment (a button, a static
markdown template, a save-file flow) with no domain or retrieval logic of
its own — it doesn't amend 05's retrieval contract or 2.3.1's chunking
contract, it only tries to shape the input they receive. Tracked as its own
numbered spec per project convention, not folded into either of those.

## The problem

Diagnosed while validating 05-02's recalibration against synthetic notes
(see [[project_strict_notes_sufficiency_threshold_too_strict]]): a note
written as continuous prose — several sub-topics bundled into one
paragraph, with no blank line separating them — produces one diluted chunk
covering all of them, the same way the original cache note's heading did
before 2.3.1. Chunking-by-paragraph only helps when the note's own author
already separates sub-topics into distinct paragraphs; it has no way to
infer that separation from prose. There is currently no guidance anywhere
in the product telling a user this matters, or what "good" looks like.

## Decision

A **"Download example"** button in the Study session's Sources panel
header, next to **Add**, that saves a small bundled markdown file
demonstrating the pattern: a heading section whose sub-topics are each
their own paragraph, plus a short explanation of why that matters, written
as part of the example note itself (so it's self-explanatory without
needing separate in-app documentation).

**Scope, explicitly bounded.** This is a passive mitigation: it only helps
a user who reads the template and follows the convention when writing
*future* notes. It does not change any already-imported note, and does not
warn when a newly-imported note doesn't follow the pattern. Detecting that
automatically (e.g. flagging a long paragraph that bundles multiple
concepts) is a different, larger feature with its own detection logic —
out of scope here, tracked as a possible follow-up.

**Mechanism.** The existing OS-native save dialog pattern already used by
`PickNotesFile`'s open dialog, mirrored for a save: `wailsruntime.
SaveFileDialog`, a new `saveFile` field on `desktop.App` (same
test-injectable-fake pattern as `openFile`/`emit`/`reloadApp` — no new
`NewApp` constructor parameter). The example content is a plain Go string
constant, not a file on disk — no `go:embed`, no new static asset
pipeline, for one small piece of static text.

## Implementation shape

- `internal/interfaces/desktop/example_note.go` (new): `exampleNoteMarkdown`
  constant and `App.DownloadExampleNote() (string, error)` — opens the save
  dialog pre-filled with `example-note.md`, writes the constant to the
  chosen path via `os.WriteFile`, returns the chosen path or `""` on
  cancellation. No session/backend-service involvement; this never touches
  `ingest`/`knowledge`.
- `internal/interfaces/desktop/app.go`: new `saveFile func(ctx
  context.Context, options wailsruntime.SaveDialogOptions) (string, error)`
  field, defaulting to `wailsruntime.SaveFileDialog` in `NewApp` — same
  shape as the existing `openFile`/`emit`/`reloadApp` fields.
- Wails bindings (`frontend/wailsjs/go/desktop/App.{js,d.ts}`): regenerated
  via `wails generate module` to pick up the new exported method.
- `frontend/src/lib/ingest.ts` (or a new small `lib` file): a thin
  `downloadExampleNote()` wrapper around the generated binding, mirroring
  `pickNotesFile`.
- `frontend/src/components/study-sources-panel.tsx`: a new button in the
  header next to **Add**, `variant="outline"`, calling the wrapper. Any
  error surfaces the same way the existing picker error does (inline text
  under the search box) — no new error-display mechanism.
- Tests: Go — dialog receives the exact title/default filename, the exact
  content is written to the chosen path, cancellation returns `""` with no
  write, a dialog/write error propagates. Frontend — button renders, click
  invokes the binding, an error from it surfaces inline.

## Known risks

- Passive mitigation only; see "Scope, explicitly bounded" above. A user
  who never opens the template gets no benefit, and an already-imported
  prose-heavy note is unaffected.
- The example's own length/structure could itself drift out of date if
  chunking's rules change again later — it is a plain string constant, not
  generated from the real chunking code, so nothing enforces that it still
  demonstrates the current rules.

## Out of scope

- Any automatic detection/warning for a newly-imported note that doesn't
  follow this structure.
- Any in-app documentation/help beyond the example file itself.
- Editing or validating an already-imported note's structure.

## References

- [[project_strict_notes_sufficiency_threshold_too_strict]] (assistant
  memory) — the validation sweep that surfaced this gap.
- [03-01-chunking-by-paragraph.md](03-01-chunking-by-paragraph.md) — the
  chunking behavior this template is written to work well with.
- `internal/interfaces/desktop/ingest.go` — `PickNotesFile`, the existing
  open-dialog pattern this mirrors for a save dialog.
