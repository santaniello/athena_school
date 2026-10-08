# Phase 2.3.1 — Chunk by Paragraph, Not by Budget

## Goal

Stop packing multiple paragraphs into one chunk up to `maxChunkChars`, so a
heading section that bundles more than one sub-topic (because the note's own
author wrote it that way) produces one chunk per sub-topic instead of one
diluted chunk covering all of them — without any migration of already
-imported documents, and without touching `minChunkChars`'s existing
merge-forward/backward safety net.

## Why this is a separate spec

03 (`03-notes-import-and-knowledge-explorer.md`) and `packParagraphs`'s own
doc comment define today's contract: paragraphs (blank-line-separated blocks
within one heading section) are greedily packed into pieces no larger than
`maxChunkChars` (2000), with `minChunkChars` (200) folding any undersized
piece into a neighbor afterward. This spec changes the first half of that
contract — packing becomes purely paragraph-boundary-driven, one candidate
per paragraph, before the existing merge step runs — while leaving the
merge step itself untouched.

## The problem

Diagnosed 2026-10-01/02 against a real imported document (see
[[project_strict_notes_sufficiency_threshold_too_strict]] and
[05-01-hyde-query-enrichment.md](05-01-hyde-query-enrichment.md) for the full
investigation and HyDE, the complementary fix already shipped for this same
symptom): a user's note has one H3 heading, "✨ Políticas de Evicção **e**
Invalidação do Cache", whose body is three paragraphs — an eviction-policy
bullet list, an invalidation-policy bullet list, and a closing remark — each
individually well over `minChunkChars`. At 829 total characters the whole
section is well under `maxChunkChars`, so `packParagraphs` packs all three
into one chunk. That chunk's single pooled embedding sits between two
distinct sub-topics, diluting its alignment with a query about only one of
them — confirmed by re-embedding variants of the same text: the narrow,
single-topic eviction bullets alone scored 0.80 against the full chunk, but
the same idea phrased as a short question dropped to ~0.60, under
`strict-notes`'s `0.68` `Sufficiency` bar.

HyDE (05-01) already attacks the second half of that gap (the
short-question-vs-long-chunk structural bias) by enriching the query itself.
This spec attacks the first half — the chunk's own topic dilution — by
changing what gets embedded in the first place, complementary to (not a
replacement for) HyDE: verified against the same live session that even
with HyDE combined, two further short/generic questions ("me explique o que
é LRU", "o que é evicção ?") still missed, one of them because HyDE's own
generated passage drifted toward "evicção"'s unrelated legal meaning. Finer
chunking raises the ceiling score for *both* the raw query and any future
HyDE passage, by giving the embedding model a purer, single-topic chunk to
compare against in the first place.

## Decision

**Chunk by paragraph, no packing**, decided during design discussion on
2026-10-08.

**Splitting strategy.** Of three options — splitting by paragraph, a
"sub-heading paragraph" heuristic (e.g. a lead-in line ending in `:`
followed by a list), or simply lowering `maxChunkChars` globally — splitting
by paragraph was chosen because the other two have narrower, less reliable
failure modes: the heuristic only helps notes that happen to use a
colon-label convention (this user's own note has *other* sections, like
"✨ Estratégias de Cache", that don't), and lowering the budget is a blunt,
global change whose exact split point is still packing-order-dependent, not
guaranteed to land between the two sub-topics at all. Splitting by paragraph
is general — it works on any document's structure — and, critically, reuses
`mergeUndersized`'s *already-existing* floor instead of adding new logic:
checked against the real section's own paragraph sizes (roughly 230/200/320
characters), all three already clear `minChunkChars` on their own, so they
survive as three separate chunks for free, while a section with several
genuinely short paragraphs still gets folded back together by the same
merge step exactly as it does today — no regression for documents that
don't have this problem.

**Accepted cost.** A section whose paragraphs are all substantial
(individually over `minChunkChars`) but genuinely part of one explanation,
merely spread across paragraphs for readability rather than covering
separate sub-topics, now also splits into multiple chunks. This is a real,
accepted trade-off — judged gentler than the alternatives' own failure
modes, and narrower embeddings rarely hurt retrieval even when the source
paragraphs were "related" rather than unrelated.

**No migration.** An already-imported document keeps its existing chunks
until explicitly re-imported — consistent with how chunking has always
worked (it only ever runs at import time) and with `shouldSkipUnchanged`
already skipping a re-import of an unchanged file. Building any automatic
re-chunk/backfill mechanism was rejected as disproportionate: it would need
to version the chunking algorithm, add UI for it, and decide what to do
about re-paying for embeddings already billed once — a much bigger lift than
this spec's actual problem warrants. A user who wants the new chunking
ahead of a real edit can force it by removing and re-adding the source, same
as any other re-import today.

### Implementation shape

- `internal/application/ingest/chunking.go` (`packParagraphs`): remove the
  greedy multi-paragraph accumulation loop. Each `paragraphSpan` from
  `splitParagraphs` becomes its own `ChunkCandidate` directly — no budget
  comparison. A single paragraph whose own length already exceeds
  `maxChunkChars` is unaffected either way (it was already kept whole,
  never split, under the old code too).
- `maxChunkChars` is removed from `chunking.go` — its only production
  usage was the budget comparison this spec removes. `minChunkChars` and
  `mergeUndersized` are untouched.
- `ChunkMarkdown`/`ChunkText` call sites are unchanged; both inherit the new
  behavior automatically, since both already route through
  `packParagraphs`/`mergeUndersized`.
- Tests: audited the existing `chunking_test.go` — the large majority
  construct single-paragraph sections and are unaffected, or already
  assert loosely enough (`len(chunks) > 1`, each `<= maxChunkChars`) to
  stay valid under the new behavior. Exactly two
  (`TestPackParagraphs_combinedParagraphsExactlyAtMaxChunkChars_packIntoOnePiece`,
  `TestPackParagraphs_combinedParagraphsOneOverMaxChunkChars_splitsIntoTwoPieces`)
  directly assert the old multi-paragraph-packing behavior and are rewritten
  to assert the new one instead. Any test that referenced the removed
  `maxChunkChars` purely as a convenient "large size" fixture value gets a
  local, test-only constant in its place. New tests cover: two substantial
  paragraphs always split into two chunks regardless of fitting the old
  budget together; the real Evicção/Invalidação/OBS section produces three
  chunks.

## Known risks

- A section with several substantial-but-related paragraphs now splits more
  than before. Accepted; see "Accepted cost" above.
- `maxChunkChars` leaving production code means the "how big can one chunk
  get" story is now entirely `minChunkChars`'s merge step plus "a lone
  paragraph is never split" — worth remembering if a future spec reintroduces
  a hard ceiling for some other reason (e.g. a provider-side embedding input
  size limit), rather than assuming today's unbounded-single-paragraph
  behavior is new.

## Out of scope

- Any automatic re-chunk/backfill mechanism for already-imported documents.
- The "sub-heading paragraph" heuristic (rejected alternative).
- Recalibrating `DefaultSufficiency` — tracked separately; see
  [[project_strict_notes_sufficiency_threshold_too_strict]].

## References

- [05-01-hyde-query-enrichment.md](05-01-hyde-query-enrichment.md) — the
  complementary fix for the same user-reported symptom.
- `03-notes-import-and-knowledge-explorer.md` — the chunking contract this
  amends.
- `internal/application/ingest/chunking.go` — `packParagraphs`,
  `mergeUndersized`.
