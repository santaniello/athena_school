# Phase 5.2 — Topic Detail

## Goal

Selecting a topic in Progress opens, on the right, what supports its status: the related
activities, each quiz's and challenge's results, and what the learning evidence shows: concepts
understood, difficulties, and concepts only explained so far. Still no LLM call.

## Decisions

1. **Related activities:** quiz count, Arch Lab count (5.1 decision 2) and the count of concepts
   worked on in the session. The mockup's "sessions" counter is dropped: a topic is one session.
2. **Results.** One row per quiz: its name ("Quiz N · difficulty", as the materials panel), latest
   percent, number of attempts. One row per challenge: title, latest diagram score, latest defense
   score (or "defense pending"), result (Passed / Failed / Pending), number of attempts. A row opens
   the item the way the materials panel does: a quiz opens the quiz, a challenge shows the exam rules
   first. When it ends, the user comes back to Progress (a `progress` return, next to `menu` and
   `study`).
3. **Concepts understood** = concepts in `DEMONSTRATED`.
4. **Difficulties** = concepts in `PARTIALLY_UNDERSTOOD` or `NEEDS_REVIEW` (the latter marked "to
   review"), plus the rubric criteria of the latest evaluated attempt of each challenge whose level
   is below `exercise.MandatoryMinLevel` (2) in its diagram or defense evaluation (criteria flagged
   `InsufficientEvidence` are skipped). Each difficulty says where it comes from ("Tutor", "Arch Lab:
   <challenge title>"). Quiz questions carry no concept, so quizzes add no named difficulty.
5. **Explained, not yet assessed** = concepts in `INTRODUCED`. The tutor explaining a concept is
   never shown as understanding (4.2 decision 1).
6. **Concept display name.** `session_concepts.name` is the normalized identity (lower case). A new
   column `display_name` stores the spelling of the latest report and is shown; rows written before
   it show the normalized name. Identity and the 30-concept cap are unchanged.
7. **Traceability.** Each concept lists its proofs (4.4) on expand, newest first, as the Session
   progress dialog does.
8. **Empty sections say so** ("No assessment yet", "No difficulty identified") and never invent a
   result.

## Domain

`internal/domain/progress`: `TopicDetail`, `BuildTopicDetail(in TopicDetailInput)`. The pass outcome of
each attempt reaches the domain already computed (5.1 decision 9), so the function needs no
`passScore`. A quiz row carries its number and difficulty (the screen composes "Quiz N · difficulty"
in the profile's language); a challenge row also carries its difficulty and time limit, which the exam
rules dialog shows.
`internal/domain/study`: `ConceptState.DisplayName`; `ApplyReport` keeps the latest non-blank
spelling.

## SQLite

`ALTER TABLE session_concepts ADD COLUMN display_name TEXT NOT NULL DEFAULT ''` (idempotent, like
the existing column migrations); the upsert writes it. The read adapter gains the detail queries
(per-attempt defense score, outcome, criterion levels with titles from the frozen rubric version).

## Interface

Wails `GetProgressTopic(sessionID)`. Frontend: the right panel of `ProgressScreen` with the topic
header (name, folder), activity counters, Results, Learning identified (understood, difficulties,
explained only).

## Acceptance Criteria

- A challenge whose latest attempt has a criterion at level 1 in the defense lists that criterion
  as a difficulty from "Arch Lab: <title>".
- An `INTRODUCED` concept appears only under "explained, not yet assessed".
- A concept reported as "Cache-aside" shows "Cache-aside", not "cache-aside".
- A topic with no quiz shows "No assessment yet" in the quiz results.
- Same quality gate as 5.1.

## Out of scope

Insights (5.4), the chart (5.3), linking a proof to its chat message.
