# Phase 5.5 — Folder Insights

## Goal

The same **Athena Insights** at the Folder level: a consolidated analysis across the folder's
topics (which are solid, which need reinforcement, what links them) and what to study next.

## Decisions

1. **Same mechanism as 5.4:** on demand, stored, stale by fingerprint, failure never blocks the
   screen, assistant language.
2. **Input = the topic summaries of the folder** (5.1: name, scores, status) plus each topic's
   difficulties (5.2), not the raw attempts, so the prompt stays bounded however large the folder.
3. **Citations point to topics** (session ids), so a recommendation opens the topic it is about.
4. **Fingerprint** covers the folder's set of sessions and each topic's own fingerprint: moving a
   session in or out of the folder, deleting it or new activity in any topic makes it stale.
5. **Shown above the topic list** of the selected folder, collapsed to the first sentence by default.
6. **Needs at least one topic outside Insufficient evidence**; otherwise no button.

## SQLite

`folder_insights`, same columns as `topic_insights` with `folder_id` as the key referencing
`folders(id)` with `ON DELETE CASCADE`; deleted on reset.

## Acceptance Criteria

- Moving a session into the folder marks its insight stale.
- A recommendation names and opens the topic it cites.
- Same quality gate as 5.1.
