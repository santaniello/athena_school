# Spec: Algorithm Mode

## Goal

Add a LeetCode-style problem-solving mode where the user writes actual code, Athena runs it against test cases, evaluates correctness and complexity, and provides feedback.

## User Story

> As a developer preparing for coding interviews, I want to solve algorithm problems inside Athena, run my solution against test cases, and get feedback on time complexity — all without leaving my terminal.

## Acceptance Criteria

- [ ] `athena algo <problem>` presents the problem statement and constraints
- [ ] The user writes their solution in a temp file opened in `$EDITOR`
- [ ] `athena algo <problem> --run solution.go` runs the solution against test cases
- [ ] Output shows which test cases passed/failed with input/expected/got
- [ ] Athena evaluates: correctness, time complexity, space complexity, code quality
- [ ] `--difficulty easy|medium|hard` filters problems
- [ ] `athena interview algorithms` runs a timed coding interview session (2 problems, 20 min each)

## CLI Usage

```bash
athena algo two-sum
athena algo two-sum --run solution.go
athena algo lru-cache --difficulty medium
athena interview algorithms
athena interview algorithms --problems 2 --time 25
```

## Session Flow

```
1. Fetch problem by name (from local problem bank or LLM-generated)
2. Print problem statement, constraints, examples
3. Open $EDITOR with a starter template file
4. User writes solution, saves, exits editor
5. Athena compiles and runs solution against test cases
6. Print results: pass/fail per test case
7. Ask LLM to evaluate correctness + complexity
8. Print structured feedback
```

## Problem Format

```go
type Problem struct {
    Name        string
    Difficulty  string      // easy | medium | hard
    Description string
    Examples    []Example
    Constraints []string
    TestCases   []TestCase
    StarterCode map[string]string  // language → starter template
}

type TestCase struct {
    Input    string
    Expected string
}
```

## Directory Structure

```
internal/
└── algo/
    ├── session.go       # Algorithm session
    ├── runner/
    │   ├── runner.go    # interface: Run(code, testCases) Results
    │   └── go_runner.go # Go implementation
    ├── problems/
    │   ├── bank.go      # load from embedded JSON bank
    │   └── problems.json
    └── prompts.go
cmd/athena/
└── cmd_algo.go
```

## Test Runner (Go)

```go
type Runner interface {
    Run(ctx context.Context, solutionPath string, testCases []TestCase) ([]TestResult, error)
}

type TestResult struct {
    Input    string
    Expected string
    Got      string
    Passed   bool
    Error    string
}
```

Implementation for Go:
1. Write a test harness file that imports the user's solution
2. Run `go test` with a timeout
3. Parse output to build `[]TestResult`

## Evaluation Prompt

```
A developer solved the "{{.Problem}}" algorithm problem.

Their solution:
{{.Code}}

Test results: {{.PassCount}}/{{.TotalCount}} passed.
{{if .FailedCases}}
Failed cases:
{{range .FailedCases}}- Input: {{.Input}} → Expected: {{.Expected}}, Got: {{.Got}}
{{end}}{{end}}

Evaluate:
1. Correctness — are there logical errors?
2. Time complexity — what is the Big-O, and can it be improved?
3. Space complexity — is memory usage optimal?
4. Code quality — is it readable and idiomatic?

✅ Strengths:
⚠️ Improvements:
⭐ Score: x/10
```

## Terminal Output Format

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Algorithm: two-sum  [easy]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Given an array of integers and a target, return indices of the two
numbers that add up to target.

Example:
  Input:  nums = [2,7,11,15], target = 9
  Output: [0,1]

Constraints:
  - 2 ≤ len(nums) ≤ 10⁴
  - -10⁹ ≤ nums[i] ≤ 10⁹

Opening editor... (save and close to submit)

Running test cases...
  ✅ [2,7,11,15], target=9  → [0,1]
  ✅ [3,2,4], target=6      → [1,2]
  ❌ [3,3], target=6        → Expected [0,1], got []

2/3 tests passed.

[feedback printed here]
```

## Security Notes

- Code execution is sandboxed with a timeout (default: 10 seconds per test case)
- Only Go is supported in MVP; Python support is a future extension
- Never run user code with elevated privileges

## Done When

```bash
$ athena algo two-sum
# → shows problem, opens editor, user writes solution
$ athena algo two-sum --run solution.go
# → runs tests, prints results and feedback
```
