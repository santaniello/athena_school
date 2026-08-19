# Phase 2.2 — Knowledge Extraction

## Goal

On demand, the LLM extracts concepts from a study session's transcript and proposes them as Knowledge Items. Nothing is persisted until the user confirms.

## Trigger

The study domain has **no session-end concept** (`Session` has no `EndedAt`, there is no `End` use case), and reintroducing one would disturb an area that is already stable. Extraction is therefore an **explicit user action**: an "Extract knowledge" button in the study chat composer.

Unlike the Send button (icon-only, used constantly, its meaning already learned by the user), "Extract knowledge" is a **labeled button** — it is a low-frequency, deliberate action the user needs to discover unprompted, and a bare icon would hide it behind a hover.

The button is enabled when:
- the session has at least one message, **and**
- no chat stream is in flight (`isStreaming`), **and**
- no extraction round trip is already in flight (`isExtracting` — a state local to the composer, separate from `isStreaming`, that disables the button and shows a loading indicator for the duration of the `ExtractKnowledge` call; without it, a double-click would fire two concurrent extraction calls and potentially two modals).

## Flow

```text
User clicks "Extract knowledge" (button shows its loading state; isExtracting = true)
    ↓
Session transcript rendered (most recent whole messages, capped — see "Transcript
rendering & truncation")
    ↓
Would the transcript be truncated?
    ├─ No  → proceed directly to the LLM call
    └─ Yes → stop *before* calling the LLM; ask the frontend to confirm
              ↓
         Modal: "This session is long — only the most recent messages will
         be considered for extraction. Continue?" → [Yes] / [No]
              ↓ (Yes)
         Re-invoke extraction with confirmedTruncation = true → proceed to
         the LLM call
    ↓
Extraction prompt sent to the LLM (Task: knowledge_extraction, non-streaming)
    ↓
Response parsed and validated in Go — nothing is trusted
    ↓
Candidates returned to the UI; NOTHING is written yet (isExtracting = false)
    ↓
Modal "New knowledge found" → one checkbox per candidate (all checked) →
[Save as drafts] (disabled if zero checked) / [Ignore]
    ↓
Only on "Save as drafts" are the checked items persisted with status = draft
```

Returning candidates instead of persisting-then-deleting is what makes "Ignore saves nothing" literally true, and it avoids ghost drafts if the app crashes between extraction and the user's choice.

The truncation-confirmation round trip only happens for sessions long enough to exceed `maxTranscriptChars`; the common case (most sessions) resolves in a single call, straight to the candidates modal.

## Configuration

`internal/domain/config.Config` (today only `OpenRouterKey`, persisted to `~/.athena/config.yaml` via the existing `Store`) gains:

```go
type Config struct {
    OpenRouterKey                string
    MaxKnowledgeExtractionItems  int // default 8, valid range 1–20
}
```

This is the safety ceiling on how many candidates a single extraction call will request from the LLM and accept in Go — it exists to protect the UI from an LLM that "gets excited" and returns far more candidates than is useful (see "Validation" below), not to limit what the user can choose to save (the per-item checkboxes already give the user full control over that; a second, separate "max selectable" ceiling was considered and rejected as redundant with manual review).

- Default: `8`, matching this spec's original behavior for anyone who never touches the setting.
- User-editable in a new "Knowledge Extraction" section of `SettingsScreen.tsx`, alongside the existing "Profile" and "OpenRouter key" sections.
- Validated range 1–20 (both in the settings form and defensively in Go) so the field can't be set to something nonsensical (0, or a value large enough to blow the prompt/modal back up).
- `knowledge.Service` takes `configs domainconfig.Store` as a constructor dependency and calls `configs.Load()` at the start of `ExtractFromSession` to read the current value, used both to tell the LLM how many items to return at most (in the prompt) and to truncate an oversized envelope in Go (validation step 3).

## Use cases — `internal/application/knowledge/`

This package is also named `knowledge`, same as `internal/domain/knowledge` — the
same collision `internal/application/folder` already has with `internal/domain/folder`.
Follow that precedent: import the domain package aliased as `domainknowledge` and
qualify the type, e.g. `domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"`.

