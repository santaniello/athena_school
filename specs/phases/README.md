# Athena — Implementation Phases

Each phase delivers working software. Do not start the next phase before the current one is complete and tested.

> **Reference:** `specs/Planning.md` is the authoritative source. This directory contains the detailed specs per phase.

---

## Stack

```text
Frontend         React + TypeScript
Desktop bridge   Wails v2
Core             Go (Clean/Hexagonal)
Local DB         SQLite (modernc.org/sqlite — pure Go, no CGO)
Vector store     Local (initial phase)
LLM              OpenRouter API
CI/CD            GitHub Actions
Payments         Paddle
```

---

## Principles

- **Incremental** — each phase ships usable software
- **Core-first** — business rules never live in the frontend
- **Knowledge-first** — query local knowledge before calling the LLM
- **Local-first** — user data stays on device; server manages only auth and licenses
- **Simple design** — no premature abstractions; solve the current problem
- **TDD** — tests before implementation; minimum 80% coverage

---

## Overview

| Phase | Name | Goal | Depends on |
|---|---|---|---|
| [0](phase-00-foundation/) | Foundation | Repo, tooling, CI/CD | — |
| [1](phase-01-desktop-mvp/) | Desktop MVP | OpenRouter key gate, onboarding, study | Phase 0 |
| [2](phase-02-knowledge-engine/) | Knowledge Engine | Knowledge Base + RAG + notes | Phase 1 |
| [3](phase-03-architecture-lab/) | Architecture Lab | Challenges, diagram whiteboard, rubric evaluation, oral defense | Phase 2 |

---

## Phase 0 — Foundation

**Done when:** `wails dev` opens a blank desktop window with no errors; `go test ./...` passes; pushing tag `v0.0.1` produces Windows and Linux binaries in GitHub Releases.

| Spec | Description |
|---|---|
| [01-repo-setup.md](phase-00-foundation/01-repo-setup.md) | gitignore, CLAUDE.md, go.mod, directory structure |
| [02-wails-setup.md](phase-00-foundation/02-wails-setup.md) | Wails init, build, dev mode |
| [03-quality-gates.md](phase-00-foundation/03-quality-gates.md) | Pre-commit hooks, lint, vuln check |
| [04-cicd.md](phase-00-foundation/04-cicd.md) | GitHub Actions: CI + release matrix |

---

## Phase 1 — Desktop MVP

**Done when:** User installs on Windows or Linux, connects an OpenRouter key, completes conversational onboarding, opens the main screen, and runs a full study session with streaming personalized response.

| Spec | Description |
|---|---|
| ~~[01-auth-backend.md](phase-01-desktop-mvp/01-auth-backend.md)~~ | Removed — see [12-remove-local-login.md](phase-01-desktop-mvp/12-remove-local-login.md) |
| ~~[02-auth-ui.md](phase-01-desktop-mvp/02-auth-ui.md)~~ | Removed — see [12-remove-local-login.md](phase-01-desktop-mvp/12-remove-local-login.md) |
| [03-home-screen.md](phase-01-desktop-mvp/03-home-screen.md) | Navigation shell (sidebar + topbar) + Home screen |
| [04-onboarding.md](phase-01-desktop-mvp/04-onboarding.md) | OpenRouter key gate + conversational onboarding → UserProfile |
| [05-llm-service.md](phase-01-desktop-mvp/05-llm-service.md) | OpenRouter: LLMProvider, streaming, model router, budget |
| [06-study-mode.md](phase-01-desktop-mvp/06-study-mode.md) | Study session with personalization and streaming |
| [07-sqlite.md](phase-01-desktop-mvp/07-sqlite.md) | Local SQLite schema: sessions, messages, usage |
| [08-settings.md](phase-01-desktop-mvp/08-settings.md) | Settings screen: OpenRouter key, profile fields |
| [09-auto-update.md](phase-01-desktop-mvp/09-auto-update.md) | GitHub Releases check + silent update notification |
| [10-study-folders.md](phase-01-desktop-mvp/10-study-folders.md) | Folder tree for study sessions in the sidebar |
| [11-documentation.md](phase-01-desktop-mvp/11-documentation.md) | In-app manual: purpose, study sessions, Knowledge Engine |
| [12-remove-local-login.md](phase-01-desktop-mvp/12-remove-local-login.md) | Removes local account/login — single-user install, no data was ever account-owned |
| [13-reset-local-data.md](phase-01-desktop-mvp/13-reset-local-data.md) | Settings action to permanently clear study sessions, folders, and knowledge, live, without closing the app |

---

## Phase 2 — Knowledge Engine

**Done when:** User imports a Markdown folder, runs a study session that uses the notes as context, extracts and approves evidence-backed Knowledge Items, reconciles duplicates against existing knowledge, sees revision history in the Knowledge Explorer, resumes answers with their original sources, and finds approved items retrieved as context in later sessions.

Delivered in three increments, each demonstrable on its own: **2.1 → 2.2 → 2.3 → 2.7** (model, extraction, notes import + explorer, review queue), then **2.4 → 2.5 → 2.6 → 2.8** (vector search, RAG, study context limits, item indexing), then **2.9 → 2.10 → 2.11 → 2.12** (provenance, duplicate detection, reconciliation, revision history).

| Spec | Description |
|---|---|
| [01-knowledge-item.md](phase-02-knowledge-engine/01-knowledge-item.md) | KnowledgeItem domain model, lifecycle rule + SQLite schema |
| [02-knowledge-extraction.md](phase-02-knowledge-engine/02-knowledge-extraction.md) | On-demand LLM extraction → unpersisted draft candidates |
| [03-notes-import-and-knowledge-explorer.md](phase-02-knowledge-engine/03-notes-import-and-knowledge-explorer.md) | Markdown ingest pipeline (parse → chunk → embed → shadow Item) + sidebar tree UI, detail screen and actions |
| [04-vector-search.md](phase-02-knowledge-engine/04-vector-search.md) | Pure-Go cosine similarity vector store (ADR-004) |
| [05-rag-integration.md](phase-02-knowledge-engine/05-rag-integration.md) | Knowledge-first retrieval flow + source modes |
| [06-study-context-limits.md](phase-02-knowledge-engine/06-study-context-limits.md) | Dynamic model context measurement, warning, and hard session limit |
| [07-knowledge-review.md](phase-02-knowledge-engine/07-knowledge-review.md) | Draft review queue + pending badge |
| [08-knowledge-item-indexing.md](phase-02-knowledge-engine/08-knowledge-item-indexing.md) | All item statuses indexed; approved-only RAG + backfill |
| [09-persistent-provenance.md](phase-02-knowledge-engine/09-persistent-provenance.md) | Evidence-backed items + persisted sources for historical answers |
| [10-duplicate-detection.md](phase-02-knowledge-engine/10-duplicate-detection.md) | Exact normalized and semantic duplicate detection |
| [11-knowledge-reconciliation.md](phase-02-knowledge-engine/11-knowledge-reconciliation.md) | Human-approved create/update/relate/conflict/no-change proposals |
| [12-knowledge-revision-history.md](phase-02-knowledge-engine/12-knowledge-revision-history.md) | Immutable evidence-backed item revision history |
| [14-study-sources-panel.md](phase-02-knowledge-engine/14-study-sources-panel.md) | NotebookLM-style Sources panel shell on the Study screen |
| [15-session-scoped-knowledge.md](phase-02-knowledge-engine/15-session-scoped-knowledge.md) | Every knowledge item, chunk and import owned by a study session; retrieval and deletes follow it |
| [16-remove-conversation-extraction.md](phase-02-knowledge-engine/16-remove-conversation-extraction.md) | Documents-only knowledge: removes conversation extraction, review, the item lifecycle and the Knowledge section (supersedes 2.2, 2.7, 2.9–2.13) |
| [17-session-sources-panel.md](phase-02-knowledge-engine/17-session-sources-panel.md) | Functional NotebookLM-style Sources panel: list, add and remove a session's documents |

---

## Phase 3 — Architecture Lab

**Done when:** User picks or generates a system-design challenge, solves it on a diagram whiteboard, submits it for a rubric evaluation, defends the decisions in a bounded Q&A with the tutor, and sees the diagram score, the understanding score and the attempt history. Delivered as **M1 (specs 01–13): standalone lab**, then **M2 (specs 14–17): tutor and study-session integration**.

