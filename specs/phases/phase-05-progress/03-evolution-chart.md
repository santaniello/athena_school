# Phase 5.3 — Evolution Chart

## Goal

Show, in the topic detail, how the user's results changed over the attempts, using only real
attempts.

## Decisions

1. **Two series:** Quiz (percent of each attempt) and Arch Lab (lab score of each attempt with both
   evaluations: the mean of diagram and defense, 5.1 decision 5). Every attempt of every quiz and challenge of the topic is a point; the
   card's "latest attempt" rule (5.1) does not filter the chart.
2. **X axis = order within the series** (1st, 2nd, 3rd assessment), chronological. Each series is
   numbered on its own. The tooltip names the quiz or challenge and the date.
3. **A dashed line at `passScore`**, the minimum used by the status rule.
4. **Shown only with at least 2 points in total**; otherwise "Not enough assessments yet". A series
   with no points is left out of the legend.
5. **Y axis 0–100**, fixed, so two topics are comparable at a glance.

## Domain

`internal/domain/progress`: `BuildEvolution(in TopicInput) Evolution` (two ordered point lists).

## Interface

`GetProgressTopic` gains the evolution. Frontend: a line chart in the topic detail, theme colors
from the design tokens, readable in light and dark.

## Acceptance Criteria

- A quiz taken 3 times and a challenge taken once give a 3-point Quiz series and a 1-point Lab
  series.
- A topic with a single attempt shows the "Not enough assessments yet" message.
- Same quality gate as 5.1.
