import { useImperativeHandle, type Ref } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { DiagramEditorHandle } from '@/components/lab/diagram-editor'
import {
  answerDefenseQuestion,
  discardAttempt,
  evaluateAttempt,
  evaluateDefense,
  listHints,
  nextDefenseQuestion,
  requestHint,
  startAttempt,
  submitAttempt,
  type Attempt,
  type Evaluation,
  type Outcome,
} from '@/lib/exercise'
import LabExamScreen from './LabExamScreen'

vi.mock('@/lib/exercise', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/exercise')>()
  return {
    ...original,
    startAttempt: vi.fn(),
    submitAttempt: vi.fn(),
    discardAttempt: vi.fn(),
    evaluateAttempt: vi.fn(),
    nextDefenseQuestion: vi.fn(),
    answerDefenseQuestion: vi.fn(),
    evaluateDefense: vi.fn(),
    requestHint: vi.fn(),
    listHints: vi.fn(),
  }
})

interface EditorProps {
  ref?: Ref<DiagramEditorHandle>
  onReady: () => void
  onChange: (xml: string) => void
  onConnectionLost: () => void
  onConnectionRestored: () => void
}

// The real editor is covered by its own tests; this stand-in exposes the
// callbacks the screen hands it and answers getXml with xmlOnBoard.
let editor: EditorProps
let xmlOnBoard = ''

vi.mock('@/components/lab/diagram-editor', () => ({
  DiagramEditor: (props: EditorProps) => {
    editor = props
    useImperativeHandle(props.ref, () => ({ getXml: () => Promise.resolve(xmlOnBoard) }))
    return <div data-testid="editor" />
  },
}))

const FULL_XML =
  '<mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/><mxCell id="2" vertex="1" parent="1"/></root></mxGraphModel>'
const EMPTY_XML =
  '<mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/></root></mxGraphModel>'

const NOW = new Date('2026-11-06T12:00:00Z').getTime()

const attempt: Attempt = {
  id: 'a1',
  challengeId: 'c1',
  title: 'Cache and invalidation',
  topic: 'caching',
  deadline: new Date(NOW + 44 * 60 * 1000).toISOString(),
  statement: {
    scenario: 'A catalog service serves millions of reads.',
    functionalRequirements: ['Browse products'],
    nonFunctionalRequirements: [],
    constraints: [],
    objectives: [],
  },
  criteria: [
    {
      id: 'k1',
      name: 'Consistency',
      description: 'Reads stay correct.',
      weight: 20,
      mandatory: true,
    },
  ],
}

const evaluation: Evaluation = {
  score: 72,
  summary: 'A solid start.',
  strengths: [],
  gaps: [],
  recommendations: [],
  results: [
    {
      key: 'k1',
      name: 'Consistency',
      level: 3,
      justification: 'Invalidation is drawn.',
      evidence: '',
      insufficientEvidence: false,
    },
  ],
}

const outcome: Outcome = {
  result: 'FAILED',
  passScore: 80,
  diagramScore: 85,
  understandingScore: 60,
  reasons: [{ kind: 'UNDERSTANDING_SCORE', evaluation: 'UNDERSTANDING', criterionName: '' }],
}

const challenge = {
  id: 'c1',
  title: 'Cache and invalidation',
  difficulty: 'INTERMEDIATE',
  timeLimitMinutes: 45,
}

function coded(code: string): Error {
  return new Error(JSON.stringify({ code, message: code }))
}

function renderScreen(props: { leaveRequested?: boolean } = {}) {
  const handlers = { onExit: vi.fn(), onLeaveCancelled: vi.fn() }
  const view = render(
    <LabExamScreen
      challenge={challenge}
      leaveRequested={false}
      now={() => Date.now()}
      {...handlers}
      {...props}
    />,
  )
  return { ...handlers, ...view }
}

async function startRunning(props: { leaveRequested?: boolean } = {}) {
  const view = renderScreen(props)
  await act(async () => editor.onReady())
  await screen.findByRole('timer')
  return view
}

