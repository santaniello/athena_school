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
Auth backend     Go HTTP API (accounts + licenses)
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
| [1](phase-01-desktop-mvp/) | Desktop MVP | Login, onboarding, study | Phase 0 |
| [2](phase-02-knowledge-engine/) | Knowledge Engine | Knowledge Base + RAG + notes | Phase 1 |
| [3](phase-03-learning-intelligence/) | Learning Intelligence | Challenge + Gap Detection + Flashcards | Phase 2 |
| [4](phase-04-interview-mode/) | Interview Mode | Full interview simulation | Phase 3 |
| [5](phase-05-comercializacao/) | Comercialização | Plans, payment, macOS, feature gating | Phase 1 |
| [6](phase-06-voice-interview/) | Voice Interview | Audio interview (STT + TTS) | Phase 4 |
| [7](phase-07-advanced-features/) | Advanced Features | Whiteboard, Knowledge Graph, Algorithm Mode | Phase 4 |

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

**Done when:** User installs on Windows or Linux, creates an account, confirms email, completes conversational onboarding, opens the main screen, and runs a full study session with streaming personalized response. 7-day trial visible in the UI.

| Spec | Description |
|---|---|
| [01-auth-backend.md](phase-01-desktop-mvp/01-auth-backend.md) | HTTP auth server: register, login, refresh, plan |
| [02-auth-ui.md](phase-01-desktop-mvp/02-auth-ui.md) | Login/register/recovery screens + Wails bindings |
| [03-trial.md](phase-01-desktop-mvp/03-trial.md) | 7-day trial, badge, blocking modal |
| [04-onboarding.md](phase-01-desktop-mvp/04-onboarding.md) | Conversational onboarding → UserProfile |
| [05-llm-service.md](phase-01-desktop-mvp/05-llm-service.md) | OpenRouter: LLMProvider, streaming, model router, budget |
| [06-study-mode.md](phase-01-desktop-mvp/06-study-mode.md) | Study session with personalization and streaming |
| [07-sqlite.md](phase-01-desktop-mvp/07-sqlite.md) | Local SQLite schema: sessions, messages, usage |
| [08-settings.md](phase-01-desktop-mvp/08-settings.md) | Settings screen: OpenRouter key, profile fields |
| [09-auto-update.md](phase-01-desktop-mvp/09-auto-update.md) | GitHub Releases check + silent update notification |

---

## Phase 2 — Knowledge Engine

**Done when:** User imports a Markdown folder, runs a study session that uses the notes as context, reviews and approves Knowledge Items at session end, and sees the Knowledge Explorer organized by topic.

| Spec | Description |
|---|---|
| [01-knowledge-item.md](phase-02-knowledge-engine/01-knowledge-item.md) | KnowledgeItem domain model + SQLite schema |
| [02-knowledge-extraction.md](phase-02-knowledge-engine/02-knowledge-extraction.md) | Post-session LLM extraction → draft items |
| [03-notes-import.md](phase-02-knowledge-engine/03-notes-import.md) | Markdown ingest pipeline: parse → chunk → embed |
| [04-vector-search.md](phase-02-knowledge-engine/04-vector-search.md) | Pure-Go cosine similarity vector store |
| [05-rag-integration.md](phase-02-knowledge-engine/05-rag-integration.md) | Knowledge-first retrieval flow |
| [06-knowledge-explorer.md](phase-02-knowledge-engine/06-knowledge-explorer.md) | Sidebar tree UI + detail screen + actions |
| [07-knowledge-review.md](phase-02-knowledge-engine/07-knowledge-review.md) | Draft review queue + pending badge |

---

## Phase 3 — Learning Intelligence

**Done when:** User runs a challenge and receives structured evaluation; gap dashboard shows weak topics; flashcards are generated from approved Knowledge Items; daily review session works with SM-2.

