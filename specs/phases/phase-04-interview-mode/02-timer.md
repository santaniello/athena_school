# Phase 4.2 — Timer

## Goal

Each question has a configurable countdown timer visible during the answer phase.

## Tasks

- [ ] Timer options per question: 30s / 1 min / 2 min / no limit
- [ ] Timer configured at session start (applies to all questions in the session)
- [ ] Countdown displayed prominently in the UI during answer input
- [ ] On expiry: partial answer is saved and the session advances to the next question automatically
- [ ] "No limit" mode: timer is hidden; user submits manually

## Acceptance Criteria

- Timer counts down from the configured value and is visible in the answer screen
- When the timer reaches zero, the current answer is submitted automatically
- The saved answer content matches what the user typed before expiry
- Selecting "no limit" hides the timer and requires manual submission
