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
   never raises it above `INTRODUCED`. The backend enforces this rule in code; the prompt only
   asks for it.
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
   of [4.1](01-adaptive-tutor-prompt.md), required), `userEvidence` (the user's latest message
   shows the behavior behind `status`), `assisted` (the tutor had to hint or explain to get
   there). At most one block per reply.
5. **Streaming never leaks the block.** A filter holds the text back from the opening tag until
   the closing tag, drops the whole block, and releases an unterminated block's text at the end of
   the stream. An opening tag fragment at a chunk boundary is held, as the offer filter does.
6. **Transition rules in the domain** (pure function, no I/O):
   - `DEMONSTRATED` requires `userEvidence = true`; otherwise the reported status is capped at
     `INTRODUCED`.
   - A concept never goes back to `NOT_ASSESSED`.
   - `DEMONSTRATED` → `PARTIALLY_UNDERSTOOD` or `NEEDS_REVIEW` is allowed (new evidence of a
     gap); a single `DEMONSTRATED` report on a concept in `NEEDS_REVIEW` first moves it to
     `PARTIALLY_UNDERSTOOD`, so one lucky answer never closes a recurring difficulty.
   - `assisted = true` adds one to the concept's `assistanceCount`.
7. **A bad marker never breaks the reply.** Malformed JSON, an unknown status or strategy, a
   blank concept, or more than one block: the reply is delivered and stored as usual, the state
   is not updated for that turn, and the problem is logged without message content.
8. **Concept identity is the normalized name** (trimmed, case-folded, inner whitespace
   collapsed), capped at 80 characters. No canonical-topic mapping; two spellings of one concept
   are two rows, accepted for the MVP.
9. **Bounded.** At most 30 concepts per session; when exceeded, the one with the oldest update
   that is `DEMONSTRATED` is dropped first, then the oldest overall.
10. **Not written when the turn failed.** Parsing and persisting happen after the reply is stored
    successfully, in the same place the offer marker is handled; a cancelled or failed stream
    updates nothing.
11. **The state belongs to the session** and is deleted with it, with the same cascade as the
    session's other dependents.
12. **Opening turn has no marker requirement.** The marker is optional on every reply; the model
    emits it only when a concept was actually worked on.
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
    LastStrategy    TutorStrategy
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
`DeleteOldest`), plus the cap of decision 9.

## Application

`internal/application/study`:

- `tutor_state_marker.go`: `tutorStateInstruction` (pure prompt string, decision 4 and the
  evidence rule), `parseTutorState(content) (cleaned string, report *StateReport, err error)`
  and the streaming `tutorStateFilter` (decision 5).
- `streamAndPersist` ([`stream.go`](../../../internal/application/study/stream.go)) runs the new
  filter next to the offer filter, strips the block from the stored content, and after the reply
  is persisted applies the report through `ConceptStateRepository` (decisions 7 and 10). A repository
  error is logged and does not fail the turn.

## SQLite

```sql
CREATE TABLE session_concepts (
  session_id       TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
  name             TEXT NOT NULL,
  status           TEXT NOT NULL CHECK (status IN
                     ('NOT_ASSESSED','INTRODUCED','PARTIALLY_UNDERSTOOD','DEMONSTRATED','NEEDS_REVIEW')),
  last_strategy    TEXT NOT NULL,
  assistance_count INTEGER NOT NULL DEFAULT 0,
  evidence_count   INTEGER NOT NULL DEFAULT 0,
  updated_at       TEXT NOT NULL,
  PRIMARY KEY (session_id, name)
);
```

Idempotent migration like the existing ones (`CREATE TABLE IF NOT EXISTS`). Session deletion and
reset ([`reset.go`](../../../internal/infrastructure/sqlite/reset.go)) include the table.

## Tasks

- [ ] Domain: statuses, strategies, `ApplyReport` (evidence cap, no regression to
      `NOT_ASSESSED`, `NEEDS_REVIEW` step, assistance count), cap of 30 concepts
- [ ] SQLite: migration, `ConceptStateRepository`, delete with the session and on reset
- [ ] Application: `parseTutorState` (valid, malformed JSON, unknown status/strategy, blank
      concept, two blocks, block not at the end)
- [ ] Application: `tutorStateFilter` (whole block in one chunk, split across chunks, split inside
      the tags, never terminated, text after the block)
- [ ] Application: `streamAndPersist` stores the cleaned content, applies the report, ignores a
      bad marker, writes nothing on a failed or cancelled stream, survives a repository error
- [ ] Prompt test for `tutorStateInstruction` pinned by key phrases (format, evidence rule,
      "never for a concept you only explained")
- [ ] Mocks regenerated (`make mock`)
- [ ] Docs: CHANGELOG (`Added`)

## Acceptance Criteria

- A reply ending with a valid block stores the text without the block, shows the user the text
  without the block even when the tags arrive split across chunks, and upserts the concept.
- A report with `userEvidence = false` and status `DEMONSTRATED` is stored as `INTRODUCED`.
- A `DEMONSTRATED` report on a `NEEDS_REVIEW` concept leaves it `PARTIALLY_UNDERSTOOD`.
- A malformed, unknown-valued, blank-concept or duplicated block leaves the reply intact and the
  state unchanged.
- A failed or cancelled reply changes no state.
- The 31st distinct concept evicts the oldest `DEMONSTRATED` one (else the oldest).
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
