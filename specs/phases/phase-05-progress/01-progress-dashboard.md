# Phase 5.1 — Progress Dashboard

## Goal

Add a **Progress** tab that shows, per Folder, every study session as a **topic** with its latest
quiz and Arch Lab results and a qualitative status (Improving, Stable, Needs reinforcement,
Insufficient evidence). Everything is derived on read from data that already exists: no LLM call,
no new table. Visual reference: the approved [mockup](mockup/).

## Decisions

1. **A topic is a study session.** Each topic is bound to exactly one session; the user keeps one
   session per subject (one "Cache" session in the "System Design" folder, never "Cache 1" and
   "Cache 2"). There is no Topic entity, no classification by the LLM and no activity belonging to
   more than one topic. A uniqueness check for session names inside a folder may come later; it is
   not part of this phase. The topic's name is the session's `Topic`.
2. **Activities belong to a topic through their session:**
   - **Quizzes:** every quiz whose `session_id` is the session, whatever its origin (chat offer or
     menu). This is wider than `quiz.Repository.ListBySession`, which returns only offer quizzes.
   - **Arch Labs:** every challenge whose `source_session_id` is the session, plus every challenge
     linked to it through `session_exercises`, counted once (by challenge id). All attempts of such a
     challenge count, standalone attempts included.
   - **Concepts:** the session's `session_concepts` (4.2) and `concept_evidence` (4.4).
3. **Derived on read, never stored.** Moving a session to another folder moves its whole history with
   it, and deleting a session removes its topic from Progress (its quizzes and labs stay in their own
   screens). A count can never be doubled, because nothing is copied.
4. **Quiz score of a topic** = the mean, over the session's quizzes that have at least one attempt,
   of each quiz's **latest** attempt (`CorrectCount / Total`, in percent). Each quiz weighs the same
   whatever its question count. Rounded half up to an integer. No attempt at all → no score ("—").
5. **Arch Lab score of a topic.** An attempt's **lab score** is the mean of its diagram and defense
   (`UNDERSTANDING`) scores, rounded half up; only attempts with **both** evaluations count, so one
   stopped before the defense is left out. The topic's score is the mean, over its challenges with
   such an attempt, of each challenge's latest one. This mean is a Progress summary only: the Arch
   Lab's pass rule (both scores ≥ `passScore`, never averaged) is unchanged and still feeds the
   status (decision 6). The screen labels both scores "latest attempt", since the lab catalog shows
   the best diagram score.
6. **Status rule**, checked in this order, first match wins (`passScore` is the existing minimum
   passing score setting of the Arch Lab, default 80, used for quizzes too):
   1. **Insufficient evidence** — no quiz attempt, no attempt with both lab evaluations and no
      concept with `evidence_count > 0`.
   2. **Needs reinforcement** — any of: the topic's quiz score is below `passScore`; the latest
      evaluated attempt of any challenge has the outcome `FAILED` (`exercise.EvaluateOutcome`:
      diagram, defense or mandatory criterion); any concept is `NEEDS_REVIEW`. A `PENDING` outcome
      (no defense yet) never counts as failed.
   3. **Improving** — some quiz or challenge has at least two scored attempts and the latest is at
      least **10 points** above the one before it (quiz percent, or lab score). Only attempts of
      the same item are compared.
   4. **Stable** — anything else.

   Needs reinforcement wins over Improving: going from 40 to 70 is progress but still below the
   minimum; the progress shows in the chart (5.3).
7. **Header counters:** Folders (folders with at least one session), Topics (sessions in those
   folders), Topics to review (topics in Needs reinforcement). No study hours, no global mastery
   percentage.
8. **Lists.** Folders in the same order as the Folders screen, the first one selected on open.
   Topics of the selected folder, most recent activity first (latest of: session start, quiz attempt,
   lab attempt, concept update). A folder with no session does not appear. No period filter.
9. **The domain owns every rule.** `internal/domain/progress` holds the rules as pure functions over
   its own input structs. It imports neither `study`, `quiz` nor `exercise` (ADR-001), so the pass
   outcome reaches it already computed. The application layer reads through a port that one SQLite
   read adapter implements with dedicated queries.
