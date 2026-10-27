# Phase 3.7 — Challenge Catalog (Architecture Lab entry screen)

## Goal

The first visible part of the Architecture Lab: a screen listing the user's challenges, with
search, filters and progress at a glance, plus the **Generate challenge with AI** action that
fills the catalog from studied sessions ([3.6.1](06-01-challenge-generation.md)).

It reads the data of [3.1](01-challenge-domain-and-persistence.md) to
[3.6](06-completion-rules.md). The lab screen layout is [3.8](08-lab-layout-sketch.md); starting an exam arrives with the
editor in [3.10](10-diagram-workspace.md). This spec does not start attempts.

## Layout

```
┌ Architecture Lab ──────────────────────────────── [ Generate challenge with AI ] ┐
│  12 challenges      3 completed      2 in progress                                  │
│  [ Search ...        ]  [Status ▾]  [Difficulty ▾]  [Topic ▾]                       │
│ ┌─────────────────────────────────────────────────────────────────────────────┐ │
│ │ Cache and invalidation in a catalog                          Intermediate   │ │
│ │ Caching · 45 min · 2 attempts                  Best 86 · Needs improvement  │ │
│ │                                                              [ Start again ]│ │
│ ├─────────────────────────────────────────────────────────────────────────────┤ │
│ │ Rate limiter for a public API                                Beginner       │ │
│ │ Rate limiting · 30 min · not started                              [ Start ] │ │
│ └─────────────────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────────┘
```

A structure reference only: colors, components and spacing follow the existing frontend.

## Decisions

1. **The sidebar item "Challenge" becomes "Architecture Lab"** and is unlocked: new label and
   description, `status: 'unlocked'`, same icon and position, same `challenge` section id in
   `lib/navigation.ts`. Progress, Flashcards and Interview stay locked.
2. **The catalog is the list of choices.** "Choose a challenge" is the list itself: there is no
   separate picker. The two entry actions are picking a row and **Generate challenge with AI**.
3. **A row shows only what does not give away the exam:** title, topic, difficulty, time limit,
   status, attempt count and the best diagram score. Never the scenario, requirements or rubric.
4. **Status labels.** `Not started`, `In progress`, `Needs improvement`, `Completed`, from
   [3.6](06-completion-rules.md). There is no "awaiting defense" status: the evaluation and the
   tutor's questions run in the same sitting as the exam ([3.10](10-diagram-workspace.md)), and
   only a finished attempt is kept.
5. **Counts at the top:** total challenges, completed, in progress. They follow the filters of
   the list's scope, not the text search.
6. **Filters:** text search over title, scenario and topic (the scenario is searched but never
   displayed), status, difficulty and topic. The topic options are the distinct topics present.
   They combine with AND. Status is derived, so the backend filters by it after deriving; the
   result is small enough to page in memory.
7. **Order:** most recently updated first; a challenge with an attempt running goes to the top.
8. **Generation dialog.** Opened by **Generate challenge with AI**:
   - a searchable list of the user's study sessions, most recently started first, each with its
     topic, folder name and date;
   - a difficulty selector, pre-selected from the profile's experience level;
   - **Generate** is enabled once a session is chosen. While running, the dialog shows progress and
     a **Cancel** button that cancels the call. On success it closes, the list reloads and the new
     challenge is highlighted. On failure the dialog stays open with the error and a retry.
   - With no study sessions at all, the dialog shows an explanation: "Study a topic first. A
     challenge is generated from a session you have already studied." and no Generate button.
9. **Start is not wired here.** Each row's **Start** / **Start again** is shown disabled with the
   tooltip "Available with the diagram editor" until [3.10](10-diagram-workspace.md), which owns
   the rules dialog, the call to start the attempt and the in-progress warning. An exam cannot
   start before the editor exists, and this keeps these specs free of throwaway screens.
10. **Empty and error states.** An empty catalog explains how to create one and offers the
    generate action; no results for the filters shows a clear-filters action; a load error shows a
    retry.
11. **Business logic stays in the backend.** The frontend renders what the backend returns and
    never derives a status itself.
12. **No data compatibility to keep.**

## Backend

Read models and use cases in `internal/application/exercise`; bindings are thin.

- `CatalogItem{ChallengeID, Title, Topic, Difficulty, TimeLimitMinutes, Status ProgressStatus,
  Attempts, BestDiagramScore}` and
  `Catalog{Total, Completed, InProgress, Items}`.
