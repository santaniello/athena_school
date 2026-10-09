# Phase 2.5.2 — `strict-notes` Query Scope and Sufficiency Recalibration

## Goal

Close the remaining gap where `strict-notes` refuses to answer a question
whose matching chunk genuinely exists — now that chunking (2.3.1) and HyDE
(2.5.1) have already closed most of it — without opening an easy path for
`strict-notes` to answer an off-topic question as if the note supported it,
and without regressing `notes` mode's own retrieval.

## Why this is a separate spec

05 (`05-rag-integration.md`) defines `buildRetrievalQuery`'s
`"Topic: <topic>\n\nMessage: <content>"` format and the two score thresholds.
05-01 (HyDE) deliberately left `DefaultSufficiency` untouched, noting it
needed more real data points first. This spec is that follow-up: it changes
what text `strict-notes` embeds for its raw query, and recalibrates
`DefaultSufficiency` against the resulting scores — both informed by data
gathered after 2.3.1 and 2.5.1 shipped, not available when 05 or 05-01 were
written.

## The problem

After chunking (2.3.1) and HyDE (2.5.1) both shipped, re-testing the
originally-diagnosed note (see
[[project_strict_notes_sufficiency_threshold_too_strict]] for the full
investigation) showed:

- The topic-level question that originally failed now clears `Sufficiency`
  easily (0.60 pre-fix → 0.81 with both fixes).
- Two acronym-definition questions ("o que é o LRU?", "o que é o LFU?")
  still don't: 0.66 and 0.61 respectively, against a 0.68 bar — within
  0.02-0.07, closer than before either fix, but still short.

Investigating why the margin was so thin surfaced the actual root cause:
`buildRetrievalQuery`'s `"Topic: <topic>"` prefix inflates every query's
score by roughly the same amount (+0.16 to +0.24 measured), on-topic or not
— because the note's own headings repeat the topic word densely, so any
query that also says it gets a relevance-blind boost. Measuring 4 genuinely
off-topic questions (B-Tree index, CAP theorem, TCP vs UDP, Dijkstra) against
the same chunks: **with** the prefix they scored 0.49-0.57, uncomfortably
close to the 0.61 LFU floor — a ~0.02 usable margin. **Without** it, the
off-topic ceiling dropped to 0.47 while the on-topic floor barely moved
(0.60) — a ~0.12 margin, six times wider.

Dropping the prefix is not free, though: the same topic context that adds
noise for a well-formed question is load-bearing for a genuinely ambiguous
one. Four zero-domain-vocabulary follow-up questions ("o que é isso?", "como
assim?", "me explica isso melhor", "pode dar um exemplo?" — `notes` mode's
real degrade-gracefully scenario, which has no HyDE to compensate) scored
0.60-0.63 **with** the prefix but only 0.28-0.40 **without** it — below
`MinSimilarity` (0.45), i.e. complete retrieval loss for an ordinary
conversational pattern.

## Decision

Two changes, decided together during design discussion on 2026-10-09:

**1. Scope `buildRetrievalQuery`'s topic prefix to `notes` only;
`strict-notes` embeds the raw trimmed message alone (still combined with its
HyDE hypothetical, unchanged from 05-01).**

Dropping the prefix for both modes was considered first and rejected once
measured: `notes` mode's own score never gates anything (it has no
`Sufficiency` check — a weak match is just loosely-injected context
alongside the model's general knowledge), so the prefix's inflation costs
`notes` nothing today. `strict-notes` is the only mode where this same
number decides whether the model may answer *exclusively* from a chunk and
cite it as the note's own guarantee — the one place the inflation actively
removes the thresholds' discriminating power. Scoping the fix to
`strict-notes` keeps `notes` exactly as it works today, including for the
ambiguous-follow-up pattern that depends on the prefix.

**2. Lower `DefaultSufficiency` from 0.68 to 0.55.**

Picked as the midpoint of the post-fix, prefix-dropped `strict-notes` margin
(~0.47 off-topic ceiling, ~0.60 on-topic floor from the 7 real queries
measured) — comfortable room on both sides given the data, rather than
hugging either edge. `DefaultMinSimilarity` (0.45) is unchanged; `0.55` still
satisfies `NewRetrievalThresholds`' `minScore <= sufficiencyScore` invariant
with room to spare.

## Implementation shape

- `internal/application/study/send_message.go`: `SendMessage`'s query
  construction branches by `sourceMode`. `notes` keeps calling
  `buildRetrievalQuery(topic, content)` exactly as today. `strict-notes`
  uses `content` (already trimmed earlier in `SendMessage`) as its raw
  query, still followed by the existing HyDE `Enrich(ctx, sessionID, topic,
  content)` call and the same combine-by-best-score `Retrieve`. `topic`
  continues to flow into `Enrich` unchanged — only the raw embedded query
  text drops it, not the hypothetical-generation prompt.
- `internal/domain/knowledge/retrieval.go`: `DefaultSufficiency` constant
  `0.68` → `0.55`; update its doc comment to explain the thin-margin history
  rather than leave a bare number.
- No change to `internal/application/knowledge/retrieval.go` (`Retrieve`
  itself, `MinSimilarity` filtering, capping) — this spec only changes what
  text reaches it as a query, and where the bar sits.
- Tests: `send_message_test.go` gains coverage that `strict-notes` embeds
  the raw message without the topic prefix (while `notes` still gets the
  prefixed form), and that the HyDE `Enrich` call still receives `topic`
  unchanged. `internal/domain/knowledge` threshold tests cover the new
  default value.

## Known risks

- `0.55` is a global constant, calibrated from 4 off-topic and 3 on-topic
  real queries against **one** note. A future note that is denser, in a
  different language, or shares more incidental vocabulary with an
  off-topic question than this note's sample did could have a noise ceiling
  above 0.47 (even above 0.55) — in which case `strict-notes` would answer
  an off-topic question as if the note supported it, with no way for the
  user or the system to tell. There is no monitoring or telemetry today that
  would surface this if it happens; it would only be caught the same way the
  original LRU/LFU gap was — a user noticing a specific wrong or missing
  answer and reporting it.
- Scoping the query-construction change by `sourceMode` inside `SendMessage`
  means the same question now embeds different text depending on mode —
  one more thing to remember when reasoning about retrieval behavior across
  modes, accepted as the smaller cost against the demonstrated regression
  of applying it uniformly.

## Out of scope

- Changing `DefaultMinSimilarity`.
- Any monitoring/telemetry for suspicious `strict-notes` answers — tracked
  as a future need if the risk above materializes, not built here.
- Revisiting HyDE's own design (05-01) — its combine-by-best-score mechanism
  and `strict-notes`-only scope are unchanged.
- Per-session or per-language threshold tuning — `DefaultSufficiency` stays
  one global constant.

## References

- [[project_strict_notes_sufficiency_threshold_too_strict]] (assistant
  memory) — the full empirical investigation across this spec, 2.3.1 and
  05-01, including the raw measured scores this decision is based on.
- `05-rag-integration.md` — the retrieval/threshold contract this amends.
- [05-01-hyde-query-enrichment.md](05-01-hyde-query-enrichment.md) — the
  prior increment this follows; its own "Known risks" flagged
  `DefaultSufficiency` as needing more data before being revisited, which
  this spec provides.
- [03-01-chunking-by-paragraph.md](03-01-chunking-by-paragraph.md) — the
  other upstream fix whose combined effect with HyDE is what narrowed the
  remaining gap to a threshold/margin problem instead of a pure scoring one.
