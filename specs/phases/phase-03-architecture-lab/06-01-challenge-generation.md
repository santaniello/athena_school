# Phase 3.6.1 — Challenge Generation (from a studied session)

## Goal

The first challenges of the lab come from the user's own study. From a session the user already
studied, the AI generates a system-design challenge and its rubric, which are saved to the catalog
for the user to start whenever they choose.

This is the minimal generation the catalog ([3.7](07-challenge-catalog-ui.md)) needs to have real
content. [3.14](14-ai-challenge-generation.md) extends it later (generation started from the chat,
tutor offers, duplicate avoidance); nothing here depends on the chat.

## Decisions

1. **Generated from a studied session.** The input is a session and a difficulty. The challenge's
   topic is that session's topic and `SourceSessionID` is that session, as decided in
   [3.1](01-challenge-domain-and-persistence.md).
2. **Context the model receives, in this order:**
   - the session's topic and goal;
   - the session's conversation summary, when one exists
     ([2.24](../phase-02-knowledge-engine/24-conversation-summary.md));
   - the user's profile: focus area and experience level;
   - excerpts from the session's attached notes, when there are any: the session-scoped search of
     [2.5](../phase-02-knowledge-engine/05-rag-integration.md) with the topic as the query, at
     most 8 chunks and 6,000 characters in total;
   - the language: the user's assistant language.

   Missing pieces are skipped silently. If the notes index is not ready, the notes are left out
   and generation continues; it never fails because of them.
3. **The user picks the difficulty**, pre-selected from the profile's experience level, in the
   generation dialog ([3.7](07-challenge-catalog-ui.md)). The difficulty sets the time limit
   through `DefaultTimeLimit` ([3.3](03-attempts-and-session-link.md)).
4. **One generation call, structured output.** The model returns JSON with the challenge
   (title, scenario, functional and non-functional requirements, constraints, objectives) and its
   challenge criteria (name, description, relative weight, mandatory flag, five level
   descriptors). The cost is one LLM call, plus one embedding call when notes are searched.
5. **Validated before anything is stored.** The output goes through `Challenge.Validate` and the
   rubric rules of [3.2](02-rubrics.md) (1 to 6 challenge criteria, five levels each, weights
   mergeable to 100). Invalid output fails as `ErrInvalidGeneration` and stores nothing.
6. **Challenge and rubric are saved together or not at all.** One transaction creates the
   `AI_GENERATED` challenge and its first rubric version. There is never a generated challenge
   without a rubric.
7. **No review step.** The challenge goes straight to the catalog and the user sees its statement
   only when they start the exam. The challenge is not started automatically.
8. **The statement does not give the answer.** The prompt forbids including a solution, an
   architecture, or the names of the components the user should choose. Nothing in the code can
   guarantee this; the prompt rule is the control, with a test that the instruction is present.
9. **Failure is the user's to retry.** On a provider error, a cancelled context or invalid
   output, an error is returned and nothing is stored. There are no hidden automatic retries.
10. **One Architecture Lab model setting, created here.** Every AI call of the lab (generation,
    hints, diagram and defense evaluation) uses one model, chosen in Settings, next to the other
    per-feature models ([2.31](../phase-02-knowledge-engine/31-llm-model-param.md)):
    `ModelSettings.ArchitectureLab`, empty meaning the default of the new `TaskArchitectureLab`
    task (tier medium). The call goes through the existing provider and model router
    ([1.5](../phase-01-desktop-mvp/05-llm-service.md)) and its tokens are recorded in the same
    usage accounting as a study turn. [3.10.1](10-01-tutor-hints.md),
    [3.11](11-architecture-evaluator.md) and [3.12](12-architecture-defense.md) reuse it.
11. **Duplicates are allowed.** Generating twice for the same session creates two challenges;
    avoiding that belongs to 3.14.
12. **No data compatibility to keep.**

## Application

`internal/application/exercise.GenerationService`:

```go
func (s *GenerationService) GenerateChallenge(ctx, sessionID string, difficulty Difficulty) (Challenge, error)
```

Order of work:

1. Validate the difficulty; check the session exists (`ErrSourceSessionNotFound`) through the
   narrow `SessionTopicReader` port from [3.1](01-challenge-domain-and-persistence.md), which also
   returns the session's goal.
2. Build the context (decision 2) through small ports, so the package imports neither `study` nor
   `knowledge`: `SummaryReader`, `ProfileReader`, `NotesSearcher`.
3. Call the LLM with the generation prompt and parse the JSON.
4. Build and validate the challenge and the rubric criteria (decision 5).
5. In one transaction, call `ChallengeService.Create` and `RubricService.SetChallengeRubric`.

The prompt lives in `internal/application/exercise/generation_prompt.go`, a pure function of the
context so its content is unit tested. Errors are exported sentinels.

## Desktop binding

A thin adapter in `internal/interfaces/desktop/exercise.go`:

- `GenerateChallenge(sessionId, difficulty)` returns the challenge summary (id, title, topic,
  difficulty, time limit). `wailsjs` is regenerated.

No screen in this spec.

## Tasks

Each slice is TDD, keeps the build green and is committed on its own.

- [ ] Application: ports (`SummaryReader`, `ProfileReader`, `NotesSearcher`, LLM), error sentinels
- [ ] Context builder: topic, goal, summary, profile, notes bounded by 8 chunks and 6,000
      characters; missing pieces skipped; notes failure does not fail generation
- [ ] Prompt builder: language, difficulty, the "no solution" rule, JSON schema
- [ ] Parsing and validation of the model output into a challenge and rubric criteria
- [ ] `GenerateChallenge`: success path saving challenge and rubric in one transaction
- [ ] Failures: unknown session, provider error, cancelled context, invalid output store nothing
- [ ] `ModelSettings.ArchitectureLab`, `TaskArchitectureLab` and its tier, the field in the model
      settings binding and Settings screen ("Architecture Lab model", with Reset to default)
- [ ] Binding and `wailsjs` regenerated; mocks regenerated; usage accounting wired
- [ ] Docs: CHANGELOG entry

## Acceptance Criteria

- Generating for an existing session creates one `AI_GENERATED` challenge with that session's
  topic and id, the chosen difficulty, the default time limit for it, and a first rubric version
  that includes the five base criteria plus the generated ones.
- A session with no summary and no notes still generates; one with notes includes at most 8
  chunks and 6,000 characters of them in the prompt.
- When the notes index is not ready, generation still succeeds without notes.
- The prompt contains the assistant language, the difficulty and the instruction not to reveal a
  solution.
- Output with a missing field, an empty list where one is required, more than six challenge
  criteria, a level count other than five, or non-JSON text is rejected and nothing is stored.
- If saving the rubric fails, the challenge is not saved either.
- A provider error or a cancelled context returns an error and stores nothing; calling again
  works.
- The call makes exactly one LLM request with the Architecture Lab model (or its default when
  the field is empty), and its usage is recorded.
- Settings shows an "Architecture Lab model" selector that saves, resets to the default and
  survives a restart.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  application/domain code, lint and vulnerability checks pass.

## Out of scope

- The generation dialog and the catalog ([3.7](07-challenge-catalog-ui.md)).
- Generation started from the chat, tutor offers, duplicate avoidance, and the offer flow
  ([3.14](14-ai-challenge-generation.md), [3.15](15-open-lab-from-chat.md)).
- A review or edit step, regenerating an existing challenge, manual authoring.
- Verifying that a statement hides its solution beyond the prompt rule.