- `CatalogFilter{Query, Status, Difficulty, Topic}`.
- `CatalogService.ListCatalog(ctx, CatalogFilter) (Catalog, error)`: lists challenges with the SQL
  filters of [3.1](01-challenge-domain-and-persistence.md), then derives each one's progress with
  `DeriveProgress` ([3.6](06-completion-rules.md)) from its attempts and filters by status in
  memory.
- `CatalogService.ListTopics(ctx) ([]string, error)` for the topic filter.
- `SessionService.ListSessionsForGeneration(ctx) ([]SessionOption, error)`: **all** study
  sessions across folders, newest first, with topic, folder name and start date. The existing
  `ListStudySessionsByFolder` lists one folder at a time and is not reused. It is a read-only
  query and returns no message content.
- To avoid a query per challenge, attempts, evaluations and outcomes needed for the catalog are
  loaded in a fixed number of queries (a repository method over all challenge ids).

Bindings in `internal/interfaces/desktop/exercise.go`: `ListChallengeCatalog(filter)`,
`ListChallengeTopics()` and `ListSessionsForGeneration()`; `GenerateChallenge` already exists
from 3.6.1. `wailsjs` regenerated.

## Frontend

- `lib/navigation.ts`: the `challenge` item as in decision 1.
- `frontend/src/screens/ArchitectureLabScreen.tsx`: the catalog, reading from a
  `useChallengeCatalog` hook that exposes `catalog`, `loading`, `error` and `reload`, and ignores a
  response that no longer matches the current filters.
- `components/challenge-row.tsx`, `components/generate-challenge-dialog.tsx`.
- `lib/exercise.ts`: the binding wrappers and the DTO types.
- `AppShell` renders `ArchitectureLabScreen` for the `challenge` section instead of
  `ComingSoonPanel`.
- All UI text is in English, like the rest of the app.

## Tasks

Each slice is TDD (Go tests, Vitest for the frontend), keeps the build green and is committed on
its own.

- [ ] Backend: `ListSessionsForGeneration` read use case and its SQL
- [ ] Backend: bulk loading of attempts and evaluations for a set of challenges
- [ ] Backend: `ListCatalog` (counts, derived status, status filter, order,
      paging) and `ListTopics`
- [ ] Bindings and `wailsjs` regenerated; mocks regenerated
- [ ] Frontend: navigation item unlocked and renamed; `AppShell` routing
- [ ] Frontend: `lib/exercise.ts` and `useChallengeCatalog`
- [ ] Frontend: catalog screen with counts, rows, search and filters, empty, no-results and error
      states
- [ ] Frontend: generation dialog (session list with search, difficulty, progress, cancel, error,
      no-sessions state)
- [ ] Docs: CHANGELOG, README and `lib/documentation.ts` describe the Architecture Lab

## Acceptance Criteria

- The sidebar shows **Architecture Lab**, unlocked, and opening it shows the catalog.
- With no challenges the screen explains how to create one and the generate action works.
- **Generate challenge with AI** lists all study sessions newest first, filters them by the search
  text, preselects the difficulty from the profile and, on success, closes with the new challenge
  visible and highlighted; cancel stops the call; a failure keeps the dialog open with a retry.
- With no study sessions the dialog says to study first and offers no Generate button.
- A row never shows the scenario, requirements or rubric.
- Status, attempt count and best score on each row match the backend.
- The counts at the top match the list; search and the three filters combine with AND, and
  searching by words that appear only in the scenario still finds the challenge.
- **Start** is disabled with its tooltip; nothing starts an attempt from this screen.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  application/domain code, lint and vulnerability checks, and the frontend tests, coverage, lint,
  typecheck, `npx prettier --check .` and Stryker on the changed files pass.

## Out of scope

- The lab screen layout ([3.8](08-lab-layout-sketch.md)), the rules dialog, starting an attempt and
  the discard warnings ([3.10](10-diagram-workspace.md)).
- Result and history screens ([3.13](13-evaluation-and-history-screen.md)).
- Generating from the chat or avoiding duplicates ([3.14](14-ai-challenge-generation.md)).
- Editing, deleting, archiving or manually creating challenges.
- A "My solutions" tab and any view of past attempts.
