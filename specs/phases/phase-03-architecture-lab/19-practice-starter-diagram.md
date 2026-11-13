# Phase 3.19 — Practice Starter Diagram

## Goal

A practice challenge ([3.18](18-practice-challenges.md)) starts from a diagram of the system as it
is today, with the problem visible in it: a product API that calls a slow price API that reads a
relational database, for example. The user edits that diagram to apply what the statement asks
for, instead of drawing from a blank board. The scenario text gets shorter, because the diagram
carries the "who calls whom".

## Findings that shape the design

- The editor is the draw.io embed. It already loads a diagram through the `load` message, but the
  initial XML is hardcoded to an empty model, and editable mode ignores the `xml` prop
  (`diagram-editor.tsx`, `useRef(readOnly ? xml : '')`).
- The model never writes XML today. Evaluation and the oral defense read a text summary
  (`SummarizeDiagram`: components, containers, connections), not the XML or an image.
- `SummarizeDiagram` is read-only and lossy. Nothing in Go generates mxGraph XML.
- The diagram lives only in the iframe until submit. There is no draft persistence.

## Decisions

1. **Practice only.** `INTERVIEW` challenges still start blank. Interviews ask for a design from
   scratch; a starting diagram would defeat that.
2. **The model describes, the code draws.** The model does not write draw.io XML: coordinates,
   styles and ids would often come out broken or ugly. It returns a small structure instead:
   `nodes` (id, label, kind) and `edges` (from, to, label). A new Go renderer in
   `internal/application/architecture` turns it into a valid `mxGraphModel` with a simple layered
   layout (left to right, by distance from the entry node). The XML is always valid and the layout
   is ours.
3. **Node kinds are a closed set.** `client`, `service`, `database`, `cache`, `queue`, `external`,
   `other`. Each maps to a draw.io style. An unknown kind becomes `other`. The prompt forbids
   `cache` and `queue` nodes unless the scenario already has one, so the diagram never contains
   the fix.
4. **Limits.** 2 to 10 nodes, at most 15 edges, labels trimmed to the existing `MaxLabelLength`.
   Edges must reference existing nodes and nodes must be unique. A structure that breaks a rule
   fails the generation like any other invalid model output.
5. **The diagram is part of the challenge.** `Challenge` gains `StarterDiagram` (mxGraphModel XML,
   empty for interviews). It is stored in a new `challenges.starter_diagram TEXT NOT NULL DEFAULT ''`
   column and loaded into the editor when an attempt starts. It is not copied to the attempt: the
   challenge is the single source. The exam screen reads it from the attempt view
   (`statement.starterDiagram`), because the board's editor is created after the attempt exists and
   the screen has no other challenge DTO at that point.
6. **The editor starts from it.** `DiagramEditor` seeds `latestXml` from the `xml` prop in editable
   mode too, so `init` loads it, and a recovered iframe reloads the user's latest work as today.
   `LabExamScreen` passes `attempt.starterDiagram` to the board's editor.
7. **An untouched diagram is not an answer.** `diagramHasContent` is true as soon as the seed
   loads, so the submit button would enable at once. Submit stays disabled until the diagram
   differs from the seed, compared on the parsed components and connections, not on raw XML,
   because draw.io rewrites the XML when it loads it.
8. **Evaluation judges the change.** `BuildEvaluationPrompt` receives the summary of the starter
   diagram and labels it as the baseline the user started from. The evaluator is told to score what
   the user added, removed or rewired relative to the baseline, and not to credit components that
   were already there. Interview evaluation is unchanged (no baseline).
9. **The oral defense and the hints need no change.** The defense uses the submitted summary. Hints
   already receive the live board.
10. **The result view is unchanged.** It shows the submitted diagram read-only. Showing the
    starter next to it is a follow-up.
11. **Existing practice challenges have no starter.** The column defaults to empty and they open
    blank, as they do today.
12. **Out of scope:** draft autosave to the database, a visual diff between starter and answer,
    a starter for interviews, hand-editing the starter diagram, concept suggestions.

## Changes to earlier specs

- **3.1:** `Challenge` gains `StarterDiagram`, required for a generated `PRACTICE`, empty for
  `INTERVIEW`.
- **3.14 / 3.18:** the practice generation output format gains the `diagram` structure, and parsing
  renders it.
- **3.5 (evaluation):** the evaluation prompt may carry a baseline summary.

## Build order

1. Renderer: structure to `mxGraphModel`, with tests that feed the output to `SummarizeDiagram` and
   check components and connections round-trip.
2. Domain field, migration, repository, DTO and wailsjs models.
3. Practice prompt and parse: the `diagram` structure, its limits and the "no fix in the diagram"
   rule.
4. Evaluation prompt with the baseline.
5. Frontend: editor seeding, the "changed from the seed" submit rule, tests.

## Verification

Unit tests per layer, then the real app: generate a practice challenge about "cache aside" and
check that the diagram opens with the slow dependency and without a cache node; the submit stays
disabled until a change; after adding a cache and submitting, the evaluation credits only the
change; an interview still opens blank.
