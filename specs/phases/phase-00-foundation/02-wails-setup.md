# Phase 0.2 — Wails Setup

## Goal

Desktop shell running. An empty Wails + React + TypeScript window opens without errors.

## Tasks

- [ ] `wails init` with React + TypeScript template
- [ ] `wails build` produces a working binary for the host OS
- [ ] `wails dev` launches the dev window with hot reload

## Acceptance Criteria

- `wails dev` opens a blank desktop window with no console errors
- `wails build` exits 0 and produces a binary in `build/bin/`
- Frontend lives under `frontend/` and is ignored by `go build`