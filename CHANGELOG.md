# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Repository scaffold: go.mod, Makefile, directory structure, .gitignore
- Wails v2 + React + TypeScript desktop shell (`wails dev`/`wails build`)

### Changed
- Moved the Wails entrypoint from `cmd/athena/` to root `main.go`: Wails v2 does not support a main package outside the project root