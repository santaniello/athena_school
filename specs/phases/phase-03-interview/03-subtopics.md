# Spec: Subtopics & Topic Discovery

## Goal

Allow commands to operate on both a broad topic and a specific subtopic. When no subtopic is given, Athena suggests related ones so the user can navigate deeper.

## User Story

> As a developer, I want to run `athena study system-design` and see a list of subtopics I can dive into, so I can choose where to focus next.

## Acceptance Criteria

- [ ] All commands accept an optional `<subtopic>` argument
- [ ] When only a topic is given, Athena generates a list of suggested subtopics
- [ ] The user can select a subtopic interactively from the list
- [ ] The user can also proceed without selecting (study the broad topic)
- [ ] Subtopic names are normalised to kebab-case for display
- [ ] If the user has progress data, subtopics with low scores appear first with a `⚠️` marker

## CLI Usage

```bash
athena study system-design              # broad topic → suggests subtopics
athena study system-design caching      # goes directly to subtopic
athena challenge system-design          # same pattern
athena interview system-design caching  # same pattern
```

## Session Flow (when subtopic is omitted)

```
1. Print topic header
2. Ask LLM to suggest 5-7 subtopics for the topic
3. If progress data exists, annotate weak subtopics
4. Print numbered list
5. Prompt: "Select a subtopic [1-7] or press Enter to study the full topic:"
6. Route to selected subtopic or continue with broad topic
```

## Terminal Output Format

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Study: system-design
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Subtopics:
  [1] caching
  [2] load-balancing
  [3] sharding          ⚠️  (avg score: 4.2)
  [4] replication
  [5] message-queues

Select a subtopic [1-5] or press Enter to study the full topic:
> _
```

## Prompt Template

### Subtopic suggestion
```
List 5 to 7 important subtopics for "{{.Topic}}" in the context of backend engineering and system design.
Return only a numbered list of short kebab-case names (e.g., "load-balancing"). No explanations.
```

## Directory Structure

```
internal/
└── topics/
    ├── topics.go        # ParseTopic, ParseSubtopic, NormaliseKebab
    └── suggest.go       # SuggestSubtopics(ctx, provider, topic) []string
```

## Implementation Notes

- `NormaliseKebab(s string) string` lowercases and replaces spaces/underscores with `-`
- Subtopic list from the LLM is parsed line-by-line, stripping leading numbers and punctuation
- Progress integration: `topics.AnnotateWithProgress(subtopics, progress.Store)` adds the `⚠️` marker
- Input validation: if user enters a number out of range, re-prompt once, then default to full topic

## Done When

```bash
$ athena study system-design
# → shows numbered subtopic list, user selects "3", session proceeds on "sharding"

$ athena study system-design caching
# → skips selection, goes directly to caching session
```
