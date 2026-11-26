# Phase 5.5 — Folder Insights

## Goal

The same **Athena Insights** at the Folder level: a consolidated analysis across the folder's
topics (which are solid, which need reinforcement, what links them) and what to study next.

## Decisions

1. **Same mechanism as 5.4:** on demand, stored, stale by fingerprint, failure never blocks the
   screen, assistant language.
2. **Input = the topic summaries of the folder** (5.1: name, scores, status) plus each topic's
   difficulties (5.2), not the raw attempts, so the prompt is bounded per topic (it still grows with the number of topics; there is no cap).
3. **Citations point to topics** (session ids), so a recommendation opens the topic it is about.
4. **Fingerprint** covers the folder's set of sessions and each topic's own fingerprint: moving a
   session in or out of the folder, deleting it or new activity in any topic makes it stale. It also covers
   everything the prompt shows (folder and topic names, status, scores, difficulties), so renaming or a new
   pass score that moves a status does too.
5. **Shown above the topic list** of the selected folder, collapsed to the first sentence by default.
6. **Needs at least one topic outside Insufficient evidence**; otherwise no button.

## Implementation

- Domain (`internal/domain/progress`): `FolderInsightInput` (the folder's topics as `FolderInsightTopic`: summary,
  difficulties and the topic's own `Fingerprint`), `FolderFingerprint` (hash of the language, the folder name and,
  for each topic in any order, its session id, name, status, scores, difficulties and own fingerprint), `ParseFolderInsight` (same validation as `ParseInsight`; the
  refs the model cites are `T1`, `T2`..., stored as `Citation{Kind: TOPIC, ID: session id, Label: topic name}`),
  `FolderInsightRepository` and `FolderInsightGenerator`. The stored value is the same `Insight`, and the card
  state the same `InsightView`.
- Application: `Service.GetFolderInsight(folderID)` (stored insight and stale flag, no LLM; an unreadable
  stored insight or language is logged and degrades to a card that can generate, like the topic's) and
  `Service.GenerateFolderInsight(folderID)`, wired with `WithFolderInsights` on top of `WithInsights` (language and
  clock). `Reader.ReadFolderTopics` reads the folder with the detail of each topic. The call uses
  `TaskProgressInsight` and belongs to no session.
- Bindings: `GetProgressFolderInsight` and `GenerateProgressFolderInsight`, both returning the topic card's
  state. The dashboard is untouched: the card reads its own state when a folder is selected.
- Frontend: `ProgressFolderInsightCard`; a citation is a button that selects the topic when it is still in the
  folder, plain text otherwise.

## SQLite

`folder_insights`, same columns as `topic_insights` with `folder_id` as the key referencing
`folders(id)` with `ON DELETE CASCADE`; deleted on reset.

## Acceptance Criteria

- Moving a session into the folder marks its insight stale.
- A recommendation names and opens the topic it cites.
- Same quality gate as 5.1.