describe('LabExamScreen', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers({ shouldAdvanceTime: true })
    vi.setSystemTime(NOW)
    xmlOnBoard = FULL_XML
    vi.mocked(startAttempt).mockResolvedValue(attempt)
    vi.mocked(submitAttempt).mockResolvedValue()
    vi.mocked(discardAttempt).mockResolvedValue()
    vi.mocked(evaluateAttempt).mockResolvedValue(evaluation)
    vi.mocked(nextDefenseQuestion).mockResolvedValue({
      position: 1,
      text: 'Why a cache?',
      total: 2,
    })
    vi.mocked(answerDefenseQuestion).mockResolvedValue({ total: 2, complete: false })
    vi.mocked(evaluateDefense).mockResolvedValue(outcome)
    vi.mocked(listHints).mockResolvedValue([])
  })
  afterEach(() => vi.useRealTimers())

  it('prepares the editor first: no statement, no clock, no attempt', () => {
    renderScreen()

    expect(screen.getByText('Preparing the editor')).toBeInTheDocument()
    expect(screen.queryByRole('timer')).not.toBeInTheDocument()
    expect(screen.queryByText(/A catalog service/)).not.toBeInTheDocument()
    expect(startAttempt).not.toHaveBeenCalled()
  })

  it('creates the attempt once the editor is ready and shows the exam from its deadline', async () => {
    await startRunning()

    expect(startAttempt).toHaveBeenCalledExactlyOnceWith('c1', false)
    expect(screen.getByText('A catalog service serves millions of reads.')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Cache and invalidation' })).toBeInTheDocument()
    expect(screen.getByText('Intermediate')).toBeInTheDocument()
    expect(screen.getByText('Consistency')).toBeInTheDocument()
    expect(screen.getByRole('timer')).toHaveTextContent('44:00')
    expect(screen.getByRole('heading', { name: 'Tutor Athena' })).toBeInTheDocument()
    expect(screen.getByTestId('editor')).toBeInTheDocument()
  })

  it('offers to try again when the editor does not load, and no attempt exists', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    renderScreen()

    await act(async () => vi.advanceTimersByTime(15000))

    expect(
      screen.getByText('The editor needs an internet connection. Try again.'),
    ).toBeInTheDocument()
    expect(startAttempt).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Try again' }))
    expect(screen.getByText('Preparing the editor')).toBeInTheDocument()
  })

  it('goes back to the catalog from the editor failure', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const { onExit } = renderScreen()
    await act(async () => vi.advanceTimersByTime(15000))

    await user.click(screen.getByRole('button', { name: 'Back to catalog' }))

    expect(onExit).toHaveBeenCalledExactlyOnceWith()
    expect(discardAttempt).not.toHaveBeenCalled()
  })

  it('warns about the running exam and replaces it on OK', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    vi.mocked(startAttempt).mockRejectedValueOnce(coded('attempt_in_progress'))
    renderScreen()
    await act(async () => editor.onReady())

    expect(
      await screen.findByText('You have an exam in progress. Starting a new one discards it.'),
    ).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'OK' }))

    expect(startAttempt).toHaveBeenLastCalledWith('c1', true)
    expect(await screen.findByRole('timer')).toBeInTheDocument()
  })

  it('returns to the catalog and touches nothing when the warning is cancelled', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    vi.mocked(startAttempt).mockRejectedValueOnce(coded('attempt_in_progress'))
    const { onExit } = renderScreen()
    await act(async () => editor.onReady())

    await user.click(await screen.findByRole('button', { name: 'Cancel' }))

    expect(onExit).toHaveBeenCalledExactlyOnceWith()
    expect(startAttempt).toHaveBeenCalledOnce()
    expect(discardAttempt).not.toHaveBeenCalled()
  })

  it('says why the exam could not start and goes back', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    vi.mocked(startAttempt).mockRejectedValue(coded('challenge_has_no_rubric'))
    const { onExit } = renderScreen()
    await act(async () => editor.onReady())

    expect(await screen.findByText('Something went wrong. Please try again.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Back to catalog' }))
    expect(onExit).toHaveBeenCalledOnce()
  })

  it('disables Finish and evaluate on an empty board and enables it with content', async () => {
    await startRunning()
    const finish = screen.getByRole('button', { name: 'Finish and evaluate' })
    expect(finish).toBeDisabled()

    act(() => editor.onChange(FULL_XML))
    expect(finish).toBeEnabled()

    act(() => editor.onChange(EMPTY_XML))
    expect(finish).toBeDisabled()
  })

  it('asks before finishing and submits the diagram when confirmed', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await startRunning()
    act(() => editor.onChange(FULL_XML))

    await user.click(screen.getByRole('button', { name: 'Finish and evaluate' }))
    expect(
      screen.getByText('Finish now? You cannot edit the diagram after this.'),
    ).toBeInTheDocument()
    expect(submitAttempt).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Finish' }))

    expect(submitAttempt).toHaveBeenCalledExactlyOnceWith('a1', FULL_XML)
    expect(await screen.findByRole('heading', { name: 'Question 1 of 2' })).toBeInTheDocument()
    expect(evaluateAttempt).toHaveBeenCalledExactlyOnceWith('a1')
    expect(screen.queryByRole('timer')).not.toBeInTheDocument()
  })

  it('shows progress while the diagram is evaluated and stays in the lab', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    vi.mocked(evaluateAttempt).mockReturnValue(new Promise(() => {}))
    const { onExit } = await startRunning()
    act(() => editor.onChange(FULL_XML))
    await user.click(screen.getByRole('button', { name: 'Finish and evaluate' }))
    await user.click(screen.getByRole('button', { name: 'Finish' }))

    expect(await screen.findByText('Evaluating your diagram...')).toBeInTheDocument()
    expect(screen.getByTestId('editor')).toBeInTheDocument()
    expect(onExit).not.toHaveBeenCalled()
  })

  it('says why the evaluation failed and evaluates again on request', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    vi.mocked(evaluateAttempt).mockRejectedValueOnce(coded('invalid_evaluation'))
    await startRunning()
    act(() => editor.onChange(FULL_XML))
    await user.click(screen.getByRole('button', { name: 'Finish and evaluate' }))
    await user.click(screen.getByRole('button', { name: 'Finish' }))

    expect(
      await screen.findByText('The evaluator returned an invalid evaluation.'),
    ).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Try evaluating again' }))

    expect(await screen.findByRole('heading', { name: 'Question 1 of 2' })).toBeInTheDocument()
    expect(evaluateAttempt).toHaveBeenCalledTimes(2)
    expect(submitAttempt).toHaveBeenCalledOnce()
  })

  it('asks before leaving during the evaluation and discards the attempt on confirm', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    vi.mocked(evaluateAttempt).mockReturnValue(new Promise(() => {}))
    const onLeaveCancelled = vi.fn()
    const view = await startRunning()
    act(() => editor.onChange(FULL_XML))
    await user.click(screen.getByRole('button', { name: 'Finish and evaluate' }))
    await user.click(screen.getByRole('button', { name: 'Finish' }))
    await screen.findByText('Evaluating your diagram...')

    view.rerender(
      <LabExamScreen
        challenge={challenge}
        leaveRequested
        onLeaveCancelled={onLeaveCancelled}
        onExit={view.onExit}
      />,
    )
    expect(
      screen.getByText('Leaving discards this exam. It will not be graded.'),
    ).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Leave' }))

    expect(discardAttempt).toHaveBeenCalledExactlyOnceWith('a1')
    expect(view.onExit).toHaveBeenCalledOnce()
  })

  it('keeps the exam when the finish question is cancelled', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await startRunning()
    act(() => editor.onChange(FULL_XML))
    await user.click(screen.getByRole('button', { name: 'Finish and evaluate' }))

    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(submitAttempt).not.toHaveBeenCalled()
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(screen.getByRole('timer')).toBeInTheDocument()
  })

  it('discards the submitted attempt and goes back from Back to catalog after a failed evaluation', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    vi.mocked(evaluateAttempt).mockRejectedValue(coded('invalid_evaluation'))
    const { onExit } = await startRunning()
    act(() => editor.onChange(FULL_XML))
    await user.click(screen.getByRole('button', { name: 'Finish and evaluate' }))
    await user.click(screen.getByRole('button', { name: 'Finish' }))
    await screen.findByText('The evaluator returned an invalid evaluation.')

    await user.click(screen.getByRole('button', { name: 'Back to catalog' }))

    expect(discardAttempt).toHaveBeenCalledExactlyOnceWith('a1')
    expect(onExit).toHaveBeenCalledExactlyOnceWith()
  })

  it('keeps the exam open and explains when the diagram is too large', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    vi.mocked(submitAttempt).mockRejectedValue(coded('submission_too_large'))
    await startRunning()
    act(() => editor.onChange(FULL_XML))
    await user.click(screen.getByRole('button', { name: 'Finish and evaluate' }))
    await user.click(screen.getByRole('button', { name: 'Finish' }))

    expect(
      await screen.findByText('The diagram is too large to submit. Remove some elements.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('timer')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Finish and evaluate' })).toBeEnabled()
  })

  it('returns to the catalog with the too-late notice', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    vi.mocked(submitAttempt).mockRejectedValue(coded('submission_too_late'))
    const { onExit } = await startRunning()
    act(() => editor.onChange(FULL_XML))
    await user.click(screen.getByRole('button', { name: 'Finish and evaluate' }))
    await user.click(screen.getByRole('button', { name: 'Finish' }))

    await screen.findByRole('timer')
    expect(onExit).toHaveBeenCalledExactlyOnceWith(
      'The time was over and the exam could not be submitted.',
    )
  })

  it('blocks editing while the diagram is being submitted', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    vi.mocked(submitAttempt).mockReturnValue(new Promise(() => {}))
    await startRunning()
    act(() => editor.onChange(FULL_XML))
    await user.click(screen.getByRole('button', { name: 'Finish and evaluate' }))
    await user.click(screen.getByRole('button', { name: 'Finish' }))

    expect(await screen.findByTestId('board-lock')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Finish and evaluate' })).toBeDisabled()
  })

  it('submits by itself at zero when the board has content, closing the finish question', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await startRunning()
    act(() => editor.onChange(FULL_XML))
    await user.click(screen.getByRole('button', { name: 'Finish and evaluate' }))

    await act(async () => vi.advanceTimersByTime(44 * 60 * 1000))

    expect(submitAttempt).toHaveBeenCalledExactlyOnceWith('a1', FULL_XML)
    expect(await screen.findByRole('heading', { name: 'Question 1 of 2' })).toBeInTheDocument()
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('discards an empty board at zero and tells the user', async () => {
    xmlOnBoard = EMPTY_XML
    const { onExit } = await startRunning()

    await act(async () => vi.advanceTimersByTime(44 * 60 * 1000))

    expect(submitAttempt).not.toHaveBeenCalled()
    expect(discardAttempt).toHaveBeenCalledExactlyOnceWith('a1')
    expect(onExit).toHaveBeenCalledExactlyOnceWith(
      'Time is up and the board was empty. The exam was discarded.',
    )
  })

  it('discards the attempt and goes back when the user confirms Leave', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const { onExit } = await startRunning()

    await user.click(screen.getByRole('button', { name: 'Leave' }))
    expect(discardAttempt).not.toHaveBeenCalled()
    await user.click(screen.getAllByRole('button', { name: 'Leave' }).at(-1)!)

    expect(discardAttempt).toHaveBeenCalledExactlyOnceWith('a1')
    expect(onExit).toHaveBeenCalledExactlyOnceWith()
  })

  it('shows a banner while the connection is lost and clears it on recovery', async () => {
    await startRunning()
    expect(screen.queryByText('Connection lost. Reconnecting...')).not.toBeInTheDocument()

    act(() => editor.onConnectionLost())
    expect(screen.getByRole('status')).toHaveTextContent('Connection lost. Reconnecting...')
    expect(screen.getByRole('timer')).toBeInTheDocument()

    act(() => editor.onConnectionRestored())
    expect(screen.queryByText('Connection lost. Reconnecting...')).not.toBeInTheDocument()
  })

  it('asks the same Leave question when navigation is requested during the exam', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const { onExit, rerender, onLeaveCancelled } = await startRunning()

    rerender(
      <LabExamScreen
        challenge={challenge}
        leaveRequested
        now={() => Date.now()}
        onExit={onExit}
        onLeaveCancelled={onLeaveCancelled}
      />,
    )
    expect(
      screen.getByText('Leaving discards this exam. It will not be graded.'),
    ).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(onLeaveCancelled).toHaveBeenCalledOnce()
    expect(onExit).not.toHaveBeenCalled()
    expect(discardAttempt).not.toHaveBeenCalled()
  })

  it('discards the attempt when the requested navigation is confirmed', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const { onExit, rerender, onLeaveCancelled } = await startRunning()
    rerender(
      <LabExamScreen
        challenge={challenge}
        leaveRequested
        now={() => Date.now()}
        onExit={onExit}
        onLeaveCancelled={onLeaveCancelled}
      />,
    )

    await user.click(screen.getByRole('button', { name: 'Leave' }))

    expect(discardAttempt).toHaveBeenCalledExactlyOnceWith('a1')
    expect(onExit).toHaveBeenCalledExactlyOnceWith()
  })

  it('lets navigation through at once when no attempt exists yet', () => {
    const { onExit, rerender, onLeaveCancelled } = renderScreen()

    rerender(
      <LabExamScreen
        challenge={challenge}
        leaveRequested
        now={() => Date.now()}
        onExit={onExit}
        onLeaveCancelled={onLeaveCancelled}
      />,
    )

    expect(onExit).toHaveBeenCalledExactlyOnceWith()
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })
  it('asks the tutor with the current board and shows the hint in the panel', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    vi.mocked(requestHint).mockResolvedValue({ id: 'h1', text: 'What if the cache is down?' })
    await startRunning()
    expect(screen.getByText('Hints left: 3 of 3')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Ask for a hint' }))

    expect(requestHint).toHaveBeenCalledExactlyOnceWith('a1', FULL_XML)
    expect(await screen.findByText('What if the cache is down?')).toBeInTheDocument()
    expect(screen.getByText('Hints left: 2 of 3')).toBeInTheDocument()
  })

  it('shows the hints given earlier for the attempt', async () => {
    vi.mocked(listHints).mockResolvedValue([{ id: 'h1', text: 'Think about reads.' }])

    await startRunning()

    expect(await screen.findByText('Think about reads.')).toBeInTheDocument()
    expect(listHints).toHaveBeenCalledExactlyOnceWith('a1')
    expect(screen.getByText('Hints left: 2 of 3')).toBeInTheDocument()
  })

  it('says a hint could not be fetched, without spending one', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    vi.mocked(requestHint).mockRejectedValue(coded('internal'))
    await startRunning()

    await user.click(screen.getByRole('button', { name: 'Ask for a hint' }))

    expect(await screen.findByText('Could not get a hint. Try again.')).toBeInTheDocument()
    expect(screen.getByText('Hints left: 3 of 3')).toBeInTheDocument()
  })

  it('sends an empty board when the editor is gone', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    vi.mocked(requestHint).mockResolvedValue({ id: 'h1', text: 'Where would you start?' })
    xmlOnBoard = ''
    await startRunning()

    await user.click(screen.getByRole('button', { name: 'Ask for a hint' }))

    expect(requestHint).toHaveBeenCalledExactlyOnceWith('a1', '')
  })

  describe('the defense', () => {
    // Gets to the first question.
    async function defending(props: { leaveRequested?: boolean } = {}) {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      const view = await startRunning(props)
      act(() => editor.onChange(FULL_XML))
      await user.click(screen.getByRole('button', { name: 'Finish and evaluate' }))
      await user.click(screen.getByRole('button', { name: 'Finish' }))
      await screen.findByRole('heading', { name: 'Question 1 of 2' })
      return { user, ...view }
    }

    it('starts by itself with the statement, the board and the rubric still visible and read-only', async () => {
      await defending()

      expect(screen.getByText('Why a cache?')).toBeInTheDocument()
      expect(screen.getByText('A catalog service serves millions of reads.')).toBeInTheDocument()
      expect(screen.getByTestId('editor')).toBeInTheDocument()
      expect(screen.getByTestId('board-lock')).toBeInTheDocument()
      expect(screen.getByText('Consistency')).toBeInTheDocument()
    })

    it('has no clock, no hint button and no finish button, and shows no score', async () => {
      await defending()

      expect(screen.queryByRole('timer')).not.toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Ask for a hint' })).not.toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Finish and evaluate' })).not.toBeInTheDocument()
      expect(screen.queryByText('72')).not.toBeInTheDocument()
      expect(screen.queryByText('Invalidation is drawn.')).not.toBeInTheDocument()
    })

    it('goes from the last answer to the outcome with Back to catalog', async () => {
      const { user, onExit } = await defending()
      vi.mocked(nextDefenseQuestion).mockResolvedValue({
        position: 2,
        text: 'What if it fails?',
        total: 2,
      })

      await user.type(screen.getByRole('textbox', { name: 'Your answer' }), 'To cut reads.')
      await user.click(screen.getByRole('button', { name: 'Send answer' }))
      expect(await screen.findByRole('heading', { name: 'Question 2 of 2' })).toBeInTheDocument()
      vi.mocked(answerDefenseQuestion).mockResolvedValue({ total: 2, complete: true })
      await user.click(screen.getByRole('button', { name: "I don't know" }))

      expect(await screen.findByRole('heading', { name: 'Failed' })).toBeInTheDocument()
      expect(answerDefenseQuestion).toHaveBeenNthCalledWith(1, 'a1', 'To cut reads.')
      expect(answerDefenseQuestion).toHaveBeenNthCalledWith(2, 'a1', "I don't know")
      expect(evaluateDefense).toHaveBeenCalledExactlyOnceWith('a1')
      expect(
        screen.getByText('The understanding score is below the minimum of 80.'),
      ).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: 'Back to catalog' }))
      expect(onExit).toHaveBeenCalledOnce()
      expect(discardAttempt).not.toHaveBeenCalled()
    })

    it('says why a question failed and asks again on request', async () => {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      vi.mocked(nextDefenseQuestion).mockRejectedValueOnce(coded('internal'))
      await startRunning()
      act(() => editor.onChange(FULL_XML))
      await user.click(screen.getByRole('button', { name: 'Finish and evaluate' }))
      await user.click(screen.getByRole('button', { name: 'Finish' }))

      expect(
        await screen.findByText('The tutor could not continue the defense.'),
      ).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: 'Try again' }))

      expect(await screen.findByRole('heading', { name: 'Question 1 of 2' })).toBeInTheDocument()
      expect(evaluateAttempt).toHaveBeenCalledOnce()
    })

    it('keeps the answers and evaluates again when the final evaluation failed', async () => {
      const { user } = await defending()
      vi.mocked(answerDefenseQuestion).mockResolvedValue({ total: 2, complete: true })
      vi.mocked(evaluateDefense).mockRejectedValueOnce(coded('invalid_evaluation'))

      await user.click(screen.getByRole('button', { name: "I don't know" }))
      expect(
        await screen.findByText('The tutor returned an invalid evaluation.'),
      ).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: 'Try again' }))

      expect(await screen.findByRole('heading', { name: 'Failed' })).toBeInTheDocument()
      expect(answerDefenseQuestion).toHaveBeenCalledOnce()
      expect(evaluateDefense).toHaveBeenCalledTimes(2)
    })

    it('shows a refused answer once, on the question, and not over the board', async () => {
      const { user } = await defending()
      vi.mocked(answerDefenseQuestion).mockRejectedValueOnce(coded('invalid_answer'))

      await user.type(screen.getByRole('textbox', { name: 'Your answer' }), 'x')
      await user.click(screen.getByRole('button', { name: 'Send answer' }))

      expect(await screen.findAllByRole('alert')).toHaveLength(1)
      expect(
        screen.getByText('The answer must be between 1 and 4,000 characters.'),
      ).toBeInTheDocument()
    })

    it('asks before leaving during the defense and discards the attempt on confirm', async () => {
      const { user, rerender, onExit } = await defending()

      rerender(
        <LabExamScreen
          challenge={challenge}
          leaveRequested
          onLeaveCancelled={vi.fn()}
          onExit={onExit}
        />,
      )
      expect(
        screen.getByText('Leaving discards this exam. It will not be graded.'),
      ).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: 'Leave' }))

      expect(discardAttempt).toHaveBeenCalledExactlyOnceWith('a1')
      expect(onExit).toHaveBeenCalledOnce()
    })

    it('shows progress while a question is written, while an answer is sent and while the answers are judged', async () => {
      const { user } = await defending()
      vi.mocked(answerDefenseQuestion).mockReturnValue(new Promise(() => {}))
      await user.type(screen.getByRole('textbox', { name: 'Your answer' }), 'x')
      await user.click(screen.getByRole('button', { name: 'Send answer' }))
      expect(await screen.findByRole('button', { name: 'Sending...' })).toBeDisabled()
    })

    it('shows progress while the next question is written', async () => {
      const { user } = await defending()
      vi.mocked(nextDefenseQuestion).mockReturnValue(new Promise(() => {}))
      await user.click(screen.getByRole('button', { name: "I don't know" }))
      expect(
        await screen.findByText('The tutor is preparing the next question...'),
      ).toBeInTheDocument()
    })

    it('shows progress while the answers are judged', async () => {
      const { user } = await defending()
      vi.mocked(answerDefenseQuestion).mockResolvedValue({ total: 2, complete: true })
      vi.mocked(evaluateDefense).mockReturnValue(new Promise(() => {}))
      await user.click(screen.getByRole('button', { name: "I don't know" }))
      expect(await screen.findByText('Evaluating your answers...')).toBeInTheDocument()
    })
  })
})
