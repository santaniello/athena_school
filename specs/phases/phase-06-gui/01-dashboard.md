# Spec: GUI Dashboard

## Goal

Provide a web-based dashboard that visualizes the user's learning progress, scores over time, and knowledge map. The CLI remains primary — the GUI is a companion for visual review.

## User Story

> As a developer, I want to open a browser dashboard to see my progress chart and knowledge map, so I can review my evolution without reading raw JSON.

## Acceptance Criteria

- [ ] `athena serve` starts a local web server (default: `http://localhost:7654`)
- [ ] Dashboard shows: total sessions, avg score, sessions per day chart
- [ ] Knowledge map shows all topics and subtopics with colour-coded mastery levels
- [ ] Clicking a topic shows session history for that topic
- [ ] Dashboard reads from `~/.config/athena/progress.json` — same data as CLI
- [ ] Server shuts down cleanly on Ctrl+C

## CLI Usage

```bash
athena serve
athena serve --port 8080
```

## Knowledge Map Mastery Levels

| Score avg | Colour | Label |
|---|---|---|
| ≥ 8.0 | Green ✅ | Mastered |
| 6.0 – 7.9 | Yellow ⚠️ | Developing |
| < 6.0 | Red ❌ | Needs work |
| No data | Grey | Not started |

## Knowledge Map Display

```
system-design
 ├── caching          ✅  8.2
 ├── load-balancing   ⚠️  6.7
 ├── sharding         ❌  4.2
 └── replication      ░   —
```

## Directory Structure

```
internal/
└── server/
    ├── server.go        # HTTP server setup
    ├── handlers/
    │   ├── progress.go  # GET /api/progress
    │   └── topics.go    # GET /api/topics
    └── static/          # embedded frontend (HTML/JS)
        ├── index.html
        ├── dashboard.js
        └── style.css
cmd/athena/
└── cmd_serve.go
```

## API Endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/api/progress` | All progress entries as JSON |
| GET | `/api/progress/:topic` | Entries filtered by topic |
| GET | `/api/topics` | Topic tree with aggregated scores |

## Frontend Stack

- Vanilla JS or Alpine.js (no build step)
- Chart.js for the sessions-over-time chart
- Static files embedded in the Go binary via `go:embed`

## Implementation Notes

- Use Go's `net/http` standard library — no external framework
- Embed static files with `//go:embed static/*` so no separate file serving is needed
- CORS: allow `localhost` only
- No authentication — local tool, single-user

## Done When

```bash
$ athena serve
# → opens browser automatically (or prints URL)
# → dashboard shows progress charts and knowledge map
```
