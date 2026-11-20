# Phase 4.2 — Pedagogical State

## Goal

Give the tutor a memory of what the user has shown, kept outside the message history and the
LLM's context, so it survives compaction and reopening a session. After each reply the model
reports, in a hidden block, which concept the turn was about, how well the user handled it and
which strategy it used. The backend validates and stores that. This spec stores and shows the
state to the user nowhere except the database; [4.3](03-state-driven-adaptation.md) feeds it back
to the tutor.

## Decisions

1. **State is derived from observable evidence, never from an explanation.** A concept reaches
   `DEMONSTRATED` only when the user's own message supports it. The tutor explaining a concept
   never raises it above `INTRODUCED`. The backend guarantees coherence in code (without
   `userEvidence`, never `DEMONSTRATED`); the prompt asks for the behavior. Whether
   `userEvidence = true` is actually true is the model's judgment: the backend does not verify it
   in the MVP, so the stored state is an estimate by the model, not a fact.
2. **Five concept statuses:** `NOT_ASSESSED`, `INTRODUCED`, `PARTIALLY_UNDERSTOOD`, `DEMONSTRATED`,
   `NEEDS_REVIEW`. One row per concept per session.
3. **No extra LLM call.** The model ends each reply with a hidden marker block, the same pattern
   as the offer markers ([3.14](../phase-03-architecture-lab/14-ai-challenge-generation.md)). The
   backend removes it from the stored and displayed text.
4. **The marker has a payload,** so the single-token filter of the offer markers is not enough.
   Format, on its own last lines:

   ```text
   [[ATHENA_TUTOR_STATE]]
   {"concept":"Race Condition","status":"PARTIALLY_UNDERSTOOD","strategy":"CORRECTIVE_FEEDBACK","userEvidence":true,"assisted":true}
   [[/ATHENA_TUTOR_STATE]]
   ```

   Fields: `concept` (short name, required), `status` (one of the five), `strategy` (one of the six
   of [4.1](01-adaptive-tutor-prompt.md); case-insensitive; the prompt asks for it, but it is
   auxiliary and tolerated, see decision 7), `userEvidence` (the user's latest message
   shows the behavior behind `status`), `assisted` (the tutor had to hint or explain to get
   there). At most one block per reply.
5. **Streaming never leaks the block.** A filter holds the text back from the opening tag until
   the closing tag, drops the whole block wherever it appears (display safety, independent of
   whether the report is valid), and drops an unterminated block at the end of the stream: the
   user never sees protocol text, the stored content is cut at the opening tag, and the turn
   counts as a bad marker (decision 7). An opening tag fragment at a chunk boundary is held, as the offer filter does.
6. **Transition rules in the domain** (pure function, no I/O). Only evidence moves a status:
   - Without `userEvidence`, a report can only create the row (at `INTRODUCED` at most, so a
     reported `DEMONSTRATED` is capped) or leave the existing status as it is. It never changes
     the status of an existing row, so a re-explanation never regresses a concept and never
     improves a `NEEDS_REVIEW` one.
   - With `userEvidence`, the reported status is applied, with the exceptions below.
   - A reported `NOT_ASSESSED` is ignored: the state does not change. A concept never goes back
     to `NOT_ASSESSED`.
   - `DEMONSTRATED` → `PARTIALLY_UNDERSTOOD` or `NEEDS_REVIEW` is allowed (new evidence of a
     gap); a single `DEMONSTRATED` report on a concept in `NEEDS_REVIEW` first moves it to
     `PARTIALLY_UNDERSTOOD`, so one lucky answer never closes a recurring difficulty.
   - `assisted = true` adds one to `assistanceCount`, `userEvidence = true` adds one to
     `EvidenceCount`, and `LastStrategy` is updated, with or without evidence.
