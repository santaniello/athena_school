# Phase 3.24 — Mind Map from a Study Session

## Goal

Let the user turn a study session into a **mind map**: a visual summary of the session's content
that can be opened, edited by hand, regenerated from the session and deleted. It is requested the
same two ways as the quiz: from the study chat with an **offer** ([3.23](23-interactive-quiz-chat-offer.md))
or directly from a new **Mind Map** menu section, choosing the session.

The map is drawn and edited with the draw.io embed that the Architecture Lab already uses
([ADR-006](../../decisions/ADR-006-diagram-editor.md)). No new dependency is added.

## Decisions

1. **Separate bounded context.** `domain/mindmap` and `application/mindmap` import neither
   `exercise`, `quiz` nor `study`. Session content is read through ports implemented by adapters in
   `infrastructure/mindmapsource`, the same way `quizsource` does it
   ([ADR-007](../../decisions/ADR-007-interactive-quiz.md)).
2. **The model returns a tree, never XML.** One `llm.Chat` call (`TaskMindMap`, no retry) returns a
   JSON tree: a root title and nested children, each with a short label. A deterministic renderer
   turns the tree into mxGraphModel XML, like `architecture/starter_diagram.go`. Ids, coordinates
   and styles are ours; labels are XML-escaped. Limits: depth, total nodes and label length are
   capped, and a tree over a limit is `ErrInvalidMindMapTree`. Nothing is stored when generation
   or rendering fails.
3. **Content fed to the model** is the same as the quiz's: session topic and goal, the conversation
   summary, the profile (area, level, assistant language) and note excerpts from the knowledge
   retriever. The map is written in the assistant language.
4. **Stored data.** A mind map keeps `treeJSON` (what the model produced) and `diagramXML` (what the
   editor shows and saves), plus title, the session title snapshot, `messageID` (nullable, set when
   it came from an offer) and timestamps. `diagramXML` is capped at 2 MiB, as submissions are.
5. **Update has two meanings.** *Save* replaces `diagramXML` with the user's manual edit and keeps
   `treeJSON`. *Regenerate* runs generation again from the session's current content and replaces
   both, keeping the id. Regenerate asks for confirmation because it discards manual edits. A failed
   regeneration leaves the map untouched.
6. **Delete is explicit** from the catalog and from the map screen, with confirmation.
7. **Lifetime follows the session.** `mind_maps.session_id` has `ON DELETE CASCADE`: deleting a
   session deletes its mind maps. This differs from quizzes on purpose (user's decision).
8. **Chat offer.** A third marker, `[[ATHENA_MINDMAP_OFFER]]`, with the same rule as the others: the
   tutor emits it only if the user explicitly asks for a mind map. `OfferKindMindMap` is added, and
   the precedence when a reply carries several markers is exercise, then quiz, then mind map. The
   marker is hidden while streaming and stripped from the stored text. The tutor's instruction
   states that a quiz or exercise request must not produce this marker.
9. **One pending offer per session across all kinds.** `mind_map_offers` has `UNIQUE (session_id)`
   and cascade foreign keys; `studyoffers.Gate` gains the mind map side so `study` imports nothing
   from this context. The composer stays blocked while any offer is pending.
10. **Accept is idempotent and serialized**, copying `quiz.OfferService.Accept`: a per-session
    lock, a lookup by `messageID` before generating, and the offer deleted after the map is saved.
11. **Direct generation.** The Mind Map screen lists maps and has a dialog to pick a session and
    generate. Only one generation runs at a time (`generationSlot`) and it can be cancelled.
12. **Frontend.** `'mindmap'` is added to `AppSection` and `NAVIGATION`. `DiagramEditor` moves from
    `components/lab/` to a shared `components/diagram/` folder and the Lab imports it from there. The
    map screen shows the "editor needs an internet connection" state the Lab already has. Returning
    from a map opened from the chat goes back to the session (`mindMapReturn`, like `quizReturn`),
    and leaving with unsaved edits asks first. Strings use a `mindmap` i18n namespace (en, pt).
13. **Model setting.** `TaskMindMap` gets its own entry in Settings → Models.

## Out of scope

Export to image or PDF, merging two maps, collaborative editing, and offline rendering.

## Tests

TDD with GivenWhenThen and Mockery mocks. Mutation testing covers `domain/mindmap` and
`application/mindmap`. Required cases: renderer output is well-formed and escaped, over-limit and
malformed trees are rejected, a failed generation stores nothing, accept is idempotent, regenerate
keeps the id and failure leaves the map untouched, deleting a session cascades, the marker is
filtered while streaming, and the precedence of markers holds.

## Documentation

`CHANGELOG.md [Unreleased]`, `README.md`, `frontend/src/lib/documentation.ts`, and an ADR-008 for
the shared diagram editor and the tree-to-XML decision.
