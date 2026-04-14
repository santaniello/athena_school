# Spec: Progress Tracking

## Goal

Persist scores from study and challenge sessions so the user can see their evolution over time and identify weak areas.

## User Story

> As a developer, I want to see my scores per topic over time, so I can identify where I need to practice more.

## Acceptance Criteria

- [ ] After each study/challenge session, score is saved to local storage
- [ ] `athena progress` prints a summary of scores per topic
- [ ] `athena progress system-design` shows scores for all subtopics
- [ ] Each entry includes: topic, mode (study/challenge), score, date
- [ ] Weak areas (avg score < 6) are flagged with a suggestion

## CLI Usage

```bash
athena progress
athena progress system-design
```

## Storage Format

File: `~/.config/athena/progress.json`

```json
[
  {
    "id": "uuid",
    "topic": "system-design",
    "subtopic": "caching",
    "mode": "challenge",
    "score": 7,
    "max_score": 10,
    "timestamp": "2026-04-03T10:30:00Z"
  }
]
```

## Directory Structure

```
internal/
└── progress/
    ├── store.go         # Load, Save, Append
    ├── report.go        # Aggregate and format report
    └── store_test.go
cmd/athena/
└── cmd_progress.go
```

## Report Format

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Progress Report
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

system-design
  caching          ████████░░  7.8/10  (5 sessions)
  load-balancing   ██████░░░░  6.1/10  (3 sessions)
  sharding         ████░░░░░░  4.2/10  (2 sessions)  ⚠️

⚠️  Weak areas detected:
  - system-design › sharding (avg 4.2)

Suggestion:
  athena study system-design sharding
```

## Implementation Notes

- Use UUID v4 for entry IDs (`github.com/google/uuid`)
- `progress.Store` appends entries; never mutates existing ones
- Progress is written after every scored session (study and challenge)
- The bar chart uses Unicode block characters: `█` for filled, `░` for empty
- Score below 6.0 is flagged as a weak area

## Done When

```bash
$ athena challenge system-design caching  # complete a session
$ athena progress
# → shows a table with caching score recorded
```
