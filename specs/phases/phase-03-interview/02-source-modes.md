# Spec: Source Modes

## Goal

Let the user control which knowledge sources Athena uses when generating explanations and evaluating answers. This is essential for users who want to be graded on their own notes only.

## User Story

> As a developer, I want to run `athena interview system-design --source strict-notes` so that the AI only asks about and evaluates content from my own notes, not general knowledge.

## Acceptance Criteria

- [ ] All session commands accept `--source <mode>` flag
- [ ] `web` mode (default): Athena uses its general knowledge
- [ ] `notes` mode: Athena augments responses with the user's ingested notes
- [ ] `strict-notes` mode: Athena is instructed to restrict itself to user notes only
- [ ] If `notes` or `strict-notes` is used but no notes are ingested, show a clear warning
- [ ] Source mode can be set as default in config: `athena config set source web`

## CLI Usage

```bash
athena study caching --source notes
athena study caching --source web
athena interview system-design --source strict-notes
athena config set source notes
```

## Source Mode Behaviour

| Mode | Behaviour |
|---|---|
| `web` | No extra context injected. Model uses its training knowledge. |
| `notes` | User notes for the topic are retrieved and injected as context. Model may supplement with general knowledge. |
| `strict-notes` | User notes injected + system prompt instructs model to only use provided context. |

## System Prompt Additions per Mode

### `web` (no addition)
```
(nothing added)
```

### `notes`
```
Use the following notes from the user's knowledge base as additional context.
You may also use your general knowledge to supplement.

--- USER NOTES ---
{{.NotesContext}}
-----------------
```

### `strict-notes`
```
You MUST base your answers ONLY on the notes provided below.
Do not use any knowledge outside of this context.
If the notes do not contain enough information, say so explicitly.

--- USER NOTES ---
{{.NotesContext}}
-----------------
```

## Directory Structure

```
internal/
└── source/
    ├── mode.go          # SourceMode type + constants
    └── injector.go      # BuildContext(mode, topic, notes) string
```

## Implementation Notes

- `SourceMode` is a `string` type with constants: `Web`, `Notes`, `StrictNotes`
- Validation: if mode is unknown, return an error with valid options listed
- The `injector.BuildContext()` function is called by every session type before sending the first prompt
- Notes retrieval (Phase 4) will be plugged in here via an interface; for now, notes context is an empty string unless Phase 4 is complete

## Config Integration

```yaml
# ~/.config/athena/config.yaml
source: web   # default source mode
```

## Done When

```bash
$ athena study caching --source strict-notes
# → system prompt instructs model to use notes only (verifiable by checking prompt output in debug mode)

$ athena study caching --source unknown-mode
# → error: invalid source mode "unknown-mode" — valid modes: web, notes, strict-notes
```
