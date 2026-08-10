# Phase 0.4 — CI/CD (GitHub Actions)

## Goal

Every PR runs tests and lint automatically. Every version tag triggers a cross-platform release build.

## Tasks

### `ci.yml`

Triggered on: push to any branch, pull request to `main`.

Steps:
- [ ] Checkout + Go setup
- [ ] `go test ./...` with race detector
- [ ] `golangci-lint run`
- [ ] `govulncheck ./...`

### `release.yml`

Triggered on: push of tag matching `v*`.

- [ ] Build matrix: `[windows-latest, ubuntu-latest]`
- [ ] Each runner: `wails build` → produces binary
- [ ] Artifacts uploaded to GitHub Releases automatically

## Acceptance Criteria

- PR opened → CI runs and reports pass/fail within 5 minutes
- `git tag v0.0.1 && git push --tags` → GitHub Release created with Windows `.exe` and Linux binary attached
