# Phase 2.20 — Import Review & Formatting

## Goal

Every file imported through the Study session's Sources panel now passes through a review
step before anything is written: Athena reformats the note into clean Markdown to improve
chunking quality, previews the exact chunks that reformatting will produce, and lets the user
approve or cancel before any embedding call or database write happens. Today's **Add** goes
straight from the file picker to a silent, synchronous import (2.17); this spec puts a Review
dialog in between.

The approved visual design — the Review dialog's two tabs (Formatted note / Chunks), the
expandable chunk cards, the Cancel/Approve footer — is this private mockup, built and iterated
on with the user before this spec was written: https://claude.ai/artifact/GQ9WziKL3HG38QayYGeBCq

## Why this is a separate spec

This rewires the Sources panel's **Add** flow (2.17) and adds two new `ingest.Service` use
cases, but it does not change the chunking algorithm (2.3.1) or the persistence logic inside
`ingestFile` (2.4.1) — those are reused as-is (chunking) or refactored for reuse without a
behavior change (persistence). It is tracked as its own spec because it is a genuinely new user
flow (a review-and-approve gate) with its own decisions, not an amendment to any of those three
contracts.

## The problem

Diagnosed across 2.5.2/2.19: a note's raw structure (run-on prose, ad-hoc lists, inconsistent
headings, PDF export artifacts) directly controls chunking quality, and today the user only
finds out how a file chunked *after* it is already imported — by opening the Knowledge
item/source viewer, if they think to. 2.19 tried to mitigate this passively, with a downloadable
template showing the structure chunking works best with. This spec is the active counterpart:
reformat the note automatically, and let the user see and approve the result — and the chunks
it produces — before committing it.

## Decisions

1. **Every Add now goes through Preview → Review dialog → Confirm.** No file is embedded or
   persisted before the user explicitly clicks **Approve & import**. Cancelling, or just closing
   the dialog, leaves the knowledge base exactly as it was — there is nothing to roll back,
   because nothing was written yet.
2. **Formatting is one more `domainllm.Provider.Chat` call**, routed under a new task type,
   `TaskNoteFormatting`. Called directly from `ingest.Service`, the same way it already calls
   `s.llm.Embeddings` — no new domain port. Unlike `TaskQueryEnrichment` (2.5.1's HyDE passage,
   hidden from the user, routed cheap), this text is persisted and read by the user, so it is
   routed `TierMedium`, the same tier as `TaskStudy`.
3. **Extension-aware prompt.** `.md` files are asked for clean Markdown (normalized H1-H3
   headings, one paragraph per sub-topic, proper bullet syntax). `.txt` files are asked for
   well-structured plain-text paragraphs only, with no Markdown syntax — `ChunkText` never
   interprets headings, so asking for them on a `.txt` file would just leave literal `##`
   characters sitting in the stored text.
4. **The chunks previewed are byte-for-byte the chunks persisted.** `PreviewImport` chunks the
   *formatted* text with the existing `ChunkMarkdown`/`ChunkText` (2.3.1), completely unchanged.
   `ConfirmImport` never re-reads or re-formats the file — it persists exactly the approved text
   the frontend sends back. This is the one invariant the whole feature's trust depends on: what
   the user reviewed is what gets embedded.