7. **A bad marker never breaks the reply.** Malformed JSON, an unknown status, a
   blank concept, more than one block, an unterminated block, or a block that is not at the end
   of the reply (non-whitespace text after the closing tag): the reply is delivered and stored as usual (the block is still
   removed), the state is not updated for that turn, and the problem is logged without message
   content. The exception is `strategy`: it is only used for `LastStrategy`, so an omitted or
   unknown value does not invalidate the report. The report is applied, `LastStrategy` keeps its
   previous value (empty on a new row), and the problem is logged. The value is compared
   case-insensitively and stored in upper case.
8. **Concept identity is the normalized name** (trimmed, case-folded, inner whitespace
   collapsed), capped at 80 runes (cut on runes, never on bytes, so the result is always valid
   UTF-8). Accents are not folded. No canonical-topic mapping; two spellings of one concept
   (including with and without accents) are two rows, accepted for the MVP. Feeding the stored
   names back to the model in [4.3](03-state-driven-adaptation.md) is expected to reduce the
   drift.
9. **Bounded.** At most 30 concepts per session; when exceeded, the one with the oldest update
   that is `DEMONSTRATED` is dropped first, then the oldest overall. The concept written in the
   current turn is never a candidate, and the upsert and the eviction run in one transaction.
10. **Not written when the turn failed.** Parsing and persisting happen after the reply is stored
    successfully, in the same place the offer marker is handled; a cancelled or failed stream
    updates nothing.
11. **The state belongs to the session** and is deleted with it, with the same cascade as the
    session's other dependents.
12. **Opening turn has no marker requirement.** The marker is optional on every reply; the model
    emits it only when a concept was actually worked on. A reply that answers no user message
    (the opening turn) has no evidence to report, so the backend forces `userEvidence = false`
    on its report before applying it; the rest of the report still counts, and decision 6 caps
    it at `INTRODUCED`.
13. **No data compatibility to keep.** Existing sessions start with no rows.

## Domain

`internal/domain/study/`:

```go
type ConceptStatus string // the five values of decision 2

type TutorStrategy string // the six values of 4.1

type ConceptState struct {
    SessionID       string
    Name            string // normalized, decision 8
    Status          ConceptStatus
    LastStrategy    TutorStrategy // may be empty (decision 7)
    AssistanceCount int
    EvidenceCount   int // turns with userEvidence = true
    UpdatedAt       time.Time
}

type StateReport struct { // what the marker carries
    Concept      string
    Status       ConceptStatus
    Strategy     TutorStrategy
    UserEvidence bool
    Assisted     bool
}

func ApplyReport(current *ConceptState, report StateReport, now time.Time) ConceptState
```

`ApplyReport` implements decision 6. `ConceptStateRepository` (`Upsert`, `ListBySession`,
`DeleteOldest(ctx, sessionID, keep string)`, where `keep` is the concept written in the current
turn and is excluded from the candidates), plus the cap of decision 9.

## Application

`internal/application/study`:

- `tutor_state_marker.go`: `tutorStateInstruction` (pure prompt string, decision 4 and the
  evidence rule), `parseTutorState(content) (cleaned string, report *StateReport, err error)`
  and the streaming `tutorStateFilter` (decision 5).
- `streamAndPersist` ([`stream.go`](../../../internal/application/study/stream.go)) chains the two
  filters in series, strips the block from the stored content, and after the reply is persisted
  applies the report through `ConceptStateRepository` (decisions 7 and 10). A repository error is
  logged and does not fail the turn.
  - **Streaming order:** `chunk → tutorStateFilter → offerMarkerFilter → onChunk`. At the end of
    the stream, `tutorStateFilter.flush` runs first and its output goes through
    `offerMarkerFilter.write`; only then `offerMarkerFilter.flush` runs. The reverse order can
    leak an offer marker split across the two filters.
  - **Final cleaning order:** the raw buffer is kept whole. `parseTutorState` runs first (it
    extracts the report and removes the block, or cuts an unterminated one), then
    `stripOfferMarkers` runs on its output, so the stored text has neither marker.

## SQLite

