# Phase 3.10 — Diagram Workspace (the exam)

## Goal

Make the exam work end to end: from **Start** in the catalog to a submitted diagram. It wires the
layout of [3.8](08-lab-layout-sketch.md), the draw.io embed validated in
[3.9](09-diagram-editor-spike.md) and the attempt use cases of [3.4](04-exercise-service.md).

This is the first time the user can take an exam. Evaluation does not exist yet
([3.11](11-architecture-evaluator.md)), so a submitted diagram waits there.

## Decisions

1. **Start opens a rules dialog.** Clicking **Start** or **Start again** in the catalog
   ([3.7](07-challenge-catalog-ui.md), now enabled) opens a dialog with: the challenge title, the
   time limit, "There is no pause and you cannot resume.", "Leaving discards the exam. It will not
   be graded.", "You can ask the tutor for up to 3 hints.", the current minimum passing score
   ([3.6](06-completion-rules.md)) and "After you submit, the tutor will ask you questions about
   your choices." Buttons: **Cancel** and **Start exam**. The dialog never shows the scenario,
   requirements or rubric.
2. **Order of the start flow.** Rules dialog → editor check → `StartAttempt`. The exam screen
   opens first in a "Preparing the editor" state that shows no statement and no clock. The
   attempt is created, and the clock starts, only after the editor reports ready
   ([3.9](09-diagram-editor-spike.md) decision 6). If the editor does not load: "The editor needs
   an internet connection. Try again." with a **Try again** button and **Back to catalog**, and no
   attempt exists.
3. **Another exam running.** If `StartAttempt` fails with `ErrAttemptInProgress`, a dialog says
   "You have an exam in progress. Starting a new one discards it." with **Cancel** and **OK**.
   **OK** repeats the call with `DiscardInProgress` and continues. **Cancel** returns to the
   catalog and touches nothing. In practice this appears only after an unclean exit that the
   startup cleanup did not catch.
4. **The screen reads the attempt, it does not own the clock.** The countdown receives the
   attempt's `Deadline` and calls `onExpire` at zero ([3.8](08-lab-layout-sketch.md)). The backend
   decides whether a submission is in time.
5. **Finish and evaluate asks for confirmation:** "Finish now? You cannot edit the diagram after
   this." with **Cancel** and **Finish**. It is enabled only when the board has content (at least
   one cell). Confirming calls `SubmitAttempt` with the latest XML.
6. **Time runs out.** At zero, with the board holding content, the screen submits the latest XML
   without asking; the backend accepts it within the 15-second grace period and records
   `TIME_EXPIRED` ([3.4](04-exercise-service.md)). If the board is empty, the screen calls
   `DiscardAttempt` and tells the user "Time is up and the board was empty. The exam was
   discarded." An empty board is not submitted.
7. **Leave discards.** **Leave** opens the confirmation of 3.8; confirming calls `DiscardAttempt`
   and returns to the catalog. The same applies after submitting, until the exam is finished
   (decision 10). Closing the window or a crash leaves the attempt for the startup cleanup
   ([3.4](04-exercise-service.md), [3.5](05-evaluation-engine.md)).
8. **The latest XML lives in memory.** The screen keeps the latest XML the editor sent
   ([3.9](09-diagram-editor-spike.md) decision 4) and saves nothing before submitting. The
   content type is `drawio-xml`; the 2 MiB limit is the backend's. A submission refused as too
   large keeps the exam open, tells the user "The diagram is too large to submit. Remove some
   elements." and lets them try again if time remains.
9. **Connection lost.** When the editor stops responding, the screen shows a banner "Connection
   lost. Reconnecting..." and tries to reconnect; the clock keeps running
   ([3.9](09-diagram-editor-spike.md) decision 5). The banner clears when the editor recovers
   with the diagram intact.
10. **After submitting, the user stays in the lab.** The exam is one sitting: draw, submit, the
    diagram evaluation ([3.11](11-architecture-evaluator.md)), the tutor's questions
    ([3.12](12-architecture-defense.md)) and the result are consecutive stages of the same screen,
    with no "waiting" status and no return to the catalog in between. Until 3.11 exists, the
    screen after submitting shows "Diagram submitted." with a **Back to catalog** button; the
    attempt stays submitted and the challenge shows as **In progress**
    ([3.6](06-completion-rules.md)). Each later spec replaces that end with its stage.
