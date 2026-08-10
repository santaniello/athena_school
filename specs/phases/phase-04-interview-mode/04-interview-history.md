# Phase 4.4 — Interview History

## Goal

User can browse past interviews, review individual reports, and track score evolution per topic over time.

## Tasks

- [ ] History list screen: interviews sorted by date descending
  - Columns: date, topic, mode, score, duration
- [ ] Detail screen: full report (same as post-interview report from 4.3)
- [ ] Per-topic chart: score over time (line chart)
- [ ] Wails binding: `ListInterviews() []InterviewSummary`, `GetInterview(id) InterviewDetail`

## Acceptance Criteria

- Completed interviews appear in the history list immediately
- Clicking an interview opens its full report
- The chart for a topic with 3+ interviews shows a trend line
- History persists across app restarts