10. **Language.** The Progress screen follows the profile's assistant language through its own
    i18next namespace (`progressTab`, `progress-tab-en.ts` / `progress-tab-pt.ts`; `progress` is
    already the Session progress dialog's). The menu item stays "Progress" in English, like the rest
    of the menu.

## Domain

`internal/domain/progress/`:

```go
type Status string // INSUFFICIENT_EVIDENCE, NEEDS_REINFORCEMENT, IMPROVING, STABLE

type QuizInput struct {
    QuizID   string
    Attempts []ScoredAttempt // oldest first
}

type ChallengeInput struct {
    ChallengeID string
    Attempts    []LabAttempt // only attempts with both evaluations, oldest first
}

type ScoredAttempt struct { Percent int; At time.Time }

type LabAttempt struct {
    DiagramScore       int
    UnderstandingScore int // lab score = mean of both, half up (decision 5)
    Failed       bool // outcome FAILED, computed by the application with exercise.EvaluateOutcome
    At           time.Time
}

type TopicInput struct {
    SessionID, Name, FolderID string
    StartedAt                 time.Time
    Quizzes                   []QuizInput
    Challenges                []ChallengeInput
    Concepts                  []ConceptInput // status, evidence count, updated at
}

type TopicSummary struct {
    SessionID, Name    string
    QuizScore, LabScore *int // nil = no score
    Status             Status
    LastActivity       time.Time
}

func SummarizeTopic(in TopicInput, passScore int) TopicSummary
func BuildDashboard(folders []FolderInput, passScore int) Dashboard // counters + folders + topics
```

## Application

`internal/application/progress`: `Service.GetDashboard(ctx)` reads the folders and topic inputs
through `progress.Reader`, reads `passScore` from settings, and returns `BuildDashboard`.

## SQLite

A read adapter implementing `progress.Reader` with queries over `folders`, `sessions`, `quizzes`,
`quiz_attempts`, `challenges`, `session_exercises`, `challenge_attempts`, `evaluations` and
`session_concepts`. No migration.

## Interface

- Wails `GetProgressDashboard()` → counters, folders (id, name, topic count) and, per folder, its
  topics (session id, name, quiz score or null, lab score or null, status).
- Frontend: a **Progress** item in the side menu, after Folders; a `ProgressScreen` with the header
  (title, subtitle), three counters, the folder chips and the topic list, following the dark theme.
  Selecting a topic only highlights it here; its detail is 5.2.
- Empty state: no folder with sessions → a message pointing to the Study tab.

## Tasks

- [ ] Domain: quiz score (latest per quiz, mean, rounding, no attempt), lab score (mean of diagram
      and defense, latest per challenge, mean over challenges), status rule in its four branches and priority, the 10-point step on the
      same item only, counters, ordering
- [ ] SQLite: read adapter (both origins of quizzes, union of challenge links without duplicates,
      only attempts with both evaluations, outcome inputs), FK-safe after a session is moved or deleted
- [ ] Application: `GetDashboard`, `passScore` from settings
- [ ] Wails binding + frontend screen, menu item, `progressTab` namespace (en/pt)
- [ ] Docs: CHANGELOG (`Added`), README features

## Acceptance Criteria

- The tab lists only folders that have sessions; selecting one lists its sessions as topics.
- A quiz retaken 60% → 90% counts 90; two quizzes at 80% and 50% give 65%.
- A challenge whose latest attempt has diagram 85 and defense 40 shows Lab 63% and Needs
  reinforcement.
- An attempt stopped before the defense does not change the lab score.
- A session with only tutor explanations (no evidence, no attempt) shows Insufficient evidence.
- A quiz retaken 60% → 85% with no failure shows Improving.
- Moving a session to another folder moves its topic, scores and status with it.
- A quiz generated from the menu for the session counts as much as one accepted in the chat.
- A challenge both generated from the session and linked through `session_exercises` counts once.
- Opening the tab makes no LLM call.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed domain/application
  code, lint, vulnerability and `npx prettier --check .` pass.

## Out of scope

Topic detail (5.2), evolution chart (5.3), insights (5.4, 5.5), a period filter, a uniqueness check
for session names, other modalities (Coding Lab).
