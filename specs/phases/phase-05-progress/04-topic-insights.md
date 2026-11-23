# Phase 5.4 — Topic Insights

## Goal

**Athena Insights** on a topic: a short analysis and actionable recommendations written by the
LLM from the persisted evidence. They are generated only when the user asks, stored, and flagged
when new evidence makes them stale.

## Decisions

1. **On demand.** A **Generate insights** button in the topic detail. Opening Progress never calls
   the LLM (RNF01) and nothing runs in the background.
2. **Stored, one per topic.** The latest insight is shown at once on every visit. Generating again
   replaces it.
3. **Input = persisted evidence only**, never the conversation: per quiz, its attempts; per
   challenge, its attempts' scores, outcome, criterion levels and justifications, `Strengths`,
   `Gaps`, `Recommendations`; the concepts with status and their proof descriptions. Every activity
   is given an id in the prompt.
4. **Output = JSON**, validated in the domain: `analysis` (text), `recommendations` (1 to 3 items),
   and per sentence or recommendation the ids of the activities that support it. Unknown ids are
   dropped. Invalid output → the generation fails and the previous insight stays.
5. **Stale detection without the LLM.** The insight stores a fingerprint of its input (hash of the
   activity ids with their latest attempt/update timestamps, plus the assistant language). When the
   current fingerprint differs, the card says "New activity since this analysis — Update".
6. **Failure never blocks the screen** (RNF06): the card shows the error and Retry; the rest of the
   detail works.
7. **Insufficient evidence → no button.** A topic in Insufficient evidence shows "Study, take a
   quiz or a lab first" instead of generating an insight from nothing (RF11).
8. **Language:** the assistant language of the profile, like the other LLM outputs.

## Domain

`internal/domain/progress`: `Insight` (`Analysis`, `Recommendations`, `Citations`, `Model`,
`Language`, `Fingerprint`, `CreatedAt`), `Fingerprint(in InsightInput, language string)`,
`ParseInsight` (validation), `InsightRepository`, `InsightGenerator` port.

## Application

`Service.GenerateTopicInsight(ctx, sessionID)`; `GetProgressTopic` returns the stored insight and
its `stale` flag. The prompt lives with the other prompts and is tested by key phrases.

## SQLite

```sql
CREATE TABLE IF NOT EXISTS topic_insights (
  session_id  TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
  analysis    TEXT NOT NULL,
  payload     TEXT NOT NULL, -- recommendations and citations, JSON
  model       TEXT NOT NULL,
  language    TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  created_at  TEXT NOT NULL
);
```

Deleted with the session and on reset. Usage is recorded like every other LLM call.

## Acceptance Criteria

- Opening a topic with a stored insight shows it with no LLM call.
- Retaking a quiz after the insight marks it stale; generating again clears the flag.
- Changing the assistant language marks it stale.
- A malformed LLM answer keeps the previous insight and shows an error.
- A citation pointing to an unknown activity is dropped.
- Same quality gate as 5.1.

## Out of scope

Folder insights (5.5), automatic generation, scheduled study plans.
