# Spec: Config System

## Goal

Provide a persistent configuration file so the user can set their preferred AI provider and model once, without repeating flags every command.

## User Story

> As a developer, I want to run `athena config set provider ollama` once and have all subsequent commands use Ollama automatically.

## Acceptance Criteria

- [ ] `athena config set provider <name>` saves the provider
- [ ] `athena config set model <name>` saves the model
- [ ] `athena config get` prints the current configuration
- [ ] Config persists between CLI invocations (written to disk)
- [ ] If no config exists, sensible defaults are used (`provider: ollama`, `model: llama3`)
- [ ] Config file location follows XDG: `~/.config/athena/config.yaml`

## Config File Format

```yaml
provider: ollama
model: llama3
ollama:
  host: http://localhost:11434
```

## Directory Structure

```
internal/
└── platform/
    └── config/
        ├── config.go       # Config struct + Load/Save
        └── config_test.go
```

`config` is a foundational package — it sets no application policy and is imported by `cmd/` and domain packages, never the reverse.

## Config Struct

```go
type Config struct {
    Provider string       `yaml:"provider"`
    Model    string       `yaml:"model"`
    Ollama   OllamaConfig `yaml:"ollama"`
}

type OllamaConfig struct {
    Host string `yaml:"host"`
}
```

## CLI Commands

```
athena config set provider ollama
athena config set model llama3
athena config set ollama.host http://localhost:11434
athena config get
```

## Implementation Notes

- Use `gopkg.in/yaml.v3` for serialization
- `config.Load()` returns defaults if the file does not exist (never errors on missing file)
- `config.Save()` creates the directory if needed
- Commands should call `config.Load()` at startup; each command may override via `--provider` and `--model` flags

## Done When

```bash
$ athena config set provider ollama
✓ provider set to "ollama"

$ athena config set model llama3
✓ model set to "llama3"

$ athena config get
provider: ollama
model:    llama3
```