5. **No embeddings during preview.** `PreviewImport` costs one Chat call plus local chunking —
   cheap and fast. The per-chunk embedding calls (today's real cost/latency in `ImportFile`)
   happen exactly once, during `ConfirmImport`, identically to today's `ingestFile`.
6. **The existing skip-unchanged dedup check runs during Preview**, before the formatting call.
   An unchanged file (same mtime, same embedding model as its last import) never pays for a
   reformatting call and the Review dialog never opens for it — the panel just reloads silently,
   the same silent skip behavior `ImportFile` has today.
7. **Formatting degrades gracefully.** A `Provider` error (missing/invalid key, no credits,
   network) or a blank response falls back to the original, unformatted text —
   `FormattingFailed: true` on the preview. Preview never blocks the import over a formatting
   failure; the dialog shows the original text with an inline notice, and the rest of the
   pipeline (chunking, the eventual embeddings) proceeds normally against it.
8. **The "formatting applied" summary is deterministic structural counts, never the model's own
   narrated claims.** The mockup's illustrative copy ("merged 6 run-on paragraphs", "stripped
   repeated page-break artifacts") is exactly the kind of claim the pipeline has no way to verify
   — a model self-reporting its own edits on a screen whose whole point is trust is a correctness
   risk, not a feature. v1 instead computes and shows real counts — headings before/after,
   paragraphs before/after, chunks produced — from the same goldmark/paragraph-splitting code
   `chunking.go` already uses. **Flagging this explicitly: it's a scope cut from what the mockup
   showed; revisit only with a verifiable source for each claim, not a richer prompt.**
9. **No index-mutation reservation is held across the dialog.** `ImportFile` holds
   `BeginMutation`/`EndMutation` for its whole synchronous call. `PreviewImport` and
   `ConfirmImport` are two separate round-trips with arbitrary user think-time in between, so
   each reserves the index only for its own short duration (`PreviewImport` only needs a
   point-in-time `CheckMutationAllowed`, since it writes nothing) — an abandoned review (closed
   dialog, closed app) can never leave a mutation lock stuck open.
10. **Cancel is a frontend-only no-op.** Nothing was persisted during Preview, so there is
    nothing to undo — closing the dialog just discards the in-memory preview state.
11. **Single file only**, same as 2.17 decision 2. No multi-select or multi-file review here.
12. **The existing `ImportFile` use case and its Wails binding are left in place**, behavior
    unchanged (its internals are refactored for reuse — see below), but it is no longer called
    from the Sources panel's Add button. Removing/deprecating it is out of scope.

## New backend surface

- `domain/llm/router.go`: new `TaskNoteFormatting TaskType`, routed `TierMedium` in `TierFor`.
- `application/ingest/import_file.go`: behavior-preserving refactor — `ingestFile` now only
  reads the raw file and delegates everything from chunking onward to a new `ingestContent(ctx,
  sessionID, candidate, displayPath, mtime, prev, hasPrev, content string) (int, error)`, which
  is today's `ingestFile` body unchanged. `ImportFile`'s existing tests keep passing unmodified —
  this is a characterization-preserving extraction, not a behavior change.
- `application/ingest/preview_import.go` (new):
  - `ChunkPreview{Heading, Content string; CharCount, EstimatedTokens int}` — one would-be
    chunk; `EstimatedTokens` is `CharCount / 4`, the same approximation `minChunkChars`'s comment
    already documents.
  - `FormattingStats{HeadingsBefore, HeadingsAfter, ParagraphsBefore, ParagraphsAfter int}`.
  - `ImportPreview{DisplayPath, OriginalContent, FormattedContent string; FormattingFailed bool;
    Stats FormattingStats; Chunks []ChunkPreview; Skipped bool; SkippedReason string}`.
  - `(s *Service) PreviewImport(ctx, sessionID string, root fs.FS, sourceRoot, filePath string)
    (ImportPreview, error)`: same input validation as `ImportFile`, `index.CheckMutationAllowed`
    (no `BeginMutation`), the same mtime/model skip check `importCandidates` already does, then
    `formatNote` + `ChunkMarkdown`/`ChunkText` over the formatted text to build `Chunks`.
  - `formatNote(ctx, sessionID, content string, isMarkdown bool) (formatted string, failed
    bool)`: one non-streamed `s.llm.Chat` call with the extension-aware system prompt (decision
    3); a `Chat` error or a blank response returns `(content, true)` — mirrors
    `HydeEnricher.Enrich`'s blank-response guard (2.5.1), but degrades instead of erroring.
  - `computeFormattingStats(original, formatted string) FormattingStats`: reuses
    `findHeadingSections`/`splitParagraphs` from `chunking.go`, no new parsing.
- `application/ingest/confirm_import.go` (new): `(s *Service) ConfirmImport(ctx, sessionID
  string, root fs.FS, sourceRoot, filePath, approvedContent string, onProgress func(Progress)
  error) (Summary, error)` — same validation and `BeginMutation`/`EndMutation` span as
  `ImportFile`, but a single candidate, calling `ingestContent` directly with `approvedContent`
  instead of reading the file. Reports progress/summary the same shape `ImportFile` does, so the
  frontend's existing `IngestSummary` handling needs no change.
- `internal/interfaces/desktop/ingest.go`: `ImportPreviewResult`/`ChunkPreviewResult`/
  `FormattingStatsResult` DTOs; `PreviewImport(sessionID, selectedPath string)
  (ImportPreviewResult, error)` and `ConfirmImport(sessionID, selectedPath, approvedContent
  string) error`, both opening the same `os.Root` around the picked file's parent directory
  `ImportFile` already does — `ConfirmImport` emits the same `ingest:progress`/`ingest:done`/
  `ingest:error` events. `wailsjs` regenerated.

## Frontend design

- **`lib/ingest.ts`**: `ChunkPreview`, `FormattingStats`, `ImportPreview` types;
  `previewImport(sessionId, path): Promise<ImportPreview>`; `confirmImport(sessionId, path,
  formattedContent): Promise<void>` — fires `ConfirmImport` and resolves the same way
  `importFile` does, relying on the existing `onIngestProgress`/`onIngestDone`/`onIngestError`.
- **`import-review-dialog.tsx`** (new): `{ open, preview, onCancel, onApprove(formattedContent)
  }`. Formatted-note tab: original vs. formatted side by side. Chunks tab: one expandable card
  per `ChunkPreview` (badge index, heading, char/token count, collapsed preview / full text
  toggle) — the interaction validated in the mockup. `preview.formattingFailed` renders an
  inline notice instead of the original/formatted comparison. Footer: **Cancel** (calls
  `onCancel`, no backend call) / **Approve & import** (calls `onApprove`).
- **`study-sources-panel.tsx`**: `handleAddClick` picks the file, then calls `previewImport`;
  while in flight, **Add** stays disabled and a new `FormattingNoteDialog` (a small, non-
  dismissible loading modal — nothing to cancel yet) is open, closing itself the instant the
  same state update either opens `ImportReviewDialog`, finishes a silent skip, or surfaces an
  error. An inline-only status text was tried first and revised after review: it was too easy to
  miss, leaving the user unsure anything was happening. `preview.skipped` reloads silently, same
  as today's silent skip. Otherwise opens `ImportReviewDialog`. **Approve** stores the approved
  text and opens the existing `IngestProgressDialog`; **Cancel** clears the preview state.
- **`ingest-progress-dialog.tsx`**: gains an optional `formattedContent?: string` prop — when
  set, its effect calls `confirmImport(sessionId, path, formattedContent)` instead of
  `importFile(sessionId, path)`. Everything else (progress/summary rendering, the
  non-cancellable/non-dismissible-until-finished behavior) is unchanged.

## Tasks

Each slice keeps the build green and is committed on its own; backend slices are TDD with
`_test.go`, frontend slices are TDD with Vitest.

- [x] `domain/llm`: `TaskNoteFormatting`, routed `TierMedium`
- [x] `application/ingest`: extract `ingestContent` from `ingestFile` (characterization —
      `ImportFile`'s existing tests must still pass unmodified)
- [x] `application/ingest`: `formatNote` (Chat call, blank/error fallback, extension-aware
      prompt) and `computeFormattingStats`
- [x] `application/ingest`: `PreviewImport` (validation, skip check, formatting, chunking)
- [x] `application/ingest`: `ConfirmImport` (validation, mutation guard, `ingestContent`,
      progress/summary)
- [x] Desktop bindings: DTOs, `PreviewImport`, `ConfirmImport`; regenerate `wailsjs`
- [x] `lib/ingest.ts`: types, `previewImport`, `confirmImport`
- [x] `ImportReviewDialog` component (tabs, expandable chunk cards, formatting-failed notice,
      Cancel/Approve) — matches the approved mockup
- [x] `study-sources-panel.tsx`: wire Add through preview → review → confirm
- [x] `IngestProgressDialog`: `formattedContent` prop, calls `confirmImport` when set
- [x] Docs: CHANGELOG, README, `lib/documentation.ts`

## Acceptance Criteria

- Clicking **Add** and picking a file never embeds or persists anything until **Approve &
  import** is clicked; cancelling or closing the Review dialog leaves the Sources panel and the
  knowledge base exactly as they were.
- The Review dialog's Formatted-note tab shows the text Athena will store; its Chunks tab lists
  every chunk `ConfirmImport` will create, each expandable to its full text.
- Approving imports exactly the reviewed/approved text: the stored document (what `SourceViewer`
  later shows) matches the Formatted tab, and the persisted chunks match the Chunks tab
  one-for-one — same count, same boundaries, same content.
- An unchanged file (same mtime, same embedding model as its last import) is skipped before any
  formatting call, exactly as today — no Review dialog opens for it.
- A formatting-provider failure never blocks the import: the dialog shows the original text with
  an inline notice, and Approve still imports it, unformatted.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  application/domain code; frontend tests, coverage, lint, typecheck, and Stryker on the changed
  files pass.

## Known risks

- **The approved text can drift from the file on disk** if it changes between Preview and
  Confirm (an external edit while the dialog sits open). v1 does not detect or warn about this —
  `ConfirmImport` persists the approved text regardless.
- **Formatting quality depends entirely on the configured OpenRouter model and prompt**; a bad
  rewrite is only caught if the user actually reads the Formatted tab before approving. This
  spec deliberately never auto-approves or skips the review for a "safe-looking" note.
- **The deterministic `FormattingStats` (decision 8) are a scope cut** from the mockup's richer,
  narrative checklist copy — revisit only with a verifiable source for each claim.

## Out of scope

- Multi-file import/review.
- Editing the formatted text by hand before approving (v1 is accept-the-rewrite-or-don't, not a
  text editor).
- Any detection/warning if the source file changes on disk between Preview and Confirm.
- Removing or deprecating the direct `ImportFile` path.
- A persistent, post-import "browse this document's chunks" screen — `SourceViewer` still only
  highlights a cited chunk transiently; this spec's chunk list only exists pre-import, in the
  Review dialog.

## References

- Review flow mockup (private): https://claude.ai/artifact/GQ9WziKL3HG38QayYGeBCq
- [03-01-chunking-by-paragraph.md](03-01-chunking-by-paragraph.md) — the chunker reused
  unchanged by `PreviewImport`.
- [04-01-import-file.md](04-01-import-file.md) — the `ImportFile` pipeline this extends and
  partially refactors (`ingestFile` → `ingestFile` + `ingestContent`).
- [05-01-hyde-query-enrichment.md](05-01-hyde-query-enrichment.md) — the Chat-call-with-
  blank-response-guard pattern `formatNote` mirrors (and deliberately diverges from: fallback
  instead of error).
- [17-session-sources-panel.md](17-session-sources-panel.md) — the Add flow this rewires.
- [19-example-note-template.md](19-example-note-template.md) — the other, passive mitigation
  for the same chunking-quality-of-input problem.
- `internal/application/ingest/import_file.go` — `ingestFile`, refactored into `ingestFile` +
  `ingestContent`.
