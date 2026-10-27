# Phase 3.8 — Lab Layout

## Goal

Build the exam screen of the Architecture Lab as a layout, and approve it before any editor is
wired in. It follows the structure of the author's original sketch (not kept in the repository): the
statement on the left, a large board in the center, the tutor and the rubric on the right, a top
bar with the countdown and the actions.

The sketch was a **structural reference only**: it did not use Athena's palette or components, so
colors, spacing, icons and controls follow the existing frontend (shadcn/ui, tokens, resizable
panels as in the Sources panel). The diagram editor is [3.10](10-diagram-workspace.md); here the
center is a placeholder. Nothing in this spec starts, submits or discards an attempt.

## Layout

```
┌ Architecture Lab › Caching › Cache and invalidation in a catalog ─────────────────┐
│ Cache and invalidation in a catalog [Intermediate] [Caching]      ⏱ 44:12  [Leave]│
│                                                    [ Finish and evaluate ]        │
├──────────────────┬─────────────────────────────────────┬──────────────────────────┤
│ Statement        │                                     │ Tutor Athena             │
│                  │                                     │  Hints left: 3           │
│ Scenario         │        Board (placeholder)          │  [ Ask for a hint ]      │
│ ...              │                                     │ ──────────────────────── │
│ Functional       │                                     │ Rubric                   │
│ requirements     │                                     │  Overall architecture 20 │
│ Non-functional   │                                     │  Scalability          20 │
│ Constraints      │                                     │  Consistency   ★ 20      │
│ Objectives       │                                     │  ...                     │
└──────────────────┴─────────────────────────────────────┴──────────────────────────┘
```

## Decisions

1. **Three panels, always visible, no tabs.** Statement, board and tutor/rubric are on screen at
   once. The sketch's top tabs (Statement, Whiteboard, My solutions, Resources, Discussion) are
   not built: there is nothing to switch between during an exam, and past attempts must not be
   reachable mid-exam. The history lives in [3.13](13-evaluation-and-history-screen.md).
2. **Resizable and collapsible side panels**, so the board can take the screen. Same panel
   library and interaction as the Sources panel. The board never collapses.
3. **Top bar:** breadcrumb (Architecture Lab › topic › title), title, difficulty and topic
   chips (no tags: [3.1](01-challenge-domain-and-persistence.md)), the countdown, **Leave** and
   **Finish and evaluate**.
4. **Countdown, not a stopwatch.** It shows the time left as `mm:ss` (`h:mm:ss` over an hour),
   computed from the attempt's deadline and the current time, and calls `onExpire` once at zero. It
   turns to a warning style at 5 minutes left and to an urgent one at 1 minute, with no sound.
   There is **no Pause button**: the clock never stops ([ADR-005](../../decisions/ADR-005-exercise-engine.md)).
   The component is pure presentation; ending the exam is the caller's decision, as the backend
   owns the clock ([3.4](04-exercise-service.md)).
5. **Statement panel.** The scenario as text and the four lists (functional requirements,
   non-functional requirements, constraints, objectives), each with a heading. No deliverables
   checklist and no "tutor tip" card, and nothing about hints inside the statement.
6. **Tutor panel, empty until asked.** At the top a short sentence, "Ask for a hint if you get
   stuck.", a **Hints left** counter out of 3 and an **Ask for a hint** button; hints appear
   below as messages. At zero hints the button is removed and the counter says none are left. The
   hint behavior itself is [3.10.1](10-01-tutor-hints.md); here the panel takes its messages and
   counter as props. While the exam is running there is no message input: the user cannot chat
   freely with the tutor, only request a hint.
7. **Rubric panel.** Below the tutor: one row per criterion with its name, a one-line description,
   its weight and a marker on mandatory ones. No percentage, level or score is shown while the
   exam runs. The sketch's History tab is not built.
