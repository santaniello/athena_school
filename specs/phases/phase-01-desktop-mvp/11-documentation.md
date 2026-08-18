# Phase 1.11 — In-App Documentation

## Goal

A manual inside the app that explains what Athena is for, how study sessions work, and what the Knowledge Engine adds — written for someone using the product, not building it.

## Why it belongs in the app

The README is written for contributors and the `specs/` tree is written for implementers. Neither answers a user's questions: *why does it want a profile before I can study?*, *why does it ask instead of explaining?*, *where do my sessions go?* Those answers only exist in the reader's head today.

## Placement

A `documentation` section in the sidebar **footer group**, next to Settings. It is a reference the user reaches for occasionally, not a workflow step, so it does not belong among the primary roadmap sections.

`AppSection` gains `documentation`, and the footer group holds two rows for the first time — `app-shell.tsx` already maps over `FOOTER_ITEMS`, so no layout change is needed.

## Content model

Content lives in `frontend/src/lib/documentation.ts` as data, not inside JSX: it stays readable, editable without touching layout, and testable on its own.

```ts
export type DocStatus = 'available' | 'planned'

export interface DocTopic { term: string; description: string }

export interface DocSection {
  id: string        // doubles as the table-of-contents anchor
  title: string
  status: DocStatus
  summary: string
  body: string[]    // paragraphs
  topics: DocTopic[]
}
```

Sections, in reading order — purpose before usage:

| id | Title | Status |
|---|---|---|
| `what-is-athena` | Why Athena exists | available |
| `study-sessions` | Study sessions | available |
| `knowledge-engine` | The Knowledge Engine | planned |
| `reference` | Quick reference | available |

**`status` is the point of the model.** The manual documents the Knowledge Engine before it ships, because that is what makes the product make sense as a whole. Describing an unbuilt feature as though it already worked would mislead the reader, so a `planned` section renders a visible badge instead.

## Tasks

- [x] `frontend/src/lib/documentation.ts` — `DocSection`/`DocTopic`/`DocStatus`, the `DOCUMENTATION` manifest, and `plannedSectionIds()`
- [x] `frontend/src/screens/DocumentationScreen.tsx` — layout only: header, anchored table of contents, one block per section with its status badge, prose, and topics as a definition list
- [x] `frontend/src/lib/navigation.ts` — `documentation` added to `AppSection` and to `NAVIGATION` as an unlocked footer row
- [x] `frontend/src/components/app-shell.tsx` — route the section to the real screen rather than `ComingSoonPanel`
- [x] Scrolls inside its own container, matching `StudyChatScreen`'s transcript — the shell's `<main>` is a flex item that must not stretch past the viewport

## Acceptance Criteria

- A **Documentation** row appears in the sidebar footer, above Settings, and is not locked
- Selecting it opens the manual, not the coming-soon panel
- Every section in the manifest renders as a heading, and every paragraph of its prose is on the page
- The table of contents lists one link per section, numbered in reading order, each pointing at that section's anchor
- The Knowledge Engine section shows a **Planned** badge; sections that already ship show none
- Every topic renders as a term with its description
- Section ids are unique, so no contents link jumps to the wrong place