```sql
CREATE TABLE session_concepts (
  session_id       TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
  name             TEXT NOT NULL,
  status           TEXT NOT NULL CHECK (status IN
                     ('NOT_ASSESSED','INTRODUCED','PARTIALLY_UNDERSTOOD','DEMONSTRATED','NEEDS_REVIEW')),
  last_strategy    TEXT NOT NULL DEFAULT '',
  assistance_count INTEGER NOT NULL DEFAULT 0,
  evidence_count   INTEGER NOT NULL DEFAULT 0,
  updated_at       TEXT NOT NULL,
  PRIMARY KEY (session_id, name)
);
```

Idempotent migration like the existing ones (`CREATE TABLE IF NOT EXISTS`). Session deletion and
reset ([`reset.go`](../../../internal/infrastructure/sqlite/reset.go)) include the table.

## Tasks

- [ ] Domain: statuses, strategies, `ApplyReport` (no status change without evidence, cap at
      `INTRODUCED` for new rows, ignored `NOT_ASSESSED`, `NEEDS_REVIEW` step, assistance and
      evidence counts, `LastStrategy`), name normalization (runes), cap of 30 concepts with `keep`
- [ ] SQLite: migration, `ConceptStateRepository`, delete with the session and on reset
- [ ] Application: `parseTutorState` (valid, malformed JSON, unknown status, blank concept,
      two blocks, block not at the end, unterminated block; strategy in lower case, omitted and
      unknown keeps the report valid)
- [ ] Application: `tutorStateFilter` (whole block in one chunk, split across chunks, split inside
      the tags, never terminated, text after the block)
- [ ] Application: `streamAndPersist` stores the cleaned content, applies the report, ignores a
      bad marker, writes nothing on a failed or cancelled stream, survives a repository error,
      forces `userEvidence = false` on the opening turn, and never leaks an offer marker split
      across chunks in the same reply as a state block
- [ ] Prompt test for `tutorStateInstruction` pinned by key phrases (format, evidence rule,
      "never for a concept you only explained")
- [ ] Mocks regenerated (`make mock`)
- [ ] Docs: CHANGELOG (`Added`)

## Acceptance Criteria

- A reply ending with a valid block stores the text without the block, shows the user the text
  without the block even when the tags arrive split across chunks, and upserts the concept.
- A report with `userEvidence = false` and status `DEMONSTRATED` for a new concept is stored as
  `INTRODUCED`.
- A `DEMONSTRATED` report on a `NEEDS_REVIEW` concept leaves it `PARTIALLY_UNDERSTOOD`.
- A report without `userEvidence` never changes the status of an existing concept (an
  `INTRODUCED` report on a `DEMONSTRATED` concept leaves it `DEMONSTRATED`), but still updates
  `LastStrategy` and `AssistanceCount`.
- A reported `NOT_ASSESSED` changes nothing.
- A report on the opening turn with `userEvidence = true` is applied as if it were `false`.
- A reply with an unterminated block is shown and stored without the block's text, and the state
  is unchanged.
- A report with an omitted or unknown `strategy` is still applied; `LastStrategy` keeps its
  previous value.
- A malformed, unknown-status, blank-concept, duplicated or not-at-the-end block leaves the reply intact and the
  state unchanged.
- A failed or cancelled reply changes no state.
- The 31st distinct concept evicts the oldest `DEMONSTRATED` one (else the oldest).
- Eviction never removes the concept written in the same turn, even when it is the only
  `DEMONSTRATED` one.
- Deleting a session, or resetting local data, removes its concepts.
- No new LLM call is made.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  domain/application code, lint and vulnerability checks pass.

## Out of scope

- Sending the state back to the model ([4.3](03-state-driven-adaptation.md)).
- Any UI showing concepts or status.
- Evidence rows with message references, the session summary and the Learn Intelligent Engine
  event ([4.4](04-evidence-and-session-summary.md)).
- A `confidence` score, canonical concept names, cross-session aggregation.
- Independent verification of `userEvidence` (it would need another LLM call, against decision 3).