Design reference: [ADR-005](../decisions/ADR-005-exercise-engine.md). The lab screen layout is described in [3.8](phase-03-architecture-lab/08-lab-layout-sketch.md); colors and components follow the existing frontend.

| Spec | Description |
|---|---|
| [01-challenge-domain-and-persistence.md](phase-03-architecture-lab/01-challenge-domain-and-persistence.md) | Challenge model, SQLite schema, filterable repository |
| [02-rubrics.md](phase-03-architecture-lab/02-rubrics.md) | Built-in base rubric + immutable per-challenge versions |
| [03-attempts-and-session-link.md](phase-03-architecture-lab/03-attempts-and-session-link.md) | Exam-style attempts (time limit, discarded if abandoned), submissions, `SessionExercise` |
| [04-exercise-service.md](phase-03-architecture-lab/04-exercise-service.md) | Start, submit, discard attempts; one exam at a time; startup cleanup |
| [05-evaluation-engine.md](phase-03-architecture-lab/05-evaluation-engine.md) | Per-criterion levels, deterministic score, retry on failure |
| [06-completion-rules.md](phase-03-architecture-lab/06-completion-rules.md) | Pass policy (both scores), frozen minimum score, derived progress, Settings field |
| [06-01-challenge-generation.md](phase-03-architecture-lab/06-01-challenge-generation.md) | Minimal AI generation from a studied session; challenge and rubric saved together |
| [07-challenge-catalog-ui.md](phase-03-architecture-lab/07-challenge-catalog-ui.md) | Architecture Lab entry screen: catalog, filters, generate from a studied session |
| [08-lab-layout-sketch.md](phase-03-architecture-lab/08-lab-layout-sketch.md) | Three-panel exam layout from fixtures in a dev-only preview; countdown, no pause, no tabs |
| [09-diagram-editor-spike.md](phase-03-architecture-lab/09-diagram-editor-spike.md) | Throwaway prototype of the draw.io online embed in Wails; produces ADR-006 |
| [10-diagram-workspace.md](phase-03-architecture-lab/10-diagram-workspace.md) | The exam end to end: rules dialog, embedded editor, countdown, submit, auto-submit on expiry, leave |
| [10-01-tutor-hints.md](phase-03-architecture-lab/10-01-tutor-hints.md) | On-demand hints during the exam, 3 per attempt, direction or question only |
| [11-architecture-evaluator.md](phase-03-architecture-lab/11-architecture-evaluator.md) | Diagram summary from the XML, LLM evaluation per criterion, starts after submit, retry |
| [12-architecture-defense.md](phase-03-architecture-lab/12-architecture-defense.md) | The tutor's 5 adaptive questions, understanding score, finishing the exam |
| [13-evaluation-and-history-screen.md](phase-03-architecture-lab/13-evaluation-and-history-screen.md) | Result view (also the exam ending) and per-challenge History of finished attempts |
| [14-ai-challenge-generation.md](phase-03-architecture-lab/14-ai-challenge-generation.md) | Exercise offer in the study chat: Accept or Decline, reuse before generating, send blocked while pending |
| [15-open-lab-from-chat.md](phase-03-architecture-lab/15-open-lab-from-chat.md) | Open in Architecture Lab from the chat card, same window, return to the study session |
| [16-session-feedback.md](phase-03-architecture-lab/16-session-feedback.md) | Short derived result summary in the session's next tutor replies, no XML or transcript |
| [17-session-exercises-ui.md](phase-03-architecture-lab/17-session-exercises-ui.md) | Persistent exercise card in the chat with progress, Start again and History |

> Spec files are added one at a time, after review. Algorithms Lab (code editor, runner, tests) is a future phase and is not specified yet.
> Manual challenge creation (the `MANUAL` origin of a challenge) is also future work: the model keeps the field, but no screen writes it yet.

---

## Dependency Graph

```text
Phase 0 (Foundation)
    ↓
Phase 1 (Desktop MVP)
    ↓
Phase 2 (Knowledge Engine)
    ↓
Phase 3 (Architecture Lab)
```
