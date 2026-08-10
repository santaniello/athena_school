# Phase 5.4 — macOS Distribution

## Goal

macOS builds are signed, notarized, and produced automatically by CI.

## Tasks

- [ ] Apple Developer account and certificates configured
- [ ] Code signing: `codesign --deep --force --verify --sign`
- [ ] Notarization: `xcrun notarytool submit` + `xcrun stapler staple`
- [ ] CI release matrix: add `macos-latest` runner to `release.yml`
- [ ] Artifact: `.dmg` file published to GitHub Releases

## Acceptance Criteria

- `wails build` on macOS produces a `.app` bundle
- `.dmg` is signed and notarized (no Gatekeeper warning on first open)
- `macos-latest` CI job passes and attaches the `.dmg` to the release