11. **A submission error after time expired** (`ErrSubmissionTooLate`) tells the user "The time
    was over and the exam could not be submitted." and returns to the catalog; the backend has
    discarded the attempt.
12. **One exam at a time, one screen.** The exam is a full-screen state of the Architecture Lab
    section, with no sidebar entry of its own. Navigating away through the sidebar while the exam
    runs asks for the same confirmation as **Leave**.
13. **No data compatibility to keep.**

## Frontend

- `lib/exercise.ts`: wrappers for `StartAttempt`, `SubmitAttempt`, `DiscardAttempt` and
  `GetAttempt`, and the error mapping for `ErrAttemptInProgress`, `ErrSubmissionTooLate` and the
  size error.
- `components/lab/start-exam-dialog.tsx` and `in-progress-dialog.tsx`.
- `components/lab/diagram-editor.tsx`: the draw.io embed wrapper following ADR-006 (iframe,
  `postMessage`, fixed configuration). It exposes `onReady`, `onChange(xml)`, `onConnectionLost`
  and `onConnectionRestored`, and nothing else. The wrapper is the only code that knows draw.io.
- `screens/LabExamScreen.tsx`: the container that runs decisions 2 to 11 using `lab-layout`
  from 3.8 and `useExam`, a hook that owns the phase (preparing, running, submitting) and the
  latest XML.
- `AppShell`: the exam as a state of the `challenge` section and the navigation guard of
  decision 12.
- The catalog's **Start** buttons are enabled and open the dialog; the "Available with the
  diagram editor" tooltip is removed.
- All UI text is in English.

No new backend and no new binding: every call exists from [3.4](04-exercise-service.md). If the
spike shows a gap in the attempt API, this spec and 3.4 are corrected first.

## Tasks

Each slice is TDD (Vitest), keeps the build green and is committed on its own. The editor is
faked in the tests behind the wrapper's interface.

- [ ] `lib/exercise.ts` wrappers and error mapping
- [ ] `diagram-editor` wrapper: ready, change, connection lost and restored events
- [ ] `start-exam-dialog` and the catalog's Start enabled
- [ ] `useExam`: preparing → create the attempt on ready → running; editor failure path with no
      attempt
- [ ] In-progress conflict dialog and `DiscardInProgress` retry
- [ ] Finish confirmation and `SubmitAttempt` with the latest XML; too-large error keeps the exam
- [ ] Expiry: submit with content, discard when empty, too-late message
- [ ] Leave and sidebar navigation guard discarding the attempt
- [ ] Connection-lost banner
- [ ] After submitting: "Diagram submitted." with Back to catalog (replaced by the evaluation stage
      in 3.11)
- [ ] Docs: CHANGELOG, README and `lib/documentation.ts` describe the exam

## Acceptance Criteria

- **Start** opens the rules dialog with the items of decision 1 and never the scenario;
  **Cancel** creates nothing.
- After **Start exam**, the editor is checked before any attempt is created; if it fails, the
  user sees the internet message and no attempt exists.
- With a running exam, starting another shows the discard warning; **OK** replaces the old
  attempt, **Cancel** keeps it.
- The exam screen shows the statement, the editor, the tutor and the rubric from 3.8, with the
  countdown set from the attempt's deadline.
- **Finish and evaluate** is disabled on an empty board and asks for confirmation; confirming
  stores the XML as the submission and shows "Diagram submitted." with **Back to catalog"
- At zero with content, the diagram is submitted automatically; with an empty board, the attempt
  is discarded and the user is told.
- **Leave** and navigating away discard the attempt after confirmation; a submission refused as
  too large or too late shows the messages of decisions 8 and 11.
- A lost connection shows the banner, the clock keeps running, and the banner clears when the
  editor recovers with the diagram intact.
- The attempt is never saved before submitting, and there is no pause or resume.
- Frontend tests, coverage ≥ 80%, lint, typecheck, `npx prettier --check .` and Stryker on the
  changed files pass.

## Out of scope

- Hints in the tutor panel ([3.10.1](10-01-tutor-hints.md)).
- Evaluating the diagram, the defense and the result screens
  ([3.11](11-architecture-evaluator.md), [3.12](12-architecture-defense.md),
  [3.13](13-evaluation-and-history-screen.md)).
- Opening the exam from the chat ([3.15](15-open-lab-from-chat.md)).
- Drafts, autosave to disk, pause, resume, undo of a submission.
- Self-hosting the editor and any image export.
