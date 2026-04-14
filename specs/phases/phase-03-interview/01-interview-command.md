# Spec: Interview Command

## Goal

Simulate a real technical interview: timed, multi-question, scored. The user is evaluated as if they were in an actual interview, not a study session.

## User Story

> As a developer preparing for interviews, I want a timed simulation where I'm asked multiple questions back-to-back and receive an overall score at the end, so I can practice under realistic conditions.

## Acceptance Criteria

- [ ] `athena interview <topic>` starts a timed interview session
- [ ] Session asks 3-5 questions in sequence (default: 3)
- [ ] Each question has a time limit (default: 5 minutes)
- [ ] Timer is visible during the question (counts down in the terminal title or inline)
- [ ] When time expires, the answer is auto-submitted as-is
- [ ] At the end, a final report is printed with per-question scores and an overall score
- [ ] `--questions N` overrides the number of questions
- [ ] `--time N` overrides minutes per question

## CLI Usage

```bash
athena interview system-design
athena interview system-design caching
athena interview system-design --questions 5 --time 3
```

## Session Flow

```
1. Generate N interview questions for the topic
2. For each question:
   a. Print question with index (e.g., "Question 1/3")
   b. Start countdown timer
   c. Accept multi-line answer from user
   d. Auto-submit when timer reaches 0
   e. Store (question, answer, time_used)
3. Evaluate all answers at once (or per-question)
4. Print final report with per-question breakdown and overall score
```

## Directory Structure

```
internal/
└── interview/
    ├── session.go       # Interview session + timer
    ├── prompts.go
    ├── timer.go         # countdown display
    └── session_test.go
cmd/athena/
└── cmd_interview.go
```

## Prompt Templates

### Question generation
```
You are a senior engineer conducting a technical interview on "{{.Topic}}".
Generate {{.Count}} distinct interview questions of increasing difficulty.
Return them as a numbered list only — no answers, no hints.
```

### Batch evaluation
```
Evaluate each answer from this technical interview on "{{.Topic}}".

{{range .QAs}}
Question {{.Index}}: {{.Question}}
Answer: {{.Answer}}
Time used: {{.TimeUsed}}s / {{.TimeLimit}}s

---
{{end}}

For each question, provide:
- Score (1-10)
- One strength
- One improvement

Then provide:
Overall Score: <avg>/10
Recommendation: <one sentence on what to study next>
```

## Terminal Output Format

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Interview: system-design
  3 questions · 5 min each
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Question 1/3  [04:32 remaining]
How would you design a URL shortener for 1B users?

> _

[time expires]
⏰ Time's up! Submitting your answer...

[... next questions ...]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Final Report
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Q1  URL shortener         8/10
Q2  Cache invalidation    6/10
Q3  Sharding strategy     5/10

⭐ Overall Score: 6.3/10
📌 Recommendation: Practice sharding — run `athena study system-design sharding`
```

## Timer Implementation Notes

- Use a goroutine to update the countdown every second
- Display remaining time inline using `\r` (carriage return) to overwrite the line
- When the timer goroutine expires, send a signal to the input reader to stop
- Time used per question should be recorded for the evaluation prompt

## Done When

```bash
$ athena interview system-design --questions 2 --time 1
# → 2 questions, 1-minute timer each, final scored report
```
