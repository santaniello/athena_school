# Phase 1.3 — Home Screen & Navigation Shell

## Goal

Replace the placeholder `MainScreen` with a real app shell (sidebar + topbar) that stays mounted for the rest of the app's lifetime, and a Home screen that greets the user and gets them into a study session. The sidebar's navigation structure is designed for the full product roadmap (Phases 1–7) up front, so later phases only flip a status flag instead of redesigning navigation.

## Flow

```text
Auth + key gate + onboarding complete → app shell mounts → Home (default section)
Login/Register/Reset/KeyGate/Onboarding screens are unaffected — they keep the existing
full-screen centered-card pattern (see auth-layout.tsx); the shell wraps nothing before
the app is entered.
Selecting a locked sidebar section → ComingSoonPanel (title + "Planned for Phase N")
Selecting an unlocked section → that section's own screen
```

## Navigation IA (full roadmap, reference)

The sidebar is built once, for the entire roadmap. Sections belonging to unbuilt phases are visible but locked, not hidden — this is the app's de facto product tour until they ship.

| id | Label | Icon | Ships in | Badge | Notes |
|---|---|---|---|---|---|
| `home` | Home | `Home` | Phase 1 (this doc) | — | default section, unlocked from day one |
| `study` | Study | `BookOpen` | Phase 1 ([06-study-mode.md](06-study-mode.md)) | — | locked until 06 ships |
| `knowledge` | Knowledge | `FolderTree` | Phase 2 | pending-draft count | folds Explorer + Review queue into tabs of one section |
| `challenge` | Challenge | `Target` | Phase 3 | — | |
| `progress` | Progress | `TrendingUp` | Phase 3 | — | folds Gap Detection in as a "Gaps" tab |
| `flashcards` | Flashcards | `Layers` | Phase 3 | due-today count | |
| `interview` | Interview | `MessagesSquare` | Phase 4 | — | Phase 6 voice becomes a mode toggle inside this section, not a new row |
| `settings` | Settings | `Settings` | Phase 1 ([08-settings.md](08-settings.md)) | — | pinned to sidebar footer, locked until 08 ships |

There is no `plans` row and no plan/trial tag in the sidebar footer: the project is not being commercialized for now, so Phase 5 (`phase-05-comercializacao/`) is out of scope until that changes. If it's revived later, it slots back in as a footer nav row the same way `settings` does.

Later additions that reuse this same manifest instead of new rows: Knowledge Graph (Phase 7) becomes a tab inside `knowledge`; Algorithm Mode (Phase 7) is the first entry gated by the user's profile area (IT only) rather than by phase — the manifest entry shape includes an optional `visibleIf(profile)` predicate from the start so that phase doesn't need to touch the shell.

Locked sections use a single, temporal lock: section not built yet, shown muted with a `Lock` icon and "Planned for Phase N" copy. (A second, plan-gated lock concept was planned for `phase-05-comercializacao/01-plans-feature-gating.md` — not relevant while the project isn't commercialized.)

## UI / Layout

```text
┌───────────────────────────────────────────────────────┐
│ Athena                                    [Home]       │  ← topbar: section title + contextual slot
├───────────┬───────────────────────────────────────────┤
│ Home      │                                           │
│ Study 🔒  │        Good evening, {name}.               │
│ Knowledge🔒│       {assistantName} is ready when       │
│ Challenge🔒│              you are.                     │
│ Progress🔒│                                           │
│ Flashcards🔒│      [ Start a study session ]            │
│ Interview🔒│                                           │
│           │                                           │
├───────────┤                                           │
│ Settings🔒│                                           │
│ {name}    │                                           │
│ [Logout]  │                                           │
└───────────┴───────────────────────────────────────────┘
```

- The topbar's contextual right slot is empty on Home. This deliberately reinterprets the original "Progress / Session / Status" bottom bar (`specs/Athena.md` §44) as a contextual slot instead of permanent global chrome — there is nothing to show there on Home in Phase 1; it gets populated later (elapsed time, streaming indicator) only while inside Study/Challenge/Interview.
- Locked rows are dimmed (`text-muted-foreground`), keep a right-aligned `Lock` icon, and show a tooltip on hover/focus. They remain clickable — clicking navigates to `ComingSoonPanel`, never a dead click.
- Home content for Phase 1 is intentionally minimal: personalized greeting + one primary CTA. No progress bars, no knowledge counts, no flashcard stats — those features don't exist yet, and the locked sidebar rows already communicate "coming soon." Nothing fabricated is shown in their place.
- The CTA ("Start a study session") triggers the same section-navigation as clicking `study` in the sidebar, so its behavior always matches the manifest's current status for `study` (locked → `ComingSoonPanel`; unlocked once 06 ships → starts a session). This keeps this doc's acceptance criteria self-contained regardless of whether 06/08 have shipped yet.
- Visual language: reuse `AthenaLogo`, `GreekKeyDivider`, Cinzel headings, existing oklch tokens (`--background`, `--primary`, `--card`) from `frontend/src/style.css`. New shadcn primitives needed beyond the existing `button`/`card`/`input`/`label`/`select`/`alert`: a sidebar/nav-menu block, `tooltip`, `badge`.

## Home Widget Contract (for future phases, not populated here)

Home defines a generic slot so later phases don't force a layout redesign:

```ts
interface HomeWidget {
  title: string
  icon: LucideIcon
  primaryStat: string
  linkTo: AppSection
}
```

Phase 2/3 specs ([03-progress-tracking.md](../phase-03-learning-intelligence/03-progress-tracking.md), [04-gap-detection.md](../phase-03-learning-intelligence/04-gap-detection.md), [05-flashcards.md](../phase-03-learning-intelligence/05-flashcards.md), knowledge specs) are each responsible for registering their own widget when they ship. This doc ships the slot with zero widgets registered — Home shows only the greeting and CTA.

Session history/resuming is intentionally not addressed here — that belongs to whatever [06-study-mode.md](06-study-mode.md) decides.

## Data / Frontend Architecture

- `frontend/src/lib/navigation.ts` — the manifest: `{ id, label, icon, group, phase, status, badge?, visibleIf? }[]`.
- `App.tsx`'s `View` union collapses its current `'main'` state into `'app'`. A new `AppSection` union (`'home' | 'study' | 'knowledge' | 'challenge' | 'progress' | 'flashcards' | 'interview' | 'settings'`) lives inside a new `AppShell` component and drives which section content renders.
- No router library: this is a single-window desktop app with no URL bar and no deep-linking requirement, so the existing hand-rolled state machine is extended rather than replaced. Revisit only if multi-window or deep-linking becomes a real requirement.

## Wails Bindings (Go)

```go
func (a *App) GetProfile() (UserProfile, error) // reads the saved UserProfile back for the greeting; SaveProfile exists, nothing reads it back today
func (a *App) Logout() error                    // clears the local session (~/.athena/session.json); no session-clear binding exists today
```

## Tasks

- [x] `frontend/src/lib/navigation.ts` — full navigation manifest (all 8 sections, all phases)
- [x] `frontend/src/components/app-shell.tsx` — sidebar + topbar layout, owns `AppSection` state
- [x] `frontend/src/components/nav-item.tsx` — sidebar row incl. locked state, tooltip
- [x] `frontend/src/components/coming-soon-panel.tsx` — generic locked-section placeholder
- [x] `frontend/src/screens/HomeScreen.tsx` — greeting + CTA, replaces `MainScreen.tsx`
- [x] Go: `GetProfile()` binding
- [x] Go: `Logout()` binding (clears local session)
- [x] `frontend/src/App.tsx`: rename `'main'` → `'app'`, mount `AppShell` instead of `MainScreen`
- [x] Remove `MainScreen.tsx` and its test
- [x] Add shadcn primitives: `tooltip`, `badge` (badge unused for now — no nav badges exist until Phase 2's Knowledge review queue)
- [x] Tests: manifest completeness (one row per known feature), locked-row navigates to `ComingSoonPanel`, greeting renders profile name/assistantName, CTA reflects `study` status, logout clears session and returns to login

## Acceptance Criteria

- After onboarding (or on a returning login), the user lands on Home inside the app shell — not the old placeholder screen
- Sidebar shows all 8 sections; only `home` is unlocked at this phase's ship time
- Clicking a locked section shows `ComingSoonPanel` with a "Planned for Phase N" message, never a dead click or hidden item
- Home greeting uses the saved profile's `name` and `assistantName`
- "Start a study session" CTA behaves consistently with the `study` row's manifest status
- No fabricated data (progress, knowledge counts, flashcard stats) appears anywhere on Home
- Logout clears the local session and returns to the login screen
- Auth, key-gate, and onboarding screens are visually and behaviorally unchanged (full-screen, no shell)
- Restarting the app after login returns directly to Home inside the shell
