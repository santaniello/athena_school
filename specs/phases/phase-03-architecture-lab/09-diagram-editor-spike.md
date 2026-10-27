# Phase 3.9 — Diagram Editor Spike

## Goal

Confirm, with a throwaway prototype, that the draw.io online embed works inside the Athena window
and can carry the exam of [3.10](10-diagram-workspace.md). The result is
`ADR-006-diagram-editor.md`, which records the measurements and the integration rules 3.10 builds
on. No production code is delivered by this spec.

## Decisions already made

1. **draw.io is the editor.** It ships the shape libraries the original lab sketch asked for (AWS, GCP, databases, messaging,
   containers) and
   saves the diagram as XML. Excalidraw is not evaluated.
2. **The online embed is the preferred integration** (`embed.diagrams.net`, loaded in an iframe
   and driven by `postMessage`). The self-hosted bundle is not built; it stays recorded in the ADR
   as the fallback if the embed fails the checks below. Athena is not offline-only (OpenRouter is
   a core dependency), so needing the internet is acceptable.
3. **The submission is the diagram XML only.** No image. The evaluator
   ([3.11](11-architecture-evaluator.md)) reads components, labels and links from the XML. The
   limit stays 2 MiB ([3.3](03-attempts-and-session-link.md)).
4. **The latest XML lives in memory on the screen.** The editor sends its changes through
   `postMessage`; the screen keeps the latest XML and nothing is saved to the database before
   submitting. This keeps the exam without drafts ([ADR-005](../../decisions/ADR-005-exercise-engine.md)).
5. **A lost connection does not stop the clock.** The screen shows a connection-lost warning and
   tries to reconnect the editor. If time runs out first, the exam is submitted with the latest XML
   the screen holds. There is no pause.
6. **The editor is checked before an exam starts.** [3.10](10-diagram-workspace.md) loads the
   editor and creates the attempt only when it is ready; on failure it shows "The editor needs an
   internet connection. Try again." and no attempt exists.
7. **The prototype is throwaway.** It runs on a development-only route
   (`import.meta.env.DEV`), is not in the navigation and is removed or rewritten in 3.10. What
   stays is the ADR.

## Questions the spike answers

1. Does the embed load and run inside the Wails webview on Linux, macOS and Windows (iframe,
   Content Security Policy, `postMessage` origin)?
2. Can it be started in embed mode with a fixed configuration: no save, no export or open menus,
   no cloud storage connections, and the shape libraries from the sketch available and searchable?
3. Which events give the latest XML, and how soon after a change does the screen have it? Does an
   exam that ends at the deadline have the last edit?
4. How does the screen detect that the editor stopped responding or lost its connection, and does
   it recover on its own when the connection returns, keeping the diagram?
5. How large is a realistic architecture diagram's XML, and is it well under 2 MiB?
6. Does the XML, as the editor emits it, carry enough (component names, labels, edges with source
   and target) for the evaluator? Compare an example evaluated from the XML alone.
7. What does the embed send to the internet? Record what the page loads and whether diagram
   content leaves the app beyond loading the editor code.
8. What is the fallback to the self-hosted bundle, and what does it cost (size, update process)?

## Deliverables

- `specs/decisions/ADR-006-diagram-editor.md`: the decision, the measurements for the questions
  above, the embed configuration, the `postMessage` events 3.10 uses, the connection-loss rule and
  the fallback.
- A fixed example set under `specs/` or the ADR: two or three real XML outputs (small, typical,
  large), reused by tests in [3.10](10-diagram-workspace.md) and [3.11](11-architecture-evaluator.md).
- Updates to [3.10](10-diagram-workspace.md) and [3.11](11-architecture-evaluator.md) if the
  findings change their text.

## Tasks

- [ ] Prototype on a dev-only route: embed in an iframe with the fixed configuration
- [ ] Measure questions 1 to 5 on the supported operating systems available
- [ ] Check question 6 with an example diagram and the XML alone
- [ ] Check question 7 with the network inspector
- [ ] Write `ADR-006-diagram-editor.md` and the XML examples
- [ ] Adjust 3.10 and 3.11 if needed; remove the prototype
- [ ] Docs: CHANGELOG entry for the ADR

## Acceptance Criteria

- ADR-006 answers each question with a measurement or an explicit "not verified" and the reason.
- It states one decision: embed or fallback to self-hosted, with the condition that would change it.
- It lists the exact events and configuration 3.10 will use, the connection-loss behavior of
  decision 5 and the pre-start check of decision 6.
- The XML examples exist and are within the 2 MiB limit.
- No prototype code remains in the repository.
- Documentation checks pass; no code gate applies since nothing ships.

## Out of scope

- The exam screen, the countdown, submitting and the start flow ([3.10](10-diagram-workspace.md)).
- Self-hosting the editor, Excalidraw, and a custom drawing component.
- Sending an image to the evaluator.
- Saving drafts or resuming a diagram.
