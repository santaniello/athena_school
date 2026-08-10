# Phase 1.3 — Trial

## Goal

New accounts automatically start a 7-day full-access trial. Expiry is visible and enforced.

## Tasks

- [ ] On register: `trial_ends = now + 7 days` set by the auth server
- [ ] App checks plan on every login and displays "Trial — X days remaining" badge
- [ ] On expiry: blocking modal with plans screen (user cannot dismiss without upgrading)
- [ ] Trial grants full access to all features available in the current phase

## Acceptance Criteria

- Badge "Trial — X dias restantes" visible on the main screen after login
- After `trial_ends` passes, a blocking modal appears on next launch
- The modal links to the plans screen; there is no way to close it and keep using the app
