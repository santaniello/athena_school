# Phase 5.5 — Distribution Channels

## Goal

App is available through official package managers and installers on Windows and Linux.

## Windows

- [ ] `.exe` installer (Wails generates natively via NSIS)
- [ ] EV code signing certificate applied to installer
- [ ] `winget` package submitted to `microsoft/winget-pkgs`

## Linux

- [ ] `.AppImage` (primary target — works on all distros without installation)
- [ ] `.deb` package for Ubuntu / Debian
- [ ] Flathub submission (`io.github.<user>.Athena`)

## CI Changes

- [ ] Release workflow produces all three Linux artifacts: `.AppImage`, `.deb`
- [ ] Artifacts named with version and platform: `athena-v1.0.0-linux-x86_64.AppImage`

## Acceptance Criteria

- Windows: user can install via `winget install athena` after submission is approved
- Linux: downloaded `.AppImage` is executable without additional setup
- `.deb` installs cleanly on Ubuntu 22.04 LTS via `dpkg -i`
