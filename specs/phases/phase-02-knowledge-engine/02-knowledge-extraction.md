# Phase 2.2 — Knowledge Extraction

## Goal

On demand, the LLM extracts concepts from a study session's transcript and proposes them as Knowledge Items. Nothing is persisted until the user confirms.

## Trigger

The study domain has **no session-end concept** (`Session` has no `EndedAt`, there is no `End` use case), and reintroducing one would disturb an area that is already stable. Extraction is therefore an **explicit user action**: an "Extract knowledge" button in the study chat composer, enabled when the session has at least one message and no stream is in flight.

## Flow

```text
User clicks "Extract knowledge"
    ↓
Session transcript loaded (most recent turns, capped)
    ↓
Extraction prompt sent to the LLM (Task: knowledge_extraction, non-streaming)
    ↓
Response parsed and validated in Go — nothing is trusted
    ↓
Candidates returned to the UI; NOTHING is written yet
    ↓
Modal "New knowledge found" → [Save as drafts] / [Ignore]
    ↓
Only on "Save as drafts" are items persisted with status = draft
```

Returning candidates instead of persisting-then-deleting is what makes "Ignore saves nothing" literally true, and it avoids ghost drafts if the app crashes between extraction and the user's choice.

## Use cases — `internal/application/knowledge/`

```go
// ExtractFromSession reads sessionID's transcript, asks the LLM for
// concepts, and returns validated *unpersisted* draft candidates.
func (s *Service) ExtractFromSession(ctx context.Context, sessionID string) ([]Item, error)

// SaveDrafts persists the items the user confirmed, re-validating every
// field and re-stamping ID/Source/Status/timestamps server-side.
func (s *Service) SaveDrafts(ctx context.Context, items []Item) (int, error)
```

This is the base 2.2 contract. After 2.9, the effective return type becomes
`[]ExtractionCandidate`, pairing each item with validated `EvidenceRefs`, and
`SaveDrafts` accepts those candidates so the item and its evidence are written
atomically. After 2.10/2.11, the same wrapper also carries duplicate matches and a
reconciliation proposal. Keeping those additions in their own specs preserves the
one-behavior-at-a-time TDD order without leaving the final integration ambiguous.

`ExtractFromSession` steps:

1. `sessions.GetByID` → the session `Topic` becomes every item's `Topic`. The LLM's own `topic` field is ignored: deterministic grouping in the explorer, one less field to trust.
2. `messages.ListBySession`. Empty history → return `nil, nil` **without calling the LLM**.
3. Render the transcript under `maxTranscriptChars = 24000`, keeping the most recent turns, so a long session cannot blow the context window.
4. `llm.Chat` with `Task: domainllm.TaskKnowledgeExtraction` (already routed to `TierCheap` in `internal/domain/llm/router.go`).
5. Parse and validate.

## Prompt

A single system message that: states the role; inlines the exact JSON schema; requires the envelope `{"items":[...]}` rather than a bare array (models wrap bare arrays in prose far more often, and an envelope leaves room for future fields); forbids markdown fences and commentary; caps the response at 8 items; and requires each definition to be self-contained rather than a restatement of the question. Spec 2.9 extends each item with literal `{message_id, quote}` evidence references after transcript turns gain stable message markers.

## Validation — do not trust the LLM

1. `extractJSONObject(raw)` — slice from the first `{` to the last `}`, which unwraps ```json fences emitted despite instructions
2. `json.Unmarshal` into a private envelope struct. **Not** `DisallowUnknownFields` — extra fields are common and harmless
3. Truncate the envelope to `maxItems = 8` **in Go**. The prompt asks for at most 8, but a prompt is a request, not a constraint — a model returning 50 items must not produce a 50-row modal
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

## Tasks

- [ ] `internal/domain/knowledge/item.go` — add `Validate()` with `ErrTopicRequired` / `ErrConceptRequired` / `ErrDefinitionRequired`, mirroring `profile.UserProfile.Validate`
- [ ] `internal/application/knowledge/service.go` — `Service{items, sessions, messages, llm}` + `NewService`
- [ ] `internal/application/knowledge/extraction.go` — `ExtractFromSession`, `SaveDrafts`
- [ ] `internal/application/knowledge/prompt.go` — extraction prompt builder
- [ ] `internal/application/knowledge/parse.go` — `extractJSONObject`, envelope decoding, caps, per-item validation
- [ ] `internal/application/knowledge/errors.go` — `ErrMalformedExtraction`
- [ ] `internal/interfaces/desktop/knowledge.go` — `ExtractKnowledge(sessionID)`, `SaveExtractedKnowledge([]KnowledgeItemInput)`; `App` gains a `knowledge` field and `NewApp` a parameter (mechanical `nil` updates across the desktop test files)
- [ ] `frontend/src/lib/knowledge.ts` — binding wrappers
- [ ] `frontend/src/components/knowledge-extraction-dialog.tsx` — shadcn `Dialog`, one checkbox per item (all checked), buttons **[Save as drafts]** and **[Ignore]**; empty result shows "No new knowledge found"
- [ ] `frontend/src/screens/StudyChatScreen.tsx` — "Extract knowledge" button in the composer row, disabled while streaming or with an empty transcript

> The third modal button, **[Save & approve]**, is added in 2.6 once `ApproveKnowledgeItem` exists. It completes the three-option flow of `specs/Athena.md` §12 without anticipating a use case that does not exist yet.

## Acceptance Criteria

- Clicking "Extract knowledge" on a session with history returns candidates; the modal lists each concept with its definition
- A session with no messages returns no candidates and **makes no LLM call**
- Extracted items carry `status = "draft"`, `source = "athena"`, the session's topic, and a server-generated ID — never values taken from the LLM payload
- Choosing "Ignore" writes nothing: `knowledge_items` row count is unchanged
- Choosing "Save as drafts" persists exactly the checked items and returns the count
- A response wrapped in ```json fences is parsed successfully
- Malformed JSON produces no crash: the binding logs it and the UI shows an empty result
- An item missing its definition is skipped while its valid siblings are still saved
- A definition exactly at the size cap is kept intact; one over the cap is truncated
- A payload with more than `maxItems` items is truncated to `maxItems`, regardless of what the prompt asked for
- `SaveDrafts` regenerates IDs, ignoring any ID supplied by the caller
- After 2.9, every saved candidate includes at least one validated evidence snapshot from the source session
- After 2.10/2.11, duplicate candidates are reconciled rather than silently inserted as independent items
