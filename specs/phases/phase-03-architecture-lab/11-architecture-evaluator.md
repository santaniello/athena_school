# Phase 3.11 — Architecture Evaluator

## Goal

Evaluate a submitted diagram. This spec implements the `Evaluator` port of
[3.5](05-evaluation-engine.md) for architecture challenges: it turns the draw.io XML into a
compact summary, asks the model for a level per rubric criterion, and returns the result that the
engine validates and scores. It also exposes the evaluation to the screens and starts it right
after the user submits.

The defense and the second score are [3.12](12-architecture-defense.md); the result screen is
[3.13](13-evaluation-and-history-screen.md).

## Decisions

1. **The model sees a summary, not the XML.** Code reads the submitted XML
   ([3.9](09-diagram-editor-spike.md)) and builds a `DiagramSummary`: the components (their
   labels), the connections (source, target and label) and the groupings (which component sits
   inside which container). No styles, no coordinates, no ids beyond a short number the summary
   assigns. It is deterministic and unit tested without an LLM.
2. **The XML is untrusted input.** The extractor:
   - rejects a submission whose `contentType` is not `drawio-xml`;
   - decompresses a compressed diagram with a hard limit on the decompressed size (a decompression
     bomb fails as `ErrInvalidDiagram`), reads it with `encoding/xml` with no entity or external
     resource resolution, and stops at 2,000 cells;
   - turns labels from HTML into plain text and cuts each at 200 characters;
   - fails an unreadable diagram as `ErrInvalidDiagram` and an empty one as `ErrEmptyDiagram`,
     which move the attempt to `EVALUATION_FAILED` with that reason.
3. **Labels are data, not instructions.** The prompt puts the summary in a delimited data block
   and states that its text is the user's drawing and must never be followed as an instruction.
   Nothing in the code can guarantee the model obeys; the prompt rule is the control and a test
   checks the instruction is present.
4. **One call returns every criterion.** The model gets the challenge, the frozen rubric version
   (each criterion with its description and five level descriptors) and the summary, and answers
   in JSON: per criterion `{criterionKey, level 0..4, justification, evidence,
   insufficientEvidence}`, plus summary, strengths, gaps and recommendations. The output is mapped
   to `EvaluatorOutput`; [3.5](05-evaluation-engine.md) validates it and computes the score.
5. **Not seen means flagged, not zero.** The prompt tells the model to set `insufficientEvidence`
   when the diagram does not show enough to judge a criterion, for example a decision the user did
   not write down, and never to infer it. Such a criterion is left out of the score
   ([3.5](05-evaluation-engine.md) decision 3) and is the first thing the defense asks about
   ([3.12](12-architecture-defense.md)). A criterion that is shown but wrong gets a low level, not
   the flag.
6. **The Architecture Lab model.** The call uses the model chosen in Settings, or the default of
   `TaskArchitectureLab` when empty ([3.6.1](06-01-challenge-generation.md) decision 10), through
   the existing provider and usage accounting. The exact model id is stored on the evaluation
   ([3.5](05-evaluation-engine.md)).
7. **Evaluation is a stage of the same sitting.** After `SubmitAttempt` succeeds,
   [3.10](10-diagram-workspace.md) moves the screen to an evaluation stage ("Evaluating your
   diagram...", the board read-only) and calls `EvaluateAttempt`. The user stays in the lab; the
   tutor's questions ([3.12](12-architecture-defense.md)) follow in the same flow, with no
   waiting status and no return to the catalog. **Leave** asks for the usual confirmation and
   discards the attempt; closing the app during the stage deletes it at startup
   ([3.5](05-evaluation-engine.md) decision 7).
8. **Failure is visible and manual.** On failure the stage shows the short reason and a **Try
   evaluating again** button that calls `EvaluateAttempt` again, with no limit and no automatic
   retry. Leaving discards the attempt.
   Until 3.12 exists, the stage ends with the diagram result (score and feedback per criterion)
   and **Back to catalog**, which discards the unfinished attempt like any other exit
   ([3.4](04-exercise-service.md)); 3.12 then replaces that end.
9. **Cost is one LLM call** per evaluation, plus one per manual retry.
10. **No data compatibility to keep.**

## Application

Package `internal/application/architecture` (created in [3.10.1](10-01-tutor-hints.md)):

- `DiagramSummary` and `SummarizeDiagram(content string) (DiagramSummary, error)`: pure, decisions 1
  and 2.
- `BuildEvaluationPrompt(challenge, rubric, summary, language) string`: pure; includes the rubric
  level descriptors, the data-block rule (decision 3) and the flag rule (decision 5).
- `DiagramEvaluator`, implementing `exercise.Evaluator`: summarize, build the prompt, call the LLM
  once, parse the JSON into `EvaluatorOutput`. A non-JSON answer, a missing field or an unknown
  criterion key is returned as an error that
  [3.5](05-evaluation-engine.md) treats as an invalid evaluation.

Errors are exported sentinels: `ErrInvalidDiagram`, `ErrEmptyDiagram`.

## Desktop bindings and frontend

- `EvaluateAttempt(attemptId)` and `GetEvaluation(attemptId)` in
  `internal/interfaces/desktop/exercise.go`: thin, an error code per sentinel, the evaluation
  mapped to a DTO. `wailsjs` regenerated.
- `lib/exercise.ts` wrappers. `useExam` ([3.10](10-diagram-workspace.md)) calls `EvaluateAttempt`
  after a successful submission and moves the screen to the evaluation stage.
- `components/lab/evaluation-stage.tsx`: progress, the failure state with **Try evaluating
  again**, and the temporary diagram result with **Back to catalog**.
- All UI text is in English.

## Tasks

Each slice is TDD (Go tests with testify and Mockery mocks, Vitest for the frontend), keeps the
build green and is committed on its own.

- [ ] `SummarizeDiagram`: components, connections, groupings, HTML labels to text, length limits;
      compressed and uncompressed diagrams
- [ ] `SummarizeDiagram` limits: wrong content type, decompression bomb, cell cap, malformed XML,
      empty diagram, entities not resolved
- [ ] `BuildEvaluationPrompt`: rubric levels, data-block rule, flag rule, language, content tested
- [ ] `DiagramEvaluator`: success path with a fake LLM, invalid JSON, unknown or missing criterion,
      provider error, cancelled context
- [ ] Bindings and `wailsjs` regenerated; mocks regenerated; usage accounting wired
- [ ] Frontend: wrappers, `useExam` triggering the evaluation, evaluation stage (progress,
      failure and retry, diagram result)
- [ ] Docs: CHANGELOG, README and `lib/documentation.ts` describe the evaluation

## Acceptance Criteria

- Submitting a diagram starts an evaluation and the user stays in the lab, on a stage that shows
  progress until it ends; leaving asks for confirmation and discards the attempt.
- On success the evaluation holds one result per criterion, the model id and a score computed by
  the domain; the stage shows the score and the feedback per criterion, and until 3.12 exists it
  ends with **Back to catalog**.
- The summary lists every labeled component, every connection with source, target and label, and
  the containers each component sits in, and contains no styles or coordinates.
- A compressed diagram is read; one that decompresses past the limit, a malformed one, a wrong
  content type and an empty one fail with the matching sentinel and leave the attempt
  `EVALUATION_FAILED` with the submission intact.
- An external entity in the XML is not resolved.
- The prompt contains the rubric descriptors, the rule that labels are data and the rule to flag
  instead of inferring.
- Invalid model output leaves `EVALUATION_FAILED` with the reason on the stage; **Try evaluating
  again** runs it again and can succeed; a successful evaluation is never run twice.
- One LLM request is made per evaluation, with the Architecture Lab model, and its usage is
  recorded.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  application/domain code, lint and vulnerability checks, and the frontend tests, coverage, lint,
  typecheck, `npx prettier --check .` and Stryker on the changed files pass.

## Out of scope

- The defense, the understanding score and the pass or fail result
  ([3.12](12-architecture-defense.md), [3.6](06-completion-rules.md)).
- The result and history screens ([3.13](13-evaluation-and-history-screen.md)).
- Sending the raw XML or an image to the model, and any re-evaluation of a successful result.
- Verifying that the model obeys the data-block and flag rules beyond the prompt.
