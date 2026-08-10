# Phase 7.2 — Whiteboard / Architecture Mode

## Goal

User draws a system architecture diagram and receives a scored evaluation from the AI.

## Semantic Model

```go
type Diagram struct {
    Nodes []Node `json:"nodes"`
    Edges []Edge `json:"edges"`
}

type Node struct {
    ID   string `json:"id"`
    Type string `json:"type"` // api-server | database | cache | queue | load-balancer | cdn | etc.
}

type Edge struct {
    Source string `json:"source"`
    Target string `json:"target"`
    Label  string `json:"label"`
}
```

## Evaluation Dimensions

- **Scalability** — can the system handle load growth?
- **Reliability** — single points of failure, replication
- **Cost** — over-engineering vs. under-engineering
- **Trade-offs** — explicit acknowledgment of design choices
- **Design quality** — separation of concerns, standard patterns

## Tasks

- [ ] `internal/domain/architecture/` — diagram model + deterministic checks
- [ ] Frontend: visual editor (React Flow or tldraw — decided during implementation)
- [ ] Deterministic rules evaluated in Go: single point of failure, missing replication
- [ ] LLM evaluation: trade-offs, missing concerns, open-ended feedback
- [ ] Score displayed per dimension + overall Architecture Score

## Acceptance Criteria

- User places nodes and edges in the visual editor
- Submitting the diagram returns scores for all 5 dimensions
- Deterministic rule "no replication on database" triggers a warning without LLM call
- Architecture Score is saved to the `evaluations` table
