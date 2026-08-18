---
description: Run the Athena quality gate and review the branch before pushing a PR
allowed-tools: Bash(git status:*), Bash(git branch:*), Bash(git diff:*), Bash(git log:*), Bash(grep:*), Bash(make test:*), Bash(make lint:*), Bash(make mutation-go:*), Bash(make mutation-frontend:*), Bash(npm --prefix frontend run:*), Bash(gh pr view:*), Bash(gh pr list:*), Skill
---

Run the local pre-push review for this branch. Follow these steps in order and stop
at the first failure — do not continue to the semantic review if the mechanical gate
is red, since CI would reject the PR anyway.

## 1. Preconditions

- `git branch --show-current` — abort if the branch is `main` or `develop`.
- `git status --porcelain` — if the working tree is dirty, list the uncommitted files
  and ask whether to review them anyway or stop so they can be committed first.
- Determine the base branch. If a PR is already open, take it from
  `gh pr view --json baseRefName`; otherwise ask whether this branch targets `develop`
  or `main`. Call it `<base>` below — do not assume `main`, since most work here is
  merged into `develop` and diffing against the wrong base reviews commits that are
  already merged.
- `git diff --name-only origin/<base>...HEAD` — this is the change set under review.
  Abort if it is empty.

## 2. Mechanical gate (the same checks CI runs)

Run only what the change set actually touches:

- Always: `make test` and `make lint`.
- If any `frontend/` file changed: `npm --prefix frontend run lint`,
  `npm --prefix frontend run format:check`, `npm --prefix frontend run test:coverage`.
- If any non-test `.go` file under `internal/domain/` or `internal/application/`
  changed: `make mutation-go`.
- If any production file under `frontend/src/` changed (excluding `*.test.ts`,
  `*.test.tsx`, `components/ui/`, `lib/utils.ts`, `main.tsx` and `vite-env.d.ts`,
  which CI also excludes): `make mutation-frontend`.
- For either mutation run, a surviving mutant means an existing assertion is too weak —
  report which one, and say that the fix is to strengthen that assertion, not to add a
  redundant test.
- Grep the changed Go files for `//nolint:*gosec` and `#nosec`. Any hit is a blocker:
  AGENTS.md forbids suppressing security findings.

Report each check as pass/fail with the real output. Do not summarize a failure as a
pass.

## 3. Semantic review

Invoke the `code-review` skill at level `high` against the branch diff. It carries the
AGENTS.md rules automatically via CLAUDE.md, so do not restate them in the prompt.

Pay particular attention to the rules that no linter can catch:

- Business logic that leaked out of `internal/domain/` into `internal/infrastructure/`
  or `internal/interfaces/desktop/`
- Imports in `internal/domain/` pointing outward (infrastructure or interfaces)
- Tests that do not follow GivenWhenThen, or that use `mock.Anything` /
  `mock.AnythingOfType` where an exact value was available
- Mocks defined in-package instead of in the sibling `mocks` subpackage
- Missing `[Unreleased]` entry in `CHANGELOG.md`
- Commit subjects that break Conventional Commits (`git log --format=%s origin/<base>..HEAD`)

## 4. Report

Print the findings in the terminal, most severe first.

Then check whether a PR already exists for this branch (`gh pr view --json number,url`).
If one does, ask whether to post the findings as inline PR comments — only re-run the
skill with `--comment` if the answer is yes. If no PR exists yet, just report locally.

Do not promise that CodeRabbit will review the PR automatically. That only holds if the
GitHub App is installed for the repository and the PR targets a branch covered by
`auto_review.base_branches` in `.coderabbit.yaml` — and even then, the open-source plan
is rate limited by star count, so the review may not fire. Say instead that
`@coderabbitai review` triggers one manually if none appears.
