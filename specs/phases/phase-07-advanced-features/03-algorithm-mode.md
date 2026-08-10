# Phase 7.3 — Algorithm Mode

> Visible only for users with IT/Software area in `UserProfile`.

## Goal

User solves algorithm problems in an integrated code editor; code is executed in a sandbox and evaluated for correctness and quality.

## Tasks

- [ ] `internal/domain/algorithm/` — problem model, test case model
- [ ] LLM generates problems with hidden test cases
- [ ] Code editor in the UI (Monaco or CodeMirror)
- [ ] Sandboxed execution:
  - Process or container with restricted syscalls
  - CPU time limit: 5s
  - Memory limit: 256 MB
  - No network access
- [ ] Test runner: executes code against hidden test cases, reports pass/fail per case
- [ ] Evaluation dimensions: correctness, time complexity, space complexity, code quality, edge case handling
- [ ] Result: test case summary + LLM code review

## Acceptance Criteria

- User submits a solution; all test cases run and results are shown (pass / fail / timeout)
- An infinite loop is terminated within 5 seconds with a "time limit exceeded" result
- Correct solution for all test cases produces a passing evaluation
- Code that attempts file or network I/O is blocked by the sandbox
- Mode is not visible in the UI for users with a non-IT area in their profile