```go
// Service implements the Knowledge Extraction use cases against a
// domainknowledge.Repository, a domainstudy.SessionRepository, a
// domainstudy.MessageRepository, a domainllm.Provider and a
// domainconfig.Store (for MaxKnowledgeExtractionItems).
type Service struct {
    items    domainknowledge.Repository
    sessions domainstudy.SessionRepository
    messages domainstudy.MessageRepository
    llm      domainllm.Provider
    configs  domainconfig.Store
}

func NewService(
    items domainknowledge.Repository,
    sessions domainstudy.SessionRepository,
    messages domainstudy.MessageRepository,
    llm domainllm.Provider,
    configs domainconfig.Store,
) *Service

// ExtractFromSession reads sessionID's transcript, asks the LLM for
// concepts, and returns validated *unpersisted* draft candidates.
//
// confirmedTruncation must be false on the first call. If the transcript
// needs truncating and confirmedTruncation is false, ExtractFromSession
// returns (nil, true, nil) *without calling the LLM* — the caller re-invokes
// with confirmedTruncation = true once the user has confirmed. If the
// transcript does not need truncating, the call completes in one round trip
// regardless of confirmedTruncation.
func (s *Service) ExtractFromSession(ctx context.Context, sessionID string, confirmedTruncation bool) (items []domainknowledge.Item, truncated bool, err error)

// SaveDrafts persists the items the user confirmed, re-validating every
// field and re-stamping ID/Source/Status/timestamps server-side.
//
// Items are saved sequentially, in the given order. If Repository.Save
// fails for one item, SaveDrafts stops immediately and returns how many
// items were successfully persisted *before* the failure — since
// processing is strictly sequential, this count doubles as an index: the
// first partialCount items in the input were saved, the one at that index
// caused the error, and everything after it was never attempted. The
// frontend uses this to mark saved items and, on retry, must resend only
// the unsaved remainder — resending already-saved items would duplicate
// them, since SaveDrafts always regenerates IDs.
func (s *Service) SaveDrafts(ctx context.Context, items []domainknowledge.Item) (partialCount int, err error)
```

This is the base 2.2 contract. After 2.9, the effective return type becomes
`[]ExtractionCandidate`, pairing each item with validated `EvidenceRefs`, and
`SaveDrafts` accepts those candidates so the item and its evidence are written
atomically. After 2.10/2.11, the same wrapper also carries duplicate matches and a
reconciliation proposal. Keeping those additions in their own specs preserves the
one-behavior-at-a-time TDD order without leaving the final integration ambiguous.

`ExtractFromSession` steps:

1. `sessions.GetByID` → the session `Topic` becomes every item's `Topic`. The LLM's own `topic` field is ignored: deterministic grouping in the explorer, one less field to trust.
2. `messages.ListBySession`. Empty history → return `nil, false, nil` **without calling the LLM**.
3. `configs.Load()` → `MaxKnowledgeExtractionItems` (defaults to 8 if unset).
4. Render the transcript (see "Transcript rendering & truncation"). If it needed truncating and `confirmedTruncation` is false, return `nil, true, nil` **without calling the LLM**.
5. `llm.Chat` with `Task: domainllm.TaskKnowledgeExtraction` (already routed to `TierCheap` in `internal/domain/llm/router.go`).
6. Parse and validate.

## Transcript rendering & truncation

The transcript is **flattened into a single block of text**, embedded inside one `system` message alongside the extraction instructions and JSON schema — not sent as a role-tagged `llm.Message` per turn (the way the ongoing study chat does it in `SendMessage`). Extraction is a one-shot analysis task, not a continuing conversation: role-tagged turns would make the model treat the transcript as a dialogue it is participating in, rather than data it is analyzing from the outside, and a single message is what the prompt spec below requires anyway.

Rendering:

```text
User: <message content>
Assistant: <message content>
User: <message content>
...
```

Truncation to `maxTranscriptChars = 24000` keeps **whole messages only**: walk the session's messages from most recent to oldest, accumulating rendered length, and stop *before* adding a message that would push the total over the limit — that message and everything older is dropped entirely. A message is never sliced mid-content; wasting a fraction of the budget is preferable to feeding the model a truncated sentence, which risks it extracting a concept from a corrupted fragment.

`ExtractFromSession` reports whether this truncation happened via its `truncated` return value, so the caller can gate on user confirmation before spending an LLM call (see "Flow" and "Use cases" above). The confirmation modal is a plain yes/no prompt — no message count or date range is shown:

> "This session is long — only the most recent messages will be considered for extraction. Continue?" — **Yes** / **No**

## Prompt

