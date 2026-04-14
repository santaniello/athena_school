# Spec: Study Command

## Goal

The core learning feature: the user picks a topic, Athena explains it, asks questions, and gives feedback on the answers. This is the first end-to-end feature.

## User Story

> As a developer, I want to run `athena study system-design` and get an interactive guided explanation with questions, so I can actively learn instead of passively reading.

## Acceptance Criteria

- [ ] `athena study <topic>` starts a guided study session
- [ ] `athena study <topic> <subtopic>` focuses on a specific subtopic
- [ ] Athena presents an explanation of the topic
- [ ] Athena asks at least one question after the explanation
- [ ] User types an answer in the terminal
- [ ] Athena evaluates the answer and gives structured feedback
- [ ] Session ends gracefully when the user types `exit` or presses Ctrl+C
- [ ] Errors from the LLM provider are shown clearly

## CLI Usage

```bash
athena study system-design
athena study system-design caching
athena study system-design caching --provider ollama --model llama3
```

## Session Flow

```
1. Build system prompt for the topic/subtopic
2. Ask LLM for an explanation (streamed to terminal)
3. Ask LLM to generate one question for the user
4. Print question, read user's answer from stdin
5. Ask LLM to evaluate the answer
6. Print structured feedback
7. Ask user: continue (next question) or exit?
```

## Directory Structure

```
internal/
└── study/
    ├── session.go       # Session struct + Run()
    ├── prompts.go       # prompt templates
    └── session_test.go
cmd/
└── athena/
    └── study.go         # cobra command wiring
```

`internal/study/` is a domain package. It may import from `internal/platform/` (e.g. `llm`), but never from `cmd/`. The cobra wiring in `cmd/athena/study.go` assembles the dependencies (config, provider) and calls `study.Run()`.

## Prompt Templates

### Explanation prompt
```
You are Athena, a technical learning assistant.
Explain the topic "{{.Topic}}" clearly and concisely for a backend developer.
Cover the key concepts, when to use it, and common trade-offs.
Keep the explanation under 300 words.
```

### Question prompt
```
Now ask the user one specific question to test their understanding of "{{.Topic}}".
Ask only the question — no explanation.
```

### Evaluation prompt
```
The user answered the following question about "{{.Topic}}":

Question: {{.Question}}
Answer: {{.Answer}}

Evaluate the answer using this format:

✅ Strengths:
- <point>

⚠️ Improvements:
- <point>

⭐ Score: <n>/10
```

## Terminal Output Format

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Study: system-design › caching
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[explanation streamed here]

❓ Question:
What are the main cache invalidation strategies and their trade-offs?

Your answer (press Enter twice to submit):
> _

[feedback printed here]

Continue? [y/n]:
```

## Implementation Notes

- Use a multi-turn conversation: keep `[]ChatMessage` in memory for the full session
- Stream the LLM response to terminal using `fmt.Print` token by token
- Read multi-line answers: collect lines until two consecutive empty lines or a `.` on its own line
- `--provider` and `--model` flags override config defaults

## Done When

```bash
$ athena study system-design caching
# → prints explanation, asks a question, accepts answer, prints feedback
```
