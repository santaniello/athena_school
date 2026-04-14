# Spec: Project Setup

## Goal

Bootstrap the Go CLI project with the minimal structure needed to add features incrementally. The result is a binary that can be installed and runs `athena --help` successfully.

## User Story

> As a developer, I want to run `athena --help` and see available commands, so I know the tool is installed and working.

## Acceptance Criteria

- [ ] `go build ./...` succeeds with no errors
- [ ] `athena --help` prints a usage message with at least one placeholder command
- [ ] `athena version` prints the current version string
- [ ] Project follows package-oriented design (`cmd/`, `internal/platform/`)
- [ ] Pre-commit hook is installed and all quality gates pass (`make install-hooks`)

## AI Assistant Configuration Files (pre-requisito)

O repositório deve conter dois arquivos de configuração para assistentes de IA na raiz do projeto, **antes de qualquer implementação**:

- `CLAUDE.md` — regras para o Claude Code
- `GEMINI.md` — regras para o Gemini CLI

Ambos definem o ciclo TDD obrigatório (Red → Green → Refactor → Commit), padrões de código Go, regras de segurança, UX, documentação e versionamento. O conteúdo completo de referência está em `/home/fsantaniello/Workspace-Golang/xp-cli/CLAUDE.md` e `/home/fsantaniello/Workspace-Golang/xp-cli/GEMINI.md`.

Esses arquivos devem ser criados imediatamente após o `git init`, antes de qualquer commit.

---

## Pre-commit Quality Gate (obrigatório antes de qualquer commit)

O repositório usa `.githooks/pre-commit` para garantir qualidade a cada commit. **Este setup deve ser feito imediatamente após clonar o repo — nenhuma feature deve ser commitada sem ele estar ativo.**

### Arquivo `.githooks/pre-commit`

```bash
#!/usr/bin/env bash
set -euo pipefail

export PATH="$HOME/go/bin:$PATH"

echo "Running pre-commit quality gate..."

# 1. Tests
echo "-> Running tests..."
go test ./...

# 2. Coverage (minimum 80%)
echo "-> Checking coverage..."
go test ./... -coverprofile=coverage.out 2>/dev/null || true
if [ ! -f coverage.out ]; then
  echo "FAIL: coverage.out was not generated."
  exit 1
fi
TOTAL=$(go tool cover -func=coverage.out | grep '^total:' | awk '{print $3}' | tr -d '%')
rm -f coverage.out
echo "   Coverage: ${TOTAL}%"
if awk "BEGIN{exit !($TOTAL < 80)}"; then
  echo "FAIL: Coverage ${TOTAL}% is below the 80% threshold."
  exit 1
fi

# 3. Lint (golangci-lint includes gosec; fallback to go vet)
echo "-> Running lint..."
if command -v golangci-lint &>/dev/null && golangci-lint run --version &>/dev/null 2>&1; then
  golangci-lint run
else
  go vet ./...
fi

# 4. Vulnerabilities
echo "-> Running govulncheck..."
govulncheck ./...

echo "OK: All quality gates passed."
```

O arquivo deve ter permissão de execução: `chmod +x .githooks/pre-commit`.

### Makefile target

```makefile
install-hooks:
    git config core.hooksPath .githooks
    chmod +x .githooks/pre-commit
    @echo "Pre-commit hook installed."
```

### Instalação

```bash
make install-hooks
```

---

## Directory Structure

```
athena/
├── cmd/
│   └── athena/
│       ├── main.go          # entrypoint — calls root.Execute()
│       ├── root.go          # root cobra command + global flags
│       └── version.go       # version constant + `version` subcommand
├── internal/
│   └── platform/            # foundational packages (no app policy)
├── go.mod
├── go.sum
├── Makefile
├── CLAUDE.md                # AI assistant rules (Claude Code)
└── GEMINI.md                # AI assistant rules (Gemini CLI)
```

### Package-Oriented Design rules

- `cmd/athena/` owns all CLI wiring (Cobra setup, flag parsing, output formatting)
- `internal/platform/` holds foundational packages that know nothing about commands
- Packages at the same level inside `internal/platform/` **cannot import each other**
- `internal/platform/` packages do not set application policy (logging config, defaults)

## Implementation Notes

- Use [Cobra](https://github.com/spf13/cobra) for CLI parsing
- Root command name: `athena`
- Version follows semver: `v0.1.0`
- Makefile targets: `build`, `install`, `test`, `lint`

## Makefile Targets

```makefile
build:
    go build -o bin/athena ./cmd/athena

install:
    go install ./cmd/athena

test:
    go test ./...

lint:
    golangci-lint run

install-hooks:
    git config core.hooksPath .githooks
    chmod +x .githooks/pre-commit
    @echo "Pre-commit hook installed."
```

## Done When

Running `athena --help` outputs:

```
Athena — active learning CLI for developers

Usage:
  athena [command]

Available Commands:
  version     Print the current version
  help        Help about any command

Flags:
  -h, --help   help for athena
```
