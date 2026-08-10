# Phase 5.1 — Plans & Feature Gating

## Goal

Features are restricted by plan; locked features are clearly indicated in the UI.

## Plan Tiers

```text
Essencial: Study Mode + basic Knowledge Base + notes import
Pro:       + Challenge + Interview + Gap Detection + Flashcards + premium models
Expert:    + future features + early access + priority support
```

## Tasks

- [ ] `Plan` type returned by `GET /account/plan` on the auth server
- [ ] `internal/application/licensing/` — permission checker: `CanAccess(feature string) bool`
- [ ] Feature constants: `feature.Challenge`, `feature.Interview`, `feature.GapDetection`, etc.
- [ ] Wails bindings: `GetCurrentPlan() Plan`, `CanAccess(feature string) bool`
- [ ] UI: locked features show a lock icon + "Available on Pro plan"
- [ ] Plan cached locally; refreshed on login and every 24h

## Acceptance Criteria

- An Essencial user cannot start a Challenge session (lock icon shown)
- A Pro user can start a Challenge session (no lock)
- `CanAccess("challenge")` returns `false` for Essencial, `true` for Pro and Expert
- Lock indicators do not appear on features the user has access to
