# GEMINI.md — Foundational Mandates for Athena School

This document defines the absolute rules and standards for development in the `athena-school` project. These instructions take precedence over general defaults.

## 1. Mandatory XP Development Cycle

For every specification (spec), you MUST follow this sequence strictly:

1.  **Red (TDD):** Write a failing test first. No production code before a failing test.
2.  **Green:** Implement the absolute minimum code to pass the test. No over-engineering.
3.  **Refactor:** Improve code quality without changing behavior. Ensure tests still pass.
4.  **Commit:** Only after passing the Quality Gate (tests + coverage ≥ 80% + lint + security + vulnerability checks).

## 2. Core XP Rules

*   **One spec at a time:** Never work on multiple behaviors simultaneously.
*   **Smallest Implementation:** If 5 lines pass the test, do not write 6.
*   **No Big Design Upfront:** Do not create abstractions for hypothetical future needs.
*   **Test as Documentation:** Test function names MUST follow the format `Test_Given<Context>_When<Action>_Then<Outcome>` — e.g. `Test_GivenNoArgs_WhenHelpFlag_ThenPrintsToolName`.

## 3. Go Coding Standards

*   **Language:** All code (variables, functions, types, comments, errors, filenames) MUST be in **English**.
*   **Package Structure:** Tests must be in the same package as the code, using the `_test.go` suffix.
*   **Error Handling:** Always handle errors explicitly. Never use `_` for error return values.
*   **Tools:**
    *   Use `testify` (`assert`/`require`) for assertions.
    *   Use `Mockery` for interface mocks (prefer exact values/typed matchers over `mock.Anything`).
*   **Quality Gate:** Minimum coverage threshold is **80%**. The pre-commit hook will fail if below this.
*   **Test Pattern:** Every test MUST follow the **Given/When/Then** structure in both name and body:
    *   **Name:** `Test_Given<Context>_When<Action>_Then<Outcome>`
    *   `// Given:` — arrange initial state and dependencies
    *   `// When:` — execute the action under test
    *   `// Then:` — assert the expected outcome

## 4. Security & Integrity (Non-Negotiable)

*   **No Command Injection:** Never concatenate user input into shell strings. Use `exec.Command` with separate arguments.
*   **No Shell Wrapping:** Call binaries directly; never use `sh -c` for subprocesses.
*   **Path Traversal:** Validate that all operations remain within expected directories.
*   **File Permissions:** Config files must use mode `0600`.
*   **No Suppression:** NEVER use `nolint`, `nosec`, or similar bypasses. Fix the root cause.
*   **Secrets:** Never hardcode or commit API keys or credentials.

## 5. User Experience (UX)

*   **Visual Feedback:** Use `github.com/briandowns/spinner` for all external operations (AI, Git, Tests, Lint).
*   **Guidance:** Every command must conclude with a "next-step hint" to guide the user.

## 6. Documentation & Versioning

*   **Commits:** Must follow **Conventional Commits** in English (e.g., `feat(setup): bootstrap athena CLI`).
*   **Versioning:** Follow **SemVer** (MAJOR.MINOR.PATCH).
*   **Changelog:** Always update the `[Unreleased]` section in `CHANGELOG.md` following the "Keep a Changelog" standard.
*   **Architecture:** Keep `docs/` and `README.md` synchronized with any structural changes.

## 7. Prohibited Actions

*   Implementing code before a failing test.
*   Creating helpers/abstractions not required by the current spec.
*   Anticipating future requirements.
*   Committing with failing quality gates or skipping the refactor phase.