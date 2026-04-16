# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `docker-compose.yml` to run Ollama locally (CPU mode) with a persistent volume
- `OLLAMA_HOST` environment variable support in config — overrides the default Ollama host without editing the config file
- Bootstrap CLI with `cobra` — root command with help output and subcommand listing
- `version` command printing the current semantic version (`v0.1.0`)
- Pre-commit quality gate enforcing tests, coverage (≥ 80%), lint, and security checks
- Project specifications covering the 7-phase development roadmap
- Config system: `internal/platform/config` package with `Load`, `Save`, and `DefaultPath`
- `athena config get` command — prints current provider and model
- `athena config set <key> <value>` command — persists provider, model, and ollama.host to `~/.config/athena/config.yaml`
- `athena study <topic> [subtopic]` command — interactive guided study session with LLM-driven explanation, question, and answer evaluation
- `internal/study` package with `Session`, prompt builders, and spinner-based UX
- Animated spinner for all LLM calls via `github.com/briandowns/spinner`
- Mockery configuration for generating typed mocks of `LLMProvider`
- Coverage measurement updated to use `-coverpkg=./...` for accurate cross-package reporting
