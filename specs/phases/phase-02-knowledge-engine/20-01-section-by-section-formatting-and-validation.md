# Phase 2.20.1 — Section-by-Section Formatting, Validation, and Chunk Provenance

## Goal

2.20 shipped `PreviewImport`'s single `formatNote` call: the whole document, one `Chat`
request, accept-if-non-empty. An external evaluation against 11 real notes (`know-improvements.md`,
not committed — a local artifact) found that contract failing in practice: 7 of 11 notes (63.6%)
hit the OpenRouter client's fixed 60s timeout and fell back to the *entire* unformatted document;
one note was silently translated PT → EN; and the one case the formatting prompt was specifically
tuned for (2.20's `TestNoteFormattingPrompts_coverBundledListDefinitions`, the LRU/LFU/FIFO/RR
list) still produces an unpredictable 1-4 chunk split across repeated runs. This spec replaces the
single whole-document call with section-by-section formatting plus deterministic, non-LLM
validation of each result, and gives the chunker breadcrumb-aware headings so a generic section
title ("Vantagens") stops losing its parent topic.

## Why this is a follow-up spec, not an amendment to 2.20

2.20's decisions (Preview → Review → Confirm gate, byte-exact reviewed-is-persisted invariant,
deterministic-stats-not-narration) all still hold — this spec only changes *how* the formatting
step inside `PreviewImport` produces its text, and adds validation and chunk-provenance
improvements around it. `ConfirmImport`/`ingestContent`'s contract is unchanged.

## The problem, diagnosed

- **Whole-document formatting is an all-or-nothing timeout gamble.** `openrouter.Client`'s HTTP
  timeout (60s, `client.go`) is fixed. A note large enough to risk it loses *all* formatting on
  timeout, not just the slow part — and size alone does not predict it (a 13KB note finished in
  51s; a 12.9KB note timed out), so raising the timeout only makes failures slower, not rarer.
- **The only validation is "did the response come back non-empty."** `formatNote`'s
  `cleaned == ""` check cannot catch a fluent, non-empty response that silently translated the
  note, dropped a number, or left a named list merged into one block — the model can report
  nothing wrong while doing all three.
- **Chunk headings are flat by design (2.3.1), which loses context under generic titles.** A
  section titled "Vantagens" nested under "Microsserviços" stores `Heading: "Vantagens"` with no
  record of which architecture it was about — fine for a uniquely-named heading, lossy for a
  repeated generic one.
- **The LRU/LFU/FIFO/RR case (2.20's own motivating example) was never fully fixed, and its
  partial fix is unstable.** The restated-context prompt change (`36aee2a`, pre-2.20.1) reliably
  produced 2 chunks instead of 4 in one verified run, but repeated runs against the same prompt
  sometimes produce 1 chunk, sometimes 2, with a different split each time — an accepted,
  documented residual (see `03-01-chunking-by-paragraph.md`), not a new discovery. **Decision,
  made with the user:** do not chase full 4-way separation by lowering the global
  `minChunkChars` floor or adding a named-list-specific chunking rule — the instability's root
  cause is the model's own rewrite variance, which a looser floor would not fix. Instead: mitigate
  the generic-heading symptom (breadcrumb-prefixed embeddings, decision 7) and make the
  still-collapsed case *visible* rather than silently reported as success (decision 6).

## Decisions

1. **Format one heading-bounded section at a time, batched under a character budget, not the
   whole document in one call.** A new `splitIntoFormattingSections(source) []FormattingSection`
   reuses `findHeadingSections`'s boundary walk (now breadcrumb-aware, decision 8) to cut the
   document the same way chunking does, before any paragraph-packing or undersized-merging — a
   document with no H1-H3 heading at all (every `.txt` file included) falls back to
   paragraph-scoped sections via `splitParagraphs`, so a large heading-less note still splits
   instead of becoming one oversized call.
2. **Sections are packed into batches under `maxFormattingChars` (6000 runes), never splitting a
   section mid-way.** `packFormattingBatches(sections) []FormattingBatch` greedily groups adjacent
   sections; a single section already over budget on its own still becomes its own (over-budget)
   batch — the same "never drop/split" philosophy `mergeUndersized`/`packParagraphs` already use,
   applied to the formatting stage instead of the chunking stage. Most notes stay one or two
   batches; a timeout's blast radius becomes one section, never the whole document.
3. **Each batch is formatted independently, with its own fallback.** `formatBatch` replaces
   `formatNote`'s whole-document call with the same contract — one non-streamed `Chat` call,
   `(content, true)` on error or blank response — scoped to one batch's own text.
   `formatBatches` runs this over every batch in document order, so one batch's failure degrades
   only that batch, never its neighbors.
4. **A batch's section breadcrumb is folded into the system prompt as context, never into the
   user message.** `formatBatch(ctx, sessionID, breadcrumb, content, isMarkdown)` appends a
   one-sentence context note to the system prompt when `breadcrumb != ""` ("this excerpt is part
   of a larger note, under the section X — use that only as context ... never repeat it verbatim
   inside your output"). Putting it in the user message instead risked the model echoing it back
   as if it were part of the note's own content.
5. **Reassembly preserves 2.20's byte-exact invariant by construction, not by convention.**
   `reassembleFormattedContent(source, outcomes)` concatenates each batch's `Formatted` text in
   order, re-inserting the *original* gap text between batches
   (`source[prevBatch.End:nextBatch.Start]`, whatever whitespace that was) rather than a fresh
   separator. When every batch falls back, the result is byte-identical to the original document
   sliced from the first batch's start to the last batch's end — proven by
   `TestReassembleFormattedContent_allBatchesFallback_isByteExactToTheOriginalSpan`.
6. **`FormattingFailed` is now "did any batch fall back," computed from real outcomes, not a
   single whole-document flag.** `anyBatchFellBack(outcomes)` ORs every batch's own fallback —
   semantically the same field the Review dialog already reads, just sourced from granular data.
   Per-batch failure detail (`FailedSections`) and the structural-health signal for the
   still-collapsed-list case (`StructureWarnings`) are also delivered in this same PR — see
   decision/surface sections below and the Frontend design section for how the Review dialog
   renders them.
7. **The embedding call for every chunk is prefixed with its own heading breadcrumb — the
   `Content` field persisted and shown to the user stays unprefixed.** `ChunkCandidate.
   EmbeddingText()` returns `Heading + "\n\n" + Content` (or `Content` alone when `Heading ==
   ""`), and `ingestContent`'s embedding loop (`import_file.go`) calls it instead of using
   `Content` directly. This mitigates the generic-heading dilution problem (decision 8 below
   fixes the *data* a generic heading now carries; this fixes what the *embedding* does with it)
   without touching the chunker's merge/split rules.
8. **Chunk headings become full breadcrumbs, not each section's own flat title.**
   `findHeadingSections` now tracks a stack of open headings by level (push on a deeper/sibling
   heading, pop back to the right depth on a shallower or sibling one) and joins it with `" > "`
   — e.g. `"Microsserviços > Tradeoffs > Vantagens"` instead of `"Vantagens"`. This reverses
   2.3.1/2.3's explicit "never a breadcrumb" simplification; that choice was made for
   implementation simplicity (~40 lines, per 03-notes-import-and-knowledge-explorer.md), not as a
   load-bearing invariant elsewhere, so reversing it only requires updating the one test that
   asserted it and the spec prose, not any schema or migration (`Chunk.Heading` stays a plain
   string column).

## New/changed backend surface

- `application/ingest/section_formatting.go` (new): `FormattingSection{Breadcrumb, Content
  string; Start, End int}`, `FormattingBatch{Sections []FormattingSection; Start, End int}` (plus
  its `breadcrumb()` helper, which uses the batch's first section), `BatchOutcome{Batch,
  Original, Formatted string; Fallback bool}`, `splitIntoFormattingSections`,
  `packFormattingBatches`, `reassembleFormattedContent`, constant `maxFormattingChars = 6000`.
- `application/ingest/preview_import.go`: `formatNote` → `formatBatch` (per-batch, breadcrumb
  param) + `formatBatches` (the per-batch orchestration loop, with an `onSectionFormatted
  func(processed, total int) error` progress hook) + `anyBatchFellBack`. `PreviewImport` gains a
  trailing `onProgress func(Progress) error` parameter (nil-safe), wired to
  `onSectionFormatted` via a small closure building `Progress{SectionsProcessed, SectionsTotal}`.
- `application/ingest/import_file.go`: `Progress` gains `SectionsProcessed`/`SectionsTotal` (0 on
  every `ImportFile`/`ConfirmImport` tick); the embedding loop in `ingestContent` calls
  `chunkCandidate.EmbeddingText()` instead of `chunkCandidate.Content`.
- `application/ingest/chunking.go`: `findHeadingSections` tracks a heading stack instead of a
  flat current title; `ChunkCandidate.Heading` is the joined breadcrumb; new
  `ChunkCandidate.EmbeddingText()` method (decision 7).
- `internal/interfaces/desktop/ingest.go`: new event `eventIngestFormattingProgress =
  "ingest:formatting-progress"`; `IngestProgressResult` gains `SectionsProcessed`/
  `SectionsTotal`; `App.PreviewImport` passes a callback emitting that event per batch. No change
  to `PreviewImport`'s own Wails-exposed signature (`(sessionID, selectedPath string)
  (ImportPreviewResult, error)`), so no `wailsjs` regeneration is needed — only a new runtime
  event name, the same mechanism `ingest:progress` already uses.
- `internal/domain/knowledge/chunk.go` / `retrieval.go`: `Heading` field doc comments updated to
  describe the breadcrumb (no type/schema change).
- `application/ingest/formatting_validation.go` (new): `BatchValidation{Passed bool; Reasons
  []string; LanguageBefore, LanguageAfter string}`, `validateFormattingFidelity`,
  `detectDominantLanguage` (stopword-rate heuristic, no new dependency),
  `extractNumbersPercentsURLsVersions`, `extractCodeBlocks`, `sizeRatio`. Wired into
  `formatBatches`: a batch that fails this check falls back to its own original content, the same
  as a Provider error.
- `application/ingest/structure_validation.go` (new): `StructureCheck{Improved,
  NamedListDetected, StillCollapsed bool; ChunksBefore, ChunksAfter int}`,
  `validateFormattingStructure`, `detectNamedListPattern`. `preview_import.go` gained
  `formatBatchWithCorrection`/`correctiveListInstruction`/`formattingPrompt`/`chatFormat`
  (`formatBatch` now a thin wrapper over the latter two) — `formatBatches` spends exactly one
  corrective retry on a batch whose structure check reports `StillCollapsed`, keeping the retry's
  result only if it both passes fidelity validation and improves the structure verdict.
  `BatchOutcome` gained `Validation BatchValidation` and `Structure StructureCheck`.
- `application/ingest/preview_import.go`: `ImportPreview` gained `FailedSections
  []FailedSection{Heading, Reason string}` and `StructureWarnings []StructureWarning{Heading,
  Message string}`, built from `outcomes` by new `failedSections`/`structureWarnings` helpers.
- `internal/interfaces/desktop/ingest.go`: `FailedSectionResult`/`StructureWarningResult` DTOs
  mirrored onto `ImportPreviewResult`; `toImportPreviewResult` maps both.
- `internal/domain/llm/provider.go`: `EmbeddingBatchRequest{SessionID string; Inputs []string}`,
  `EmbeddingResult{Embedding []float64}`, `EmbeddingBatchResponse{Embeddings []EmbeddingResult;
  Model string; Usage}`; `Provider` gained `EmbeddingsBatch(ctx, EmbeddingBatchRequest)
  (EmbeddingBatchResponse, error)`.
- `internal/infrastructure/openrouter/client.go`: `EmbeddingsBatch` posts `{model, input: []string}`
  to the same `/api/v1/embeddings` endpoint `Embeddings` uses and maps `data` back in order —
  tested against a fake HTTP server, **not yet against the real OpenRouter API** (see Known
  risks). `mock_Provider.go` regenerated via `mockery` for the new interface method.

## Frontend design

- **`lib/ingest.ts`**: new `FormattingProgress{sectionsProcessed, sectionsTotal}` type and
  `onFormattingProgress(handler)`, subscribing to `ingest:formatting-progress`.
- **`formatting-note-dialog.tsx`**: subscribes to its own `onFormattingProgress` ticks (the same
  self-contained pattern `IngestProgressDialog` already uses for `ingest:progress`), showing
  "Formatting section X of Y…" once more than one batch exists; a single-batch note keeps the
  original generic message, since "section 1 of 1" tells the user nothing new. Resets its
  progress state on close.
- **`lib/study.ts`**: new `truncateBreadcrumb(breadcrumb, maxLength = 60)` helper, keeping the
  breadcrumb's last (most specific) segment intact and collapsing earlier ones behind a leading
  "… > " — applied inside `sourceLabel()`'s default-case subtitle, so `citation-chip.tsx` and
  `local-sources-strip.tsx` get the truncation for free with no component changes.
- **`lib/ingest.ts`**: new `FailedSection{heading, reason}`/`StructureWarning{heading, message}`
  types, added to `ImportPreview`.
- **`import-review-dialog.tsx`**: a `failedSections`-aware notice (naming each affected section
  and its reason, replacing the old generic "Formatting is unavailable right now" copy whenever
  at least one section is named) plus a separate alert listing every `structureWarnings` entry
  (shown regardless of `formattingFailed`, since it is informational, not a failure), plus a
  one-line `statsLabel(preview.stats)` ("0 → 1 headings · 1 → 2 paragraphs") shown whenever
  formatting succeeded — the `FormattingStats` counts 2.20 computed but never rendered.

## Tasks

- [x] `application/ingest`: `section_formatting.go` (`FormattingSection`, `FormattingBatch`,
      `splitIntoFormattingSections`, `packFormattingBatches`, `reassembleFormattedContent`)
- [x] `application/ingest`: `chunking.go` breadcrumb-aware `findHeadingSections` +
      `ChunkCandidate.EmbeddingText()`; `import_file.go` embedding loop uses it
- [x] `application/ingest`: `formatBatch`/`formatBatches`/`anyBatchFellBack`, wired into
      `PreviewImport`
- [x] `application/ingest`/`interfaces/desktop`: `Progress.SectionsProcessed/SectionsTotal`,
      `PreviewImport`'s `onProgress` parameter, `ingest:formatting-progress` event
- [x] `lib/ingest.ts`/`formatting-note-dialog.tsx`: `onFormattingProgress`, per-section progress
      message
- [x] `lib/study.ts`: `truncateBreadcrumb`, wired into `sourceLabel`
- [x] `application/ingest`: deterministic fidelity validation (language, numbers/URLs/code
      blocks, size ratio, heading-translation) rejecting a batch's rewrite before it is accepted
- [x] `application/ingest`: structural validation (chunk the batch's own content before/after,
      detect a still-collapsed named list, one bounded corrective retry, surface
      `Improved=false` rather than reporting silent success)
- [x] `interfaces/desktop`/frontend: surface per-batch failure detail and the structural-health
      signal in the Review dialog, alongside the `FormattingStats` counts 2.20 already computes
      but never rendered
- [x] `domain/llm`/`infrastructure/openrouter`: `EmbeddingsBatch` capability (types, `Provider`
      interface method, OpenRouter client implementation, wire-format tests against a fake
      server) — **`ingestContent`'s embedding loop is deliberately NOT switched to use it yet.**
      The capability is implemented per the documented OpenAI-compatible contract (`input` as an
      array, `data` returned in request order), but this was never confirmed against the real
      OpenRouter API in this session (no network/credentials available). Flipping the hot-path
      switch on an unverified assumption would also force rewriting every existing test that
      mocks `Provider.Embeddings` per chunk (`import_file_test.go`, `confirm_import_test.go`,
      `interfaces/desktop/ingest_test.go`) for a P2 latency optimization, not a correctness fix.
      Verify via the opt-in real-API suite below first, then switch the call site in a follow-up
- [x] Deterministic regression coverage for every case the Codex evaluation diagnosed (named-list
      collapse, PT→EN translation, repeated generic headings under different parents via the
      breadcrumb tests, an over-budget single section) — written inline per Go convention
      (`section_formatting_test.go`, `formatting_validation_test.go`,
      `structure_validation_test.go`, `chunking_test.go`, `preview_import_test.go`), no external
      fixture files (no precedent for them in `application/ingest`). Query-time regression cases
      (a negative question semantically close to the corpus, a typo in a search term) belong to
      `application/knowledge`, not this spec — out of scope here, unaffected by anything in it.
- [x] Opt-in real-API test suite (`internal/application/ingest/realapi_test.go`, gated on
      `ATHENA_REALAPI_TESTS`/`OPENROUTER_API_KEY`, skipped by plain `go test ./...`) replicating
      the LRU/LFU/FIFO/RR case and the BFF-style PT note — **not yet run against the real API in
      this session** (no credentials available); run manually before relying on this spec's
      fixes as verified end-to-end, and before switching `ingestContent` to `EmbeddingsBatch`

## Acceptance Criteria

- A multi-section document's formatting failure (provider error or timeout) degrades only the
  failing section(s) to their original text — every other section's rewrite still lands in
  `FormattedContent` and `FormattingFailed` reports the aggregate.
- When every batch falls back, `PreviewImport`'s `FormattedContent` is byte-identical to the
  original document's own span (leading/trailing whitespace trimmed the same way chunking already
  trims it) — proven by a dedicated round-trip test, not just inspected by eye.
- `ChunkMarkdown` on a nested-heading document produces one breadcrumb per chunk (e.g.
  `"Física > Cinemática > Aceleração"`), and a sibling heading after a deeper one resets the
  breadcrumb to the correct depth rather than appending to the deeper one.
- The embedding call for a chunk with a non-empty heading sends `Heading + "\n\n" + Content`; the
  persisted/displayed `Content` never carries that prefix.
- A batch's rewrite that translates the note, drops a number/URL/code block, or loses its own
  heading is rejected by `validateFormattingFidelity` and the batch falls back to its original
  content — proven against the real diagnosed BFF-translation shape, not just a synthetic one.
- A batch whose original content matches `detectNamedListPattern` and whose first rewrite still
  chunks into exactly one piece gets exactly one corrective retry; the retry's result is kept
  only if it both passes fidelity validation and clears `StructureCheck.Improved` — otherwise the
  first (fidelity-passing) rewrite is kept, `Structure.Improved` stays false, and no third attempt
  is made.
- The Review dialog shows the deterministic `headingsBefore → headingsAfter` /
  `paragraphsBefore → paragraphsAfter` stats line whenever formatting succeeded, names every
  `FailedSection` by heading and reason instead of only a generic notice, and lists every
  `StructureWarning` regardless of whether `FormattingFailed` is true.
- `go test ./...` and the frontend Vitest suite stay green; `gofmt`/`go vet`/`tsc --noEmit`/
  `prettier --check` report no issues on every file this spec touches.

## Known risks

- **The structural instability of the LRU/LFU/FIFO/RR-shaped case is deliberately not resolved
  at its root.** Decision 7 (embedding-text breadcrumb prefix) mitigates the generic-heading
  symptom; structural validation (`validateFormattingStructure`) detects a still-collapsed named
  list and spends one bounded corrective retry on it, and when even that retry doesn't separate
  the list, the batch is accepted with `Structure.Improved = false` and surfaced as a
  `StructureWarning` in the Review dialog — visible, not silent, but still not a fix for the
  underlying non-determinism. The user can still approve the import with the list collapsed;
  this spec only makes that choice an informed one.
- **`maxFormattingChars = 6000` is a budget, not a guarantee.** A single heading section larger
  than the budget on its own still becomes one oversized, unsplit batch — strictly better than
  today's single whole-document call, but not immune to the same timeout risk at a smaller
  blast radius.
- **Breadcrumb headings can grow long under deep nesting.** `truncateBreadcrumb` bounds the
  *displayed* subtitle; the persisted `Chunk.Heading` string itself is not length-bounded.
- **`detectDominantLanguage`'s stopword heuristic is approximate, not a real language detector.**
  It is deliberately biased toward silence (missing a real translation) over a false block of a
  legitimate same-language rewrite, per `languageConfidenceMargin`/`minWordsForLanguageSignal` —
  a short batch, or one in a language outside its PT/EN stopword lists, gets no language signal
  at all rather than a wrong one.
- **`EmbeddingsBatch` is implemented but unverified against the real OpenRouter API, and
  `ingestContent`'s embedding loop still calls `Embeddings` per chunk.** The array-`input`
  contract is standard OpenAI-compatible behavior and is tested against a fake HTTP server, but
  no live call confirmed it in this session. Switching the hot path before that verification
  would risk silently breaking embedding generation for every import, for a latency optimization
  with no correctness upside — see Tasks for the opt-in real-API suite this should run against
  before the call site changes.

## Out of scope

- Lowering `minChunkChars` or adding a chunking rule specific to named-term lists (see "The
  problem, diagnosed" above for why this was declined).
- Any change to `ConfirmImport`/`ingestContent`'s persistence contract or transaction boundary.
- Editing the formatted text by hand before approving (still out of scope per 2.20).
- Any detection/warning if the source file changes on disk between Preview and Confirm (still out
  of scope per 2.20).

## References

- [20-import-review-and-formatting.md](20-import-review-and-formatting.md) — the spec this
  follows up on; its Preview → Review → Confirm gate and byte-exact invariant are unchanged.
- [03-01-chunking-by-paragraph.md](03-01-chunking-by-paragraph.md) — documents the original
  LRU/LFU/FIFO/RR chunking investigation and the accepted 2-chunk (not 4-chunk) residual this
  spec's decision 7/structural-validation follow-up responds to.
- [03-notes-import-and-knowledge-explorer.md](03-notes-import-and-knowledge-explorer.md) — the
  original flat-heading chunking decision this spec reverses (decision 8).
- `internal/application/ingest/section_formatting.go`,
  `internal/application/ingest/preview_import.go`,
  `internal/application/ingest/chunking.go` — the implementation.