8. **Leave** opens a confirmation: "Leaving discards this exam. It will not be graded." with
   **Cancel** and **Leave**. Confirming calls a handler the caller provides; this spec connects it
   to nothing.
9. **Finish and evaluate** is a button that calls a handler the caller provides. It is shown
   enabled only when the caller says the board has content.
10. **Props, not fetching.** The layout component receives the statement, the rubric criteria, the
    deadline, the hint data and the handlers as props, and fetches nothing. The caller in
    [3.10](10-diagram-workspace.md) reads the attempt through `GetAttempt` ([3.4](04-exercise-service.md))
    and the challenge.
11. **Desktop widths only,** with a minimum window width below which the side panels start
    collapsed. No mobile layout.
12. **How it is verified now.** A development-only preview route renders the layout from fixtures
    (a realistic Intermediate challenge, a 45-minute deadline, three hints, five base criteria plus
    two challenge ones, one mandatory). It exists only when `import.meta.env.DEV` is true, is not
    in the navigation and creates no attempt.
13. **No data compatibility to keep.**

## Frontend

- `frontend/src/components/lab/lab-layout.tsx`: the three-panel shell and top bar, props as in
  decision 10.
- `lab-countdown.tsx`: the countdown (decision 4), with an injectable clock for tests.
- `lab-statement-panel.tsx`, `lab-tutor-panel.tsx`, `lab-rubric-panel.tsx`, `lab-leave-dialog.tsx`.
- `lab-preview-fixtures.ts` and the dev-only preview route in `AppShell`, tree-shaken from
  production builds.
- Types for the props live in `frontend/src/lib/lab.ts`.
- All UI text is in English.

No backend and no Wails binding in this spec.

## Tasks

Each slice is TDD (Vitest), keeps the build green and is committed on its own.

- [ ] `lab-countdown`: formatting, warning and urgent thresholds, `onExpire` called once,
      expired deadline at mount
- [ ] `lab-statement-panel`: scenario and the four lists, empty lists hidden
- [ ] `lab-rubric-panel`: name, description, weight, mandatory marker, no score shown
- [ ] `lab-tutor-panel`: initial text, counter, button, hint messages, none-left state, no free
      input
- [ ] `lab-leave-dialog`: confirmation, cancel, confirm
- [ ] `lab-layout`: top bar, three resizable and collapsible panels, board placeholder, Finish
      enablement, handlers
- [ ] Dev-only preview route from fixtures, excluded from production
- [ ] Docs: CHANGELOG

## Acceptance Criteria

- The preview shows the statement on the left, a large placeholder board in the center, and the
  tutor above the rubric on the right, with the top bar as described, and it follows the existing
  theme and components rather than the sketch's colors.
- The countdown shows the time left, switches to the warning style at 5 minutes and the urgent
  style at 1 minute, calls `onExpire` exactly once at zero, and there is no pause control.
- The statement shows the scenario and the four lists; the rubric shows each criterion with its
  weight and a marker on the mandatory one, and never a score.
- The tutor panel shows the hint counter and button, shows hints as messages, drops the button at
  zero hints, and has no free-text input.
- **Leave** asks for confirmation and calls the handler only when confirmed.
- Side panels resize and collapse and the board keeps the remaining space; there are no tabs.
- The preview route is absent from a production build and creates no attempt.
- Frontend tests, coverage ≥ 80%, lint, typecheck, `npx prettier --check .` and Stryker on the
  changed files pass.

## Out of scope

- The diagram editor, the real clock source, starting, submitting, discarding and the start
  dialog ([3.10](10-diagram-workspace.md)).
- Hint generation and its limit enforcement ([3.10.1](10-01-tutor-hints.md)).
- The evaluation, defense and result views ([3.11](11-architecture-evaluator.md),
  [3.12](12-architecture-defense.md), [3.13](13-evaluation-and-history-screen.md)).
- Tabs, a pause button, a history tab, a deliverables checklist, Resources and Discussion.
- Mobile layout and the sketch's palette.