A single system message that: states the role; inlines the exact JSON schema; requires the envelope `{"items":[...]}` rather than a bare array (models wrap bare arrays in prose far more often, and an envelope leaves room for future fields); forbids markdown fences and commentary; caps the response at `MaxKnowledgeExtractionItems` items (interpolated from config, not a hardcoded number in the prompt text); and requires each definition to be self-contained rather than a restatement of the question. The rendered transcript (see above) is embedded in this same message. Spec 2.9 extends each item with literal `{message_id, quote}` evidence references after transcript turns gain stable message markers.

## Validation — do not trust the LLM

1. `extractJSONObject(raw)` — slice from the first `{` to the last `}`, which unwraps ```json fences emitted despite instructions
2. `json.Unmarshal` into a private envelope struct. **Not** `DisallowUnknownFields` — extra fields are common and harmless
3. Truncate the envelope to `MaxKnowledgeExtractionItems` **in Go**, keeping the first N items in the order the LLM returned them. The prompt asks for at most N, but a prompt is a request, not a constraint — a model returning 50 items must not produce a 50-row modal. There is no relevance/confidence field in the schema to rank by, so "first N as returned" is the only non-arbitrary cut available
4. Per item: trim every string; drop blank list entries; cap sizes (lists 10 entries, `concept` 120 chars, `definition` 2000, each list entry 200) so a hallucinated or hostile payload cannot bloat SQLite
5. Stamp the **server-owned** fields: `ID = uuid.NewString()`, `Topic` from the session, `Source = SourceAthena`, `Status = StatusDraft`, `CreatedAt = UpdatedAt = now`
6. `item.Validate()` — an invalid item is **skipped and the batch continues**; one bad item must not discard the good ones
7. Whole payload unparseable → `nil, ErrMalformedExtraction`

`SaveDrafts` re-runs steps 3–6 on whatever the frontend sends back **and regenerates the ID**. This is the security-relevant part: accepting a client-supplied ID would let a crafted call overwrite or collide with an existing item.

Specs 2.9–2.11 progressively harden this save boundary: evidence ownership is
revalidated, exact duplicates cannot bypass reconciliation, and updates to an
existing item require an explicit user-approved proposal. The frontend never gains
a generic write path around those checks.

Malformed-JSON handling is split by layer: the application returns `ErrMalformedExtraction`; the **desktop binding logs it and returns an empty list**. This keeps `internal/application` free of logging (it has none today, and log lines are mutation-testing noise) while satisfying "caught and logged, no crash".

## Save boundary — SaveDrafts and partial failure

`SaveDrafts` persists items **sequentially**, in the order given, calling `Repository.Save` once per item — there is no batch-save method and no cross-item transaction (no other write path in this codebase uses one today, and introducing one would be new infrastructure for a rare failure mode: a mid-batch SQLite error across at most `MaxKnowledgeExtractionItems` items).

If `Repository.Save` fails for one item, `SaveDrafts` **aborts immediately** and returns the error, along with `partialCount` — how many items were actually persisted before the failure. This is a deliberate choice among three options:
- aborting and propagating the error (chosen) surfaces genuine infrastructure faults (disk full, DB locked) instead of hiding them;
- swallowing the error and treating it like a validation skip was rejected — a repository failure is not an expected outcome the way an invalid LLM field is, and hiding it would leave a real problem invisible;
- wrapping every `Save` in one transaction (true all-or-nothing atomicity) was rejected as unproven infrastructure for a corner case, given no other write path needs it today.

Because processing is strictly sequential, `partialCount` is enough for the frontend to know **exactly which** items succeeded (the first `partialCount` in the array it sent) without any extra bookkeeping from the backend. The frontend uses this to mark those items as saved in the dialog and, if the user retries, resends **only the unsaved remainder** — resending the full original list would re-insert the already-saved items under new (regenerated) IDs, duplicating them.

## Tasks

- [ ] `internal/domain/config/config.go` — add `MaxKnowledgeExtractionItems int` to `Config`, defaulting to 8; validate range 1–20
- [ ] `internal/domain/knowledge/item.go` — add `Validate()` with `ErrTopicRequired` / `ErrConceptRequired` / `ErrDefinitionRequired`, mirroring `profile.UserProfile.Validate`
- [ ] `internal/application/knowledge/service.go` — `Service{items, sessions, messages, llm, configs}` + `NewService`
- [ ] `internal/application/knowledge/extraction.go` — `ExtractFromSession` (with `confirmedTruncation`/`truncated`), `SaveDrafts` (with `partialCount`)
- [ ] `internal/application/knowledge/prompt.go` — extraction prompt builder; flattened transcript rendering with whole-message truncation; item count interpolated from config
- [ ] `internal/application/knowledge/parse.go` — `extractJSONObject`, envelope decoding, caps (using the configured max), per-item validation
- [ ] `internal/application/knowledge/errors.go` — `ErrMalformedExtraction`
- [ ] `internal/interfaces/desktop/knowledge.go` — `ExtractKnowledge(sessionID string, confirmedTruncation bool) (ExtractionResult, error)` returning a wrapper `{Items []KnowledgeItemResult, Truncated bool}` (same pattern as `ResumeStudySession`'s wrapper result), `SaveExtractedKnowledge([]KnowledgeItemInput) (int, error)`; both `KnowledgeItemResult` and `KnowledgeItemInput` mirror `domainknowledge.Item` in full (including `Properties`/`TradeOffs`/`RelatedConcepts`), so those fields survive the round trip even though the dialog never renders them; `App` gains a `knowledge` field and `NewApp` a parameter (mechanical `nil` updates across the desktop test files)
- [ ] `internal/interfaces/desktop/settings.go` — read/update `MaxKnowledgeExtractionItems` alongside the existing config fields
- [ ] `frontend/src/lib/knowledge.ts` — binding wrappers
- [ ] `frontend/src/components/knowledge-extraction-dialog.tsx` — shadcn `Dialog`, one checkbox per item (all checked), buttons **[Save as drafts]** (disabled when zero items are checked) and **[Ignore]**; empty result shows "No new knowledge found"; on a partial-save error, marks the already-saved items and retries only the remainder. The dialog **displays only `concept` and `definition`** per candidate, but keeps the full candidate object (including `properties`/`tradeOffs`/`relatedConcepts`) in component state and resends it unmodified on "Save as drafts" — those fields are never shown in this modal, but must not be dropped, or every saved draft would silently lose them
- [ ] `frontend/src/components/transcript-truncation-dialog.tsx` (or inlined into the extraction flow) — plain yes/no confirmation shown only when `truncated` comes back true
- [ ] `frontend/src/screens/StudyChatScreen.tsx` — labeled "Extract knowledge" button in the composer row, disabled while streaming, extracting (`isExtracting`), or with an empty transcript; a distinct error `Alert` (reusing the existing pattern) for genuine `Chat` failures, separate from the "No new knowledge found" empty state
- [ ] `frontend/src/screens/SettingsScreen.tsx` — new "Knowledge Extraction" section with a numeric input for `MaxKnowledgeExtractionItems`, validated 1–20

> The third modal button, **[Save & approve]**, is added in 2.6 once `ApproveKnowledgeItem` exists. It completes the three-option flow of `specs/Athena.md` §12 without anticipating a use case that does not exist yet.

## Acceptance Criteria

- Clicking "Extract knowledge" on a session with history returns candidates; the modal lists each concept with its definition
- A session with no messages returns no candidates and **makes no LLM call**
- A session whose transcript exceeds `maxTranscriptChars` triggers the truncation-confirmation prompt **before** any LLM call; declining makes no LLM call; confirming proceeds using only the most recent whole messages that fit the budget
- A session whose transcript fits under `maxTranscriptChars` completes in a single call, with no truncation prompt shown
- Extracted items carry `status = "draft"`, `source = "athena"`, the session's topic, and a server-generated ID — never values taken from the LLM payload
- Choosing "Ignore" writes nothing: `knowledge_items` row count is unchanged
- Choosing "Save as drafts" persists exactly the checked items and returns the count; the button is disabled when zero items are checked
- A response wrapped in ```json fences is parsed successfully
- Malformed JSON produces no crash: the binding logs it and the UI shows an empty result
- An item missing its definition is skipped while its valid siblings are still saved
- A definition exactly at the size cap is kept intact; one over the cap is truncated
- A payload with more items than `MaxKnowledgeExtractionItems` is truncated to that many, keeping the first N in returned order, regardless of what the prompt asked for
- `MaxKnowledgeExtractionItems` is configurable in Settings, defaults to 8, and is rejected outside the 1–20 range
- `SaveDrafts` regenerates IDs, ignoring any ID supplied by the caller
- A `SaveDrafts` call that fails partway persists every item before the failure, returns that count, and a retry that resends only the unsaved remainder does not duplicate the already-saved items
- A genuine extraction failure (e.g. missing API key) surfaces as an inline error, distinct from the "No new knowledge found" empty-result modal
- After 2.9, every saved candidate includes at least one validated evidence snapshot from the source session
- After 2.10/2.11, duplicate candidates are reconciled rather than silently inserted as independent items