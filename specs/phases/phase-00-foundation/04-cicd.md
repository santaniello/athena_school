# Phase 0.4 — CI/CD (GitHub Actions)

## Goal

Every PR runs tests, coverage, and lint automatically in a neutral environment.
Every version tag triggers a cross-platform release build with distributable artifacts.

## Tasks

### `ci.yml`

Triggered on: push to any branch, pull request to `main`.

Steps:
- [x] Checkout + Go setup (`actions/setup-go`)
- [x] `go test -race -coverprofile=coverage.out ./...`
- [x] Coverage threshold check — fail if below 80%
- [x] Suppression check — `grep -rn --include="*.go" -E '//\s*(nolint:.*gosec|nosec)' .` must return no matches
- [x] `golangci-lint run` (via `golangci-lint-action`)
- [x] `govulncheck ./...`

### `release.yml`

Triggered on: push of tag matching `v*`.

- [x] Build matrix: `[windows-latest, ubuntu-latest]`
- [x] Each runner: `wails build` → produces binary
- [x] Artifacts uploaded to GitHub Releases automatically via `softprops/action-gh-release`

> **Note:** macOS builds require an Apple Developer certificate for code signing.
> Add `macos-latest` to the matrix only when the signing workflow is in place (Phase 0.5 or later).

## Acceptance Criteria

- PR opened → CI runs and reports pass/fail within 5 minutes
- PR with coverage below 80% → CI fails with explicit message
- PR with `//nolint:gosec` or `//nosec` anywhere in `.go` files → CI fails
- `git tag v0.0.1 && git push --tags` → GitHub Release created with Windows `.exe` and Linux binary attached