| Spec | Description |
|---|---|
| [01-challenge-mode.md](phase-03-learning-intelligence/01-challenge-mode.md) | ChallengeSession domain + LLM problem generation |
| [02-evaluation-engine.md](phase-03-learning-intelligence/02-evaluation-engine.md) | Structured JSON evaluation + results UI |
| [03-progress-tracking.md](phase-03-learning-intelligence/03-progress-tracking.md) | Per-topic metrics aggregation + progress screen |
| [04-gap-detection.md](phase-03-learning-intelligence/04-gap-detection.md) | Gap analysis algorithm + dashboard UI |
| [05-flashcards.md](phase-03-learning-intelligence/05-flashcards.md) | Flashcard model, SM-2 scheduler, review UI |
| [06-knowledge-promotion.md](phase-03-learning-intelligence/06-knowledge-promotion.md) | Draft → approved + auto flashcard generation |

---

## Phase 4 — Interview Mode

**Done when:** User completes an interview with 3+ questions and a timer, receives a per-question evaluation report and final score, can browse interview history, and the domain matches the user profile.

| Spec | Description |
|---|---|
| [01-interview-session.md](phase-04-interview-mode/01-interview-session.md) | InterviewSession domain + progressive LLM conduct |
| [02-timer.md](phase-04-interview-mode/02-timer.md) | Configurable timer per question |
| [03-interview-evaluation.md](phase-04-interview-mode/03-interview-evaluation.md) | Per-answer + aggregate evaluation + report |
| [04-interview-history.md](phase-04-interview-mode/04-interview-history.md) | History list UI + detail + topic evolution |
| [05-domain-aware-evaluation.md](phase-04-interview-mode/05-domain-aware-evaluation.md) | Domain-specific evaluation criteria via UserProfile |

---

## Phase 5 — Comercialização

**Done when:** Trial expires and user can upgrade with real payment; locked features clearly indicate their plan; macOS build works in CI; app published on main distribution channels.

| Spec | Description |
|---|---|
| [01-plans-feature-gating.md](phase-05-comercializacao/01-plans-feature-gating.md) | Plan model + licensing service + UI lock icons |
| [02-paddle-integration.md](phase-05-comercializacao/02-paddle-integration.md) | Paddle products, webhook, checkout flow |
| [03-plans-screen.md](phase-05-comercializacao/03-plans-screen.md) | Comparison table + monthly/annual toggle |
| [04-macos-distribution.md](phase-05-comercializacao/04-macos-distribution.md) | Code signing, notarization, dmg artifact |
| [05-distribution-channels.md](phase-05-comercializacao/05-distribution-channels.md) | Windows installer + winget; Linux AppImage + deb + Flathub |

---

## Phase 6 — Voice Interview

**Done when:** User completes a full interview speaking and listening; the AI transcribes the answer, evaluates it, and asks the next question by voice; history is saved with text transcript.

| Spec | Description |
|---|---|
| [01-audio-providers.md](phase-06-voice-interview/01-audio-providers.md) | STT (Whisper) + TTS (OpenAI TTS) infrastructure |
| [02-voice-ui.md](phase-06-voice-interview/02-voice-ui.md) | Microphone controls, live transcript, speaking indicator |

---

## Phase 7 — Advanced Features

**Done when:** Knowledge Graph shows concept relations; user draws an architecture and receives a scored evaluation; Algorithm Mode runs safely isolated code (IT users only).

| Spec | Description |
|---|---|
| [01-knowledge-graph.md](phase-07-advanced-features/01-knowledge-graph.md) | Concept relations model + graph UI |
| [02-whiteboard-mode.md](phase-07-advanced-features/02-whiteboard-mode.md) | Semantic diagram model + visual editor + evaluation |
| [03-algorithm-mode.md](phase-07-advanced-features/03-algorithm-mode.md) | Code editor + sandboxed execution + evaluation |

---

## Dependency Graph

```text
Phase 0 (Foundation)
    ↓
Phase 1 (Desktop MVP)
    ↓                 ↘
Phase 2              Phase 5 (in parallel with 2–4)
    ↓
Phase 3
    ↓
Phase 4
    ↓         ↘
Phase 6      Phase 7
```