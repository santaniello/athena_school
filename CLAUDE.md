# CLAUDE.md — Athena School Development Rules

## Mandatory cycle per spec

For each spec, the cycle MUST be followed in this exact order:

1. **Write the test first (TDD — Red phase)**
   - The test must fail before any implementation
   - Never write production code before the test

2. **Implement the minimum necessary (Green phase)**
   - Write only the code needed to make the test pass
   - No over-engineering, no anticipating future requirements

3. **Refactor (Refactor phase)**
   - Improve quality without changing behavior
   - Tests must continue to pass after any refactoring

4. **Commit**
   - Only after: tests passing + coverage ≥ threshold + lint ok + security ok + vulnerability ok

## Mandatory XP rules

- **One spec at a time** — never work on multiple behaviors simultaneously
- **Do not skip steps** — do not implement before having a test; do not refactor before going green
- **Smallest possible implementation** — if the test passes with 5 lines, do not write 20
- **No big design upfront** — do not create abstractions for hypothetical future needs
- **Test is documentation** — the test name must describe the expected behavior

## Language

- All code must be written in **English**: variable names, functions, types, constants, comments, internal error messages, and file names
- Exception: user-facing terminal messages may follow the project's language

## Go code rules

- Small functions with single responsibility
- Errors always handled explicitly (never `_` for errors)
- Descriptive English names
- Tests in the same package with `_test.go` suffix
- Test coverage: every behavior documented in a spec must have a test
- Minimum coverage threshold: **80%** — the pre-commit hook will refuse to proceed if overall coverage is below this value
- Tests must follow the **GivenWhenThen** pattern both in name and body:
  - **Function name:** `Test_Given<Context>_When<Action>_Then<Outcome>` — e.g. `Test_GivenNoArgs_WhenHelpFlag_ThenPrintsToolName`
  - **Body:** three labeled sections — `// Given:` (arrange), `// When:` (act), `// Then:` (assert)
- Use **testify** for assertions (`assert` / `require`)
- Use **Mockery** to generate mocks from interfaces
- Mock parameters must be typed explicitly — avoid `mock.AnythingOfType` and `mock.Anything` unless there is no viable alternative; prefer exact values or typed matchers

## Security rules

- **No command injection** — never concatenate user input directly into shell strings; always pass arguments as separate values to `exec.Command`
- **No shell wrapping** — never invoke subprocesses via `exec.Command("sh", "-c", userInput)`; call binaries directly with explicit args
- **Path traversal prevention** — validate that all generated paths stay within expected directories
- **Config command validation** — commands read from config must be split into binary + args before execution; never passed as a raw shell string
- **File permissions** — all files created inside config directories must use permission `0600` (owner read/write only)
- **No hardcoded secrets** — no API keys, tokens, or credentials anywhere in the code
- **No suppression of findings** — never use `#nosec`, `//nolint:gosec`, `//nolint:<linter>`, or any other mechanism to silence security, vulnerability, or code smell warnings; fix the root cause instead of suppressing the diagnostic

## Versioning

- The project follows **Semantic Versioning** as defined at [semver.org](https://semver.org/)
- Format: `MAJOR.MINOR.PATCH`
  - `MAJOR` — incompatible API or CLI interface changes
  - `MINOR` — new commands or features added in a backward-compatible manner
  - `PATCH` — backward-compatible bug fixes
- Version is bumped manually when releasing — never automatically on every commit

## Commits

- All commit messages must follow the **Conventional Commits** standard and be written in **English**
- Format: `<type>(<scope>): <description>` — e.g. `feat(setup): bootstrap athena CLI`
- Common types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `ci`
- Description must be lowercase, imperative mood, and without a trailing period

## User experience

- Every operation that invokes the AI or runs an external command must display an animated spinner while waiting, using `github.com/briandowns/spinner`
- Spinner must start immediately before the operation and stop as soon as it completes or fails
- Spinner suffix must describe what is happening — e.g. `" Running tests..."`, `" Calling AI..."`
- Every command must display a next-step hint at the end of its output, guiding the user to the next command in the workflow

## Documentation

- After every change, evaluate whether `README.md` needs to be updated — update it if the change affects usage, commands, configuration, or project structure
- Architecture documentation lives in `docs/` at the project root — every structural decision must be reflected there
- `CHANGELOG.md` follows the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) standard — every change must add an entry to the `[Unreleased]` section

## What the AI must NOT do

- Implement code before there is a test for it
- Create helpers or abstractions not required by the current spec
- Anticipate future specs in the current code
- Commit without the quality gate passing
- Skip the refactoring phase
