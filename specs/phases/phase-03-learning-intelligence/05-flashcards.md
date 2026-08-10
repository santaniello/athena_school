# Phase 3.5 — Flashcards

## Goal

Approved Knowledge Items are turned into flashcards reviewed with spaced repetition (SM-2).

## Domain

```go
type Flashcard struct {
    ID            string
    KnowledgeItem string
    Topic         string
    Type          string   // front_back | cloze | multiple_choice
    Front         string
    Back          string
    Options       []string
    CorrectOption int
    Status        string // draft | active | suspended
    CreatedAt     time.Time
}

type FlashcardReview struct {
    FlashcardID string
    ReviewedAt  time.Time
    Quality     int     // 0–5 (SM-2)
    Interval    int     // days
    EaseFactor  float64
}
```

## Generation

- [ ] LLM extracts 3 flashcard types from each approved Knowledge Item:
  - `front_back` — question / answer
  - `cloze` — fill-in-the-blank
  - `multiple_choice` — 4 options
- [ ] Cards created as `draft`; user approves before they enter the review queue

## SM-2 Scheduler

- [ ] `internal/application/flashcards/sm2.go` — SM-2 algorithm
- [ ] Calculates next review interval from quality rating (0–5)
- [ ] Daily review queue: cards due today, sorted by overdue days descending

## Review UI

- [ ] Screen: front of card → "Show answer" button → back → rating buttons (❌ / ⚠️ / ✅)
- [ ] Session summary: correct count, incorrect count, next review date

## Gap Integration

- [ ] Cards with recurring errors feed the gap detector for their topic
- [ ] Cards with ≥ 3 consecutive failures trigger a study session recommendation

## Acceptance Criteria

- Approving a Knowledge Item shows a prompt to generate flashcards
- Generated cards appear as drafts in the review queue
- Reviewing a card with rating 5 schedules it further in the future than rating 1
- Daily queue shows only cards due today or overdue
- Session summary displays accurate counts
