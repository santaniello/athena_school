# Phase 2.5.1 — HyDE Query Enrichment for `strict-notes`

## Goal

Close the gap where `strict-notes` refuses to answer a question whose matching
chunk genuinely exists and was correctly imported, because the raw
short-question embedding scores below `Sufficiency` even against a
near-verbatim match — without touching `notes` mode, the thresholds
themselves, or chunking.

## Why this is a separate spec

05 (`05-rag-integration.md`) defines the retrieval contract this spec extends:
`Retrieve(ctx, sessionID, query string)`, one deterministic query per turn, no
LLM call rewrites it ("no LLM call rewrites it" is explicit in
`buildRetrievalQuery`'s own doc comment), and the two score thresholds
(`DefaultMinSimilarity`/`DefaultSufficiency`) calibrated against
`text-embedding-3-small`'s anisotropic cosine space. This spec adds a new
pre-retrieval step on top of that contract rather than amending it in place,
because the two problems are genuinely different: 05 already protects against
a chunk that merely *shares vocabulary* with the query; this spec is about a
chunk that *is* the right answer scoring low anyway, for structural reasons
05 didn't anticipate.

## The problem

Diagnosed 2026-10-01 against a real imported document (see
[[project_strict_notes_sufficiency_threshold_too_strict]] for the full
investigation): a user imported a Portuguese cache/Redis study note and asked
`strict-notes` two questions whose answers are in a chunk whose own heading
almost literally repeats the question. Both got the fixed miss response.

Calling the real embeddings endpoint directly and comparing against the real
stored chunk vectors isolated two compounding, non-bug causes:

- **Topic dilution**: the chunk's own heading combines two sub-topics
  ("Políticas de Evicção **e** Invalidação do Cache"), diluting its pooled
  embedding's alignment with a query about only one of them.
- **Short-query-vs-long-chunk length bias**: even a text that is *only* about
  the matching sub-topic, with no dilution, scores 0.7974 against the full
  chunk — but the same content reduced to a short natural-language question
  drops to ~0.60. This is a structural effect of how the embedding model
  treats short vs. long text, independent of relevance.

This spec addresses the second cause. The first (topic dilution via finer
chunking) is tracked as a separate, not-yet-started increment — the two are
complementary, not alternatives, but were deliberately sequenced apart so
each one's effect can be measured in isolation (see "Out of scope" below).

## Decision

**HyDE (Hypothetical Document Embeddings)**, scoped to `strict-notes` only,
combined with (never replacing) the raw query, decided during design
discussion on 2026-10-01.

**Technique.** Of the three realistic options — reformulating the question,
HyDE, and multi-query expansion — HyDE was chosen because the diagnosed cause
is a *length/density* mismatch, not a *vocabulary* mismatch. A reformulated
question is still a short question; it doesn't change shape enough to close
the measured gap. Multi-query expansion targets vocabulary mismatch, which
isn't what was measured here. HyDE's own failure mode — the hypothetical
passage drifting from how the real note is actually written — is contained by
construction: the hypothetical text is used only to search, then discarded;
it is never persisted, never shown, and never reaches the system prompt that
produces the visible answer. The final reply is still generated exclusively
from the real retrieved chunk content, exactly as today.

**Scope.** `notes` mode is untouched. The same diagnosed query (score 0.5992)
already clears `DefaultMinSimilarity` (0.45) today, so `notes` already uses
that chunk as supporting context in its middle tier ("related but may not
fully answer") — it never hit this failure at all. `notes` is also the
default, most-used mode; paying an extra LLM call on every one of its turns
for a gain that's already covered by its own graceful degradation doesn't pay
for itself the way it does in `strict-notes`, where the failure is a hard,
visible refusal.

**Where it lives.** Not inside `SendMessage` (`internal/application/study`),
and not inside `Retrieve` (`internal/application/knowledge`). A new, small,
focused port (`QueryEnricher`) owns the enrichment, injected into
`study.Service` the same way `Retriever` already is. This was revised
mid-discussion once it came up that other future modules (Architecture Lab,
Challenge, etc.) will have their own `notes`/`strict-notes` flows and will
want the same enrichment: putting it inside `SendMessage` would mean
reimplementing it per module; putting it inside `Retrieve` would force
`Retrieve` to learn about source modes, which it deliberately does not do
today (05's own contract: retrieval is identical regardless of mode, the
caller decides what to do with the result). A standalone, reusable service
avoids both costs — a new module injects the same `QueryEnricher`
implementation, it does not get it for free just by calling `Retrieve`.

**Combine vs. replace.** `Retrieve`'s signature generalizes from one query
string to a list of query strings; it embeds and searches each, then combines
per chunk by taking the **best** score across all of them, before
`MinSimilarity` filtering, capping and rendering proceed exactly as today.
Replacing the raw query outright was rejected: if the hypothetical passage
drifts, it could make a case that already works today (the raw query alone
clearing `MinSimilarity`) worse. Combining can only match or improve on the
raw-query-only result. This does have a real, accepted cost: combining two
independent scores via "take the best" structurally gives a false-positive
query two chances instead of one to clear `MinSimilarity`, which the
0.45 floor was raised specifically to make harder (05's "Thresholds"
section). The floor still applies to the combined score, so this narrows,
rather than removes, that protection.

**Failure behavior.** If the enrichment call itself fails (network, rate
limit), the turn fails with a visible error — the same pattern `Retrieve`
already follows today when its own `Embeddings` call fails. This is a
deliberate trade-off, accepted knowing it: a turn that would have succeeded
fine on the raw query alone can now fail if only the enrichment call errors.
Failing open (silently falling back to the raw query) was rejected because it
would hide a systemic enrichment failure (e.g. a bad model ID) behind
always-normal-looking behavior, with no signal to notice or debug it by.

### Implementation shape

- `internal/domain/knowledge`: new port
  `QueryEnricher.Enrich(ctx, sessionID, topic, content string) (string, error)`.
- `internal/domain/llm/router.go`: new `TaskQueryEnrichment` `TaskType`,
  routed to `TierCheap` — same routing mechanism already used for
  onboarding.
- `internal/application/knowledge`: new implementation backed by
  `domainllm.Provider.Chat` (not `ChatStream` — this is a single, short,
  non-streamed completion), prompted to write a short hypothetical passage
  that would answer the question, not an answer directed at the user.
- `internal/domain/knowledge.Retriever.Retrieve` / `application/knowledge.Service.Retrieve`:
  signature generalizes from `(ctx, sessionID, query string)` to
  `(ctx, sessionID string, queries []string)`. Internally: embed each query,
  search each, merge by chunk ID taking the max score, then proceed with the
  existing `MinSimilarity` filter → concept resolution → cap → render
  pipeline unchanged.
- `internal/application/study.Service`: new constructor dependency
  `enricher domainknowledge.QueryEnricher`. In `SendMessage`, when
  `sourceMode == SourceModeStrictNotes`: call `Enrich` before `Retrieve`;
  on success call `Retrieve(ctx, sessionID, []string{rawQuery, hypothetical})`;
  on failure, propagate the error (turn fails). Every other mode calls
  `Retrieve(ctx, sessionID, []string{rawQuery})` — unchanged behavior.
- Wiring: `internal/interfaces/desktop` app construction gains the new
  `QueryEnricher` implementation and passes it into `study.NewService`.
- Mocks: regenerate `MockRetriever` (new signature) and add
  `MockQueryEnricher` via `make mock`.
- Tests: `Retrieve`'s existing single-query tests adapt to the new `[]string`
  parameter (wrapping one element); new tests cover multi-query combination
  (a chunk scored low by one query and high by another surfaces at the high
  score). `SendMessage` gains tests for the `strict-notes` enrichment call
  (success path uses both queries, failure path propagates the error,
  `notes` mode never calls the enricher at all).

## Known risks

- Combining via "take the best" slightly increases the chance of an
  unrelated query clearing `MinSimilarity` by coincidence — `MinSimilarity`
  still applies to the combined score, so this narrows rather than removes
  that protection. Accepted; see "Combine vs. replace" above.
- A new failure mode: the enrichment call failing now fails turns that would
  have succeeded fine on the raw query alone. Accepted; see "Failure
  behavior" above.
- `DefaultSufficiency` (0.68) itself is not revisited here — recalibrating it
  needs more real data points than the two gathered so far (see
  [[project_strict_notes_sufficiency_threshold_too_strict]]). This spec
  improves the input to that threshold instead of changing the threshold.

## Out of scope

- Changing `DefaultMinSimilarity`/`DefaultSufficiency`.
- Finer-grained chunking to address topic dilution — a separate, not-yet
  started increment, deliberately sequenced after this one so each one's
  effect on retrieval scores can be measured independently.
- Applying HyDE to `notes` mode.
- Any Architecture Lab / Challenge module work — this spec only makes
  `QueryEnricher` reusable for when those are built, it does not build them.

## References

- Diagnosed from a user report during manual testing of spec
  [18-01](18-01-inline-citations-in-chat.md)'s branch; see
  [[project_strict_notes_sufficiency_threshold_too_strict]] (assistant memory)
  for the full empirical investigation (real embeddings-API scores against
  the real imported document).
- `05-rag-integration.md` — the retrieval/threshold contract this extends.
- `internal/application/study/send_message.go` — `buildRetrievalQuery`,
  the `strict-notes` branch this hooks into.
- `internal/application/knowledge/retrieval.go` — `Retrieve`'s existing
  single-query pipeline.
