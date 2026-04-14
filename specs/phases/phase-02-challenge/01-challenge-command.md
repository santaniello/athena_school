# Spec: Challenge Command

## Goal

Give the user a practical problem to solve (not just explain). Unlike study mode, the user must propose a solution to a real scenario and receive structured evaluation against defined criteria.

## User Story

> As a developer, I want to run `athena challenge system-design` and get a real-world problem to solve, so I can practice applying concepts rather than just explaining them.

## Acceptance Criteria

- [ ] `athena challenge <topic>` presents a practical scenario problem
- [ ] `athena challenge <topic> <subtopic>` focuses on a specific subtopic
- [ ] The problem includes: scenario, requirements, and constraints
- [ ] User submits a free-text solution
- [ ] Athena evaluates against five criteria (see below)
- [ ] Output includes strengths, improvements, and a score
- [ ] Session can be retried with a different problem using `--new`

## CLI Usage

```bash
athena challenge system-design
athena challenge system-design caching
athena challenge system-design caching --new
```

## Session Flow

```
1. Ask LLM to generate a challenge problem for the topic
2. Print problem to terminal
3. Read user's solution from stdin (multi-line)
4. Ask LLM to evaluate against 5 criteria
5. Print structured feedback with score
6. Ask: retry with new problem, or exit?
```

## Directory Structure

```
internal/
└── challenge/
    ├── session.go       # Challenge session
    ├── prompts.go
    └── session_test.go
cmd/athena/
└── cmd_challenge.go
```

## Prompt Templates

### Problem generation prompt
```
You are Athena, a technical interview coach.
Generate a realistic system design challenge for the topic "{{.Topic}}".

Format:
## Scenario
<2-3 sentence context>

## Requirements
- <functional requirement>
- <functional requirement>

## Constraints
- <scale / technical constraint>

Keep it concise. Do not include hints or expected answers.
```

### Evaluation prompt
```
Evaluate the following solution to this system design challenge.

Challenge: {{.Problem}}

User's solution:
{{.Solution}}

Score each criterion from 1-10 and explain briefly:

1. Clarity — is the solution well-structured and easy to follow?
2. Organization — is the architecture logically organized?
3. Scalability — does the solution handle scale requirements?
4. Trade-offs — are trade-offs identified and justified?
5. Technical depth — does the solution show deep understanding?

Output format:

## Evaluation

| Criterion       | Score | Notes |
|-----------------|-------|-------|
| Clarity         | x/10  | ...   |
| Organization    | x/10  | ...   |
| Scalability     | x/10  | ...   |
| Trade-offs      | x/10  | ...   |
| Technical depth | x/10  | ...   |

✅ Strengths:
- <point>

⚠️ Improvements:
- <point>

⭐ Overall Score: <avg>/10
```

## Terminal Output Format

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Challenge: system-design › caching
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## Scenario
A social media platform with 10M daily active users needs a caching
strategy for user profile data...

## Requirements
- Profile reads < 50ms p99
- Handle 100k reads/sec at peak

## Constraints
- Budget limits to 3 cache nodes
- Profile data changes ~5% per day

Your solution (press Enter twice to submit):
> _

[evaluation printed here]

Try again? [y/n]:
```

## Done When

```bash
$ athena challenge system-design caching
# → presents a scenario, accepts solution, prints scored evaluation table
```
