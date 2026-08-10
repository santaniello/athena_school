# ADR-001 — Hexagonal Architecture (Clean Architecture)

**Status:** Accepted  
**Date:** 2026-08-10

---

## Context

Athena is a desktop application with a rich domain: spaced repetition (SM-2), gap detection, knowledge extraction, interview evaluation, and RAG retrieval. It integrates multiple infrastructure adapters (SQLite, OpenRouter, local vector store, audio APIs) and exposes business logic through Wails bindings to a React frontend.

Two architectural styles were evaluated:

**Package Oriented Design (POD)**  
Each package is a self-contained unit of behavior, named by what it does. Used successfully in Go CLIs and libraries. The previous version of this project (CLI + Ollama) used POD.

**Clean / Hexagonal Architecture**  
Concentric layers with a strict dependency rule: outer layers depend on inner layers, never the reverse. Domain is pure Go with no infrastructure dependencies. Use cases orchestrate domain and infrastructure via interfaces (ports). Infrastructure implements those interfaces (adapters).

```
┌─────────────────────────────────────┐
│  interfaces/desktop  (Wails bindings)│
│  ┌───────────────────────────────┐  │
│  │  application  (use cases)     │  │
│  │  ┌─────────────────────────┐  │  │
│  │  │  domain  (pure rules)   │  │  │
│  │  └─────────────────────────┘  │  │
│  └───────────────────────────────┘  │
│  infrastructure  (SQLite, OpenRouter)│
└─────────────────────────────────────┘
```

---

## Decision

**Use Clean / Hexagonal Architecture.**

The domain lives in `internal/domain/`, use cases in `internal/application/`, adapters in `internal/infrastructure/`, and Wails bindings in `internal/interfaces/desktop/`. Dependencies point inward only.

---

## Rationale

**1. Domain complexity justifies the isolation.**  
SM-2 scheduling, gap detection thresholds, evaluation scoring, and knowledge promotion rules are non-trivial business logic. Isolating them in a pure-Go layer makes them easy to test without spinning up a database or calling a real LLM.

**2. Infrastructure will change.**  
The embedding model, audio STT/TTS provider, and potentially the vector store backend are all subject to replacement. Ports/adapters lets infrastructure be swapped without touching domain or use case code.

**3. TDD is a first-class requirement.**  
With POD, packages that mix business rules and infrastructure calls require more elaborate test setup. With hexagonal, domain tests are pure unit tests; infrastructure tests are integration tests — the boundary is explicit.

**4. Wails bindings are adapters, not controllers.**  
The React frontend calls Go functions via Wails. Those functions must be thin translators, not decision-makers. Hexagonal makes this constraint structural: bindings live in `interfaces/desktop/` and can only call `application/` use cases.

**5. POD was appropriate for the old CLI.**  
The previous CLI had simple, linear flows (command → LLM → output). POD matched that well. Athena has bidirectional data flows, background jobs, and cross-cutting concerns (licensing, progress tracking, gap detection) that cross package boundaries — exactly where POD becomes hard to manage.

---

## Rejected Alternative: POD

POD was rejected because:
- Business rules would spread across packages with implicit cross-dependencies
- No structural enforcement of "UI cannot contain business logic"
- Harder to mock infrastructure in tests without a clear port boundary
- Worked for the old CLI; does not scale to Athena's domain complexity

---

## Rejected Alternative: Java + Spring / Hexagonal

Java with Spring Modulith or a manual hexagonal setup was considered. Rejected because:
- **Wails** is the desktop bridge — it exists only for Go. Replacing it would require JavaFX or Swing (poor UX) or Electron (heavy, loses the native binary advantage)
- Distribution: Go produces a single binary per platform with no runtime dependency; Java requires a bundled JVM or GraalVM native compilation (complex CI setup)
- SQLite: `modernc.org/sqlite` is pure Go with no CGO; the Java equivalent requires native drivers, complicating cross-platform builds
- Java's hexagonal ecosystem is more mature, but those advantages do not outweigh the Wails / distribution tradeoffs for a desktop-first product

---

## Consequences

**Positive:**
- Domain logic is fully testable without infrastructure
- Infrastructure adapters are swappable without modifying business rules
- Wails bindings remain thin by structural convention
- Clear onboarding: new contributors know exactly where each type of code lives

**Negative:**
- More boilerplate than POD: interfaces must be defined in `domain/`, implemented in `infrastructure/`, injected at the composition root
- Dependency injection wiring lives in `cmd/athena/main.go` — this file grows as the app does and must be kept tidy