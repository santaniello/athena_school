# Phase 2.2 — Knowledge Extraction

## Goal

After each study session, the LLM extracts concepts and creates `draft` Knowledge Items automatically.

## Flow

```text
Study session ends
    ↓
Extraction prompt sent to LLM with full session messages
    ↓
LLM returns structured JSON ([]KnowledgeItem)
    ↓
Items saved as "draft"
    ↓
UI modal: "New knowledge found" → [Save / Keep as Draft / Ignore]
```

## Tasks

- [ ] `internal/application/knowledge/extraction.go` — extraction use case
- [ ] Structured extraction prompt → validated JSON response
- [ ] Schema validation in Go before persisting (not trusting raw LLM output)
- [ ] UI modal displayed at end of session if items were extracted
- [ ] "Ignore" discards items without saving; "Save" promotes to `draft`

## Acceptance Criteria

- A completed study session triggers extraction automatically
- Extracted items are saved with `status = "draft"` and `source = "athena"`
- The modal lists each extracted concept with its definition
- Ignoring all items saves nothing to the database
- Malformed LLM JSON response is caught and logged; no crash
