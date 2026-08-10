# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Repository scaffold: go.mod, Makefile, directory structure, .gitignore
- Wails v2 + React + TypeScript desktop shell (`wails dev`/`wails build`)
- CI workflow (`.github/workflows/ci.yml`): tests with coverage, 80% coverage gate, security-suppression check, `golangci-lint`, `govulncheck` on every push and PR to `main`/`develop`
- Release workflow (`.github/workflows/release.yml`): cross-platform `wails build` (Windows, Linux) and GitHub Release publishing on `v*` tags

### Changed
- Moved the Wails entrypoint from `cmd/athena/` to root `main.go`: Wails v2 does not support a main package outside the project root