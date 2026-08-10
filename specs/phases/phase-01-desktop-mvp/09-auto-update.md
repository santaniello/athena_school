# Phase 1.9 — Auto-Update

## Goal

The app silently checks for new versions at startup and notifies the user without blocking the UI.

## Tasks

- [ ] On startup: fetch latest release from GitHub Releases API
- [ ] Compare fetched version tag with current build version (injected at build time via `-ldflags`)
- [ ] If newer version available: show silent notification "New version available — click to update"
- [ ] On user confirmation: download installer, launch it, and exit the current app
- [ ] No blocking; if the check fails (offline, timeout), app continues normally

## Acceptance Criteria

- App starts and the update check completes within 3 seconds without blocking the UI
- If the current version is the latest, no notification appears
- If a newer version exists, the notification is shown; clicking it starts the download
- A failed update check (network error) logs the error but does not affect app startup
