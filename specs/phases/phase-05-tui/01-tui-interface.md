# Spec: TUI Interface

## Goal

Replace the plain stdout/stdin CLI interactions with a richer terminal UI using Bubble Tea. The core logic stays in `internal/` — only the presentation layer changes.

## User Story

> As a developer, I want a polished terminal experience with proper layout, visible timers, and scrollable output, so I can focus on learning rather than fighting with raw terminal output.

## Acceptance Criteria

- [ ] `athena tui` launches the TUI mode (or it becomes the default)
- [ ] All existing commands work inside the TUI (study, challenge, interview)
- [ ] Interview timer is shown in a dedicated status bar
- [ ] LLM output is streamed into a scrollable text area
- [ ] User input is captured in a styled text input at the bottom
- [ ] `q` or `Ctrl+C` exits cleanly
- [ ] TUI degrades gracefully: if terminal width < 80, falls back to plain CLI

## CLI Usage

```bash
athena tui
athena study system-design --tui   # per-command opt-in (alternative)
```

## Layout

```
┌────────────────────────────────────────┐
│  Athena  ·  study › system-design      │  ← header
├────────────────────────────────────────┤
│                                        │
│  Caching is a technique that stores    │  ← scrollable
│  frequently accessed data in a fast    │     output pane
│  temporary storage layer...            │
│                                        │
├────────────────────────────────────────┤
│  ❓ What is cache invalidation?        │  ← question bar
├────────────────────────────────────────┤
│  > Your answer here_                   │  ← input
├────────────────────────────────────────┤
│  [Tab] next  [Esc] exit  [?] help      │  ← key hints
└────────────────────────────────────────┘
```

## Interview Layout (with timer)

```
┌────────────────────────────────────────┐
│  Athena  ·  interview  ·  Q 2/3        │  ← header
│                              ⏱ 03:42  │  ← timer (right-aligned)
├────────────────────────────────────────┤
│  [question + output pane]              │
├────────────────────────────────────────┤
│  > _                                   │  ← input
└────────────────────────────────────────┘
```

## Directory Structure

```
internal/
└── tui/
    ├── app.go           # root Bubble Tea model
    ├── panes/
    │   ├── header.go
    │   ├── output.go    # scrollable viewport
    │   ├── input.go     # text input
    │   └── statusbar.go # timer, hints
    └── styles/
        └── styles.go    # Lip Gloss style definitions
cmd/athena/
└── cmd_tui.go
```

## Technology

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — layout and styling
- [Bubbles](https://github.com/charmbracelet/bubbles) — `viewport`, `textinput` components

## Architecture Note

The TUI layer only handles rendering and input. It delegates to the same `internal/study`, `internal/challenge`, and `internal/interview` session types. Sessions must expose a channel-based API for streaming output:

```go
type Session interface {
    Start(ctx context.Context) <-chan Event
    Answer(text string)
    Stop()
}

type Event struct {
    Type    EventType  // Token | Question | Feedback | Done | Error
    Content string
}
```

## Implementation Notes

- The plain CLI commands are kept intact — TUI is additive
- Terminal width detection: `tea.WindowSizeMsg` from Bubble Tea
- If width < 80 columns, print a warning and fall back to CLI mode
- Streaming tokens are sent via channel and appended to the viewport content

## Done When

```bash
$ athena tui
# → launches the TUI, user navigates to "study", runs a session,
#   sees streamed output, types an answer, sees feedback — all in the styled layout
```
