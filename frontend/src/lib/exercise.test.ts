import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  CancelChallengeGeneration,
  DiscardAttempt,
  EvaluateAttempt,
  EvaluateDefense,
  GenerateChallenge,
  GetAttempt,
  GetEvaluation,
  ListChallengeCatalog,
  ListChallengeTopics,
  ListHints,
  ListSessionsForGeneration,
  NextDefenseQuestion,
  AnswerDefenseQuestion,
  RequestHint,
  StartAttempt,
  SubmitAttempt,
} from '../../wailsjs/go/desktop/App'
import { desktop } from '../../wailsjs/go/models'
import {
  DIFFICULTY_LABELS,
  STATUS_LABELS,
  ATTEMPT_IN_PROGRESS,
  DIAGRAM_CONTENT_TYPE,
  SUBMISSION_TOO_LARGE,
  SUBMISSION_TOO_LATE,
  DONT_KNOW_ANSWER,
  MAX_ANSWER_LENGTH,
  answerDefenseQuestion,
  cancelChallengeGeneration,
  defenseErrorMessage,
  discardAttempt,
  evaluateDefense,
  nextDefenseQuestion,
  evaluateAttempt,
  evaluationErrorMessage,
  exerciseErrorCode,
  difficultyForExperience,
  exerciseErrorMessage,
  formatSessionDate,
  generateChallenge,
  getAttempt,
  getEvaluation,
  listChallengeCatalog,
  listChallengeTopics,
  listHints,
  listSessionsForGeneration,
  requestHint,
  startAttempt,
  submitAttempt,
} from './exercise'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  CancelChallengeGeneration: vi.fn(),
  GenerateChallenge: vi.fn(),
  ListChallengeCatalog: vi.fn(),
  ListChallengeTopics: vi.fn(),
  ListSessionsForGeneration: vi.fn(),
  StartAttempt: vi.fn(),
  SubmitAttempt: vi.fn(),
  DiscardAttempt: vi.fn(),
  GetAttempt: vi.fn(),
  EvaluateAttempt: vi.fn(),
  EvaluateDefense: vi.fn(),
  NextDefenseQuestion: vi.fn(),
  AnswerDefenseQuestion: vi.fn(),
  GetEvaluation: vi.fn(),
  RequestHint: vi.fn(),
  ListHints: vi.fn(),
}))

const item = {
  id: 'c1',
  title: 'Cache',
  topic: 'caching',
  difficulty: 'INTERMEDIATE',
  timeLimitMinutes: 45,
  status: 'COMPLETED',
  attempts: 2,
  bestDiagramScore: 86,
}

describe('exercise bindings', () => {
  beforeEach(() => vi.clearAllMocks())

  it('lists the catalog with the filter as the backend expects it', async () => {
    vi.mocked(ListChallengeCatalog).mockResolvedValueOnce(
      desktop.ChallengeCatalogDTO.createFrom({
        total: 3,
        completed: 1,
        inProgress: 1,
        items: [item],
      }),
    )

    const catalog = await listChallengeCatalog({
      query: 'cache',
      status: 'COMPLETED',
      difficulty: 'INTERMEDIATE',
      topic: 'caching',
    })

    expect(ListChallengeCatalog).toHaveBeenCalledWith({
      query: 'cache',
      status: 'COMPLETED',
      difficulty: 'INTERMEDIATE',
      topic: 'caching',
    })
    expect(catalog).toEqual({ total: 3, completed: 1, inProgress: 1, items: [item] })
  })

  it('sends empty strings for filters that are not set', async () => {
    vi.mocked(ListChallengeCatalog).mockResolvedValueOnce(
      desktop.ChallengeCatalogDTO.createFrom({ total: 0, completed: 0, inProgress: 0, items: [] }),
    )

    await listChallengeCatalog({})

    expect(ListChallengeCatalog).toHaveBeenCalledWith({
      query: '',
      status: '',
      difficulty: '',
      topic: '',
    })
  })

  it('lists the topics', async () => {
    vi.mocked(ListChallengeTopics).mockResolvedValueOnce(['caching', 'queues'])

    await expect(listChallengeTopics()).resolves.toEqual(['caching', 'queues'])
  })

  it('lists the study sessions a challenge can be generated from', async () => {
    vi.mocked(ListSessionsForGeneration).mockResolvedValueOnce([
      { id: 's1', topic: 'Caching', folderName: 'Backend', startedAt: '2026-11-02T09:30:00Z' },
    ])

    await expect(listSessionsForGeneration()).resolves.toEqual([
      { id: 's1', topic: 'Caching', folderName: 'Backend', startedAt: '2026-11-02T09:30:00Z' },
    ])
  })

  it('generates a challenge for a session and difficulty', async () => {
    vi.mocked(GenerateChallenge).mockResolvedValueOnce({
      id: 'c9',
      title: 'New',
      topic: 'caching',
      difficulty: 'BEGINNER',
      timeLimitMinutes: 30,
    })

    const created = await generateChallenge('s1', 'BEGINNER')

    expect(GenerateChallenge).toHaveBeenCalledWith('s1', 'BEGINNER')
    expect(created).toEqual({
      id: 'c9',
      title: 'New',
      topic: 'caching',
      difficulty: 'BEGINNER',
      timeLimitMinutes: 30,
    })
  })

  it('cancels the generation in flight', async () => {
    vi.mocked(CancelChallengeGeneration).mockResolvedValueOnce()

    await cancelChallengeGeneration()

    expect(CancelChallengeGeneration).toHaveBeenCalledOnce()
  })
})

describe('labels', () => {
  it('names every status the way the catalog shows it', () => {
    expect(STATUS_LABELS).toEqual({
      NOT_STARTED: 'Not started',
      IN_PROGRESS: 'In progress',
      NEEDS_IMPROVEMENT: 'Needs improvement',
      COMPLETED: 'Completed',
    })
  })

  it('names every difficulty', () => {
    expect(DIFFICULTY_LABELS).toEqual({
      BEGINNER: 'Beginner',
      INTERMEDIATE: 'Intermediate',
      ADVANCED: 'Advanced',
    })
  })
})

describe('exerciseErrorCode', () => {
  it('reads the code from the JSON the backend rejects with, as a string or an Error', () => {
    const text = JSON.stringify({ code: 'invalid_generation', message: 'x' })

    expect(exerciseErrorCode(text)).toBe('invalid_generation')
    expect(exerciseErrorCode(new Error(text))).toBe('invalid_generation')
  })

  it('is empty for anything that is not a coded error', () => {
    expect(exerciseErrorCode('plain text')).toBe('')
    expect(exerciseErrorCode(new Error('{"message":"no code"}'))).toBe('')
    expect(exerciseErrorCode('{"code": 7}')).toBe('')
    expect(exerciseErrorCode(undefined)).toBe('')
    expect(exerciseErrorCode('7')).toBe('')
    expect(exerciseErrorCode('null')).toBe('')
    expect(exerciseErrorCode(7)).toBe('')
  })
})

describe('exerciseErrorMessage', () => {
  it('explains an invalid model output and invites a retry', () => {
    expect(exerciseErrorMessage(JSON.stringify({ code: 'invalid_generation' }))).toBe(
      'The generated challenge was not valid. Try again.',
    )
  })

  it('explains a session that no longer exists', () => {
    expect(exerciseErrorMessage(JSON.stringify({ code: 'source_session_not_found' }))).toBe(
      'That study session no longer exists. Pick another one.',
    )
  })

  it('explains a generation that is already running', () => {
    expect(exerciseErrorMessage(JSON.stringify({ code: 'generation_in_progress' }))).toBe(
      'A challenge is already being generated.',
    )
  })

  it('falls back to a generic message for anything else', () => {
    expect(exerciseErrorMessage(JSON.stringify({ code: 'internal' }))).toBe(
      'Something went wrong. Please try again.',
    )
    expect(exerciseErrorMessage(undefined)).toBe('Something went wrong. Please try again.')
  })
})

describe('difficultyForExperience', () => {
  it('preselects the difficulty matching the profile experience level', () => {
    expect(difficultyForExperience('beginner')).toBe('BEGINNER')
    expect(difficultyForExperience('intermediate')).toBe('INTERMEDIATE')
    expect(difficultyForExperience('advanced')).toBe('ADVANCED')
  })

  it('falls back to intermediate for an unknown or missing level', () => {
    expect(difficultyForExperience('guru')).toBe('INTERMEDIATE')
    expect(difficultyForExperience(undefined)).toBe('INTERMEDIATE')
  })
})

describe('formatSessionDate', () => {
  it('writes the day, month and year of the start time', () => {
    expect(formatSessionDate('2026-11-02T12:00:00Z')).toBe('Nov 2, 2026')
  })
})

const attemptDto = desktop.AttemptDTO.createFrom({
  id: 'a1',
  challengeId: 'c1',
  challengeTitle: 'Cache',
  challengeTopic: 'caching',
  state: 'IN_PROGRESS',
  deadline: '2026-11-01T10:45:00Z',
  statement: {
    scenario: 'A link service.',
    functionalRequirements: ['Shorten a URL'],
    nonFunctionalRequirements: ['p99 under 100 ms'],
    constraints: ['No new database'],
    objectives: ['Cut read load'],
  },
  criteria: [
    {
      key: 'architecture',
      name: 'Overall architecture',
      description: 'Coherent.',
      weight: 20,
      mandatory: true,
      levels: ['0', '1', '2', '3', '4'],
    },
  ],
})

const attempt = {
  id: 'a1',
  challengeId: 'c1',
  title: 'Cache',
  topic: 'caching',
  deadline: '2026-11-01T10:45:00Z',
  statement: {
    scenario: 'A link service.',
    functionalRequirements: ['Shorten a URL'],
    nonFunctionalRequirements: ['p99 under 100 ms'],
    constraints: ['No new database'],
    objectives: ['Cut read load'],
  },
  criteria: [
    {
      id: 'architecture',
      name: 'Overall architecture',
      description: 'Coherent.',
      weight: 20,
      mandatory: true,
    },
  ],
}

describe('attempt bindings', () => {
  beforeEach(() => vi.clearAllMocks())

  it('starts a standalone attempt and maps it to what the exam screen needs', async () => {
    vi.mocked(StartAttempt).mockResolvedValueOnce(attemptDto)

    const started = await startAttempt('c1', false)

    expect(StartAttempt).toHaveBeenCalledWith('c1', '', false)
    expect(started).toEqual(attempt)
  })

  it('asks the backend to discard the running attempt only when told to', async () => {
    vi.mocked(StartAttempt).mockResolvedValueOnce(attemptDto)

    await startAttempt('c1', true)

    expect(StartAttempt).toHaveBeenCalledWith('c1', '', true)
  })

  it('submits the diagram XML as drawio-xml', async () => {
    vi.mocked(SubmitAttempt).mockResolvedValueOnce(attemptDto)

    await submitAttempt('a1', '<mxGraphModel/>')

    expect(DIAGRAM_CONTENT_TYPE).toBe('drawio-xml')
    expect(SubmitAttempt).toHaveBeenCalledWith('a1', 'drawio-xml', '<mxGraphModel/>')
  })

  it('discards an attempt', async () => {
    vi.mocked(DiscardAttempt).mockResolvedValueOnce()

    await discardAttempt('a1')

    expect(DiscardAttempt).toHaveBeenCalledWith('a1')
  })

  it('reads an attempt', async () => {
    vi.mocked(GetAttempt).mockResolvedValueOnce(attemptDto)

    expect(await getAttempt('a1')).toEqual(attempt)
    expect(GetAttempt).toHaveBeenCalledWith('a1')
  })

  it('names the codes the exam screen switches on', () => {
    expect(ATTEMPT_IN_PROGRESS).toBe('attempt_in_progress')
    expect(SUBMISSION_TOO_LATE).toBe('submission_too_late')
    expect(SUBMISSION_TOO_LARGE).toBe('submission_too_large')
  })

  it('explains a diagram that is too large', () => {
    expect(exerciseErrorMessage(JSON.stringify({ code: 'submission_too_large' }))).toBe(
      'The diagram is too large to submit. Remove some elements.',
    )
  })

  it('explains a submission that arrived after the time was over', () => {
    expect(exerciseErrorMessage(JSON.stringify({ code: 'submission_too_late' }))).toBe(
      'The time was over and the exam could not be submitted.',
    )
  })
})

describe('hint bindings', () => {
  beforeEach(() => vi.clearAllMocks())

  const hintDto = (id: string, content: string) =>
    new desktop.HintDTO({
      id,
      attemptId: 'a1',
      position: 1,
      content,
      createdAt: '2026-11-07T09:15:00Z',
    })

  it('asks for a hint with the attempt and the board XML and maps it to a message', async () => {
    vi.mocked(RequestHint).mockResolvedValueOnce(hintDto('h1', 'What if the cache is down?'))

    const hint = await requestHint('a1', '<mxGraphModel/>')

    expect(RequestHint).toHaveBeenCalledExactlyOnceWith('a1', '<mxGraphModel/>')
    expect(hint).toEqual({ id: 'h1', text: 'What if the cache is down?' })
  })

  it('lists the hints already given, in order', async () => {
    vi.mocked(ListHints).mockResolvedValueOnce([hintDto('h1', 'first'), hintDto('h2', 'second')])

    const hints = await listHints('a1')

    expect(ListHints).toHaveBeenCalledExactlyOnceWith('a1')
    expect(hints).toEqual([
      { id: 'h1', text: 'first' },
      { id: 'h2', text: 'second' },
    ])
  })
})

describe('evaluation bindings', () => {
  beforeEach(() => vi.clearAllMocks())

  const dto = () =>
    desktop.EvaluationDTO.createFrom({
      attemptId: 'a1',
      model: 'vendor/model-x',
      score: 72,
      summary: 'Solid.',
      strengths: ['API'],
      gaps: ['No failover'],
      recommendations: ['Add replicas'],
      results: [
        {
          criterionKey: 'scalability',
          criterionName: 'Scalability',
          level: 3,
          justification: 'Cache shown.',
          evidence: 'API -> Cache',
          insufficientEvidence: false,
        },
        {
          criterionKey: 'resilience',
          criterionName: 'Resilience',
          level: 0,
          justification: 'Not shown.',
          evidence: '',
          insufficientEvidence: true,
        },
      ],
      createdAt: '2026-11-07T10:00:00Z',
    })

  const expected = {
    score: 72,
    summary: 'Solid.',
    strengths: ['API'],
    gaps: ['No failover'],
    recommendations: ['Add replicas'],
    results: [
      {
        key: 'scalability',
        name: 'Scalability',
        level: 3,
        justification: 'Cache shown.',
        evidence: 'API -> Cache',
        insufficientEvidence: false,
      },
      {
        key: 'resilience',
        name: 'Resilience',
        level: 0,
        justification: 'Not shown.',
        evidence: '',
        insufficientEvidence: true,
      },
    ],
  }

  it('evaluates the attempt and maps the result', async () => {
    vi.mocked(EvaluateAttempt).mockResolvedValueOnce(dto())

    const evaluation = await evaluateAttempt('a1')

    expect(EvaluateAttempt).toHaveBeenCalledExactlyOnceWith('a1')
    expect(evaluation).toEqual(expected)
  })

  it('reads the stored evaluation', async () => {
    vi.mocked(GetEvaluation).mockResolvedValueOnce(dto())

    const evaluation = await getEvaluation('a1')

    expect(GetEvaluation).toHaveBeenCalledExactlyOnceWith('a1')
    expect(evaluation).toEqual(expected)
  })

  it.each([
    ['invalid_diagram', 'The diagram could not be read.'],
    ['empty_diagram', 'The diagram is empty.'],
    ['invalid_evaluation', 'The evaluator returned an invalid evaluation.'],
    ['internal', 'The evaluator could not complete the evaluation.'],
    ['', 'The evaluator could not complete the evaluation.'],
  ])('explains an evaluation failure coded %s', (code, message) => {
    expect(evaluationErrorMessage(JSON.stringify({ code }))).toBe(message)
  })
})

describe('defense bindings', () => {
  beforeEach(() => vi.clearAllMocks())

  it('uses the fixed answer the backend recognises and its length limit', () => {
    expect(DONT_KNOW_ANSWER).toBe("I don't know")
    expect(MAX_ANSWER_LENGTH).toBe(4000)
  })

  it('asks for the next question and maps it', async () => {
    vi.mocked(NextDefenseQuestion).mockResolvedValueOnce(
      desktop.DefenseQuestionDTO.createFrom({ position: 2, question: 'Why a cache?', total: 5 }),
    )

    const question = await nextDefenseQuestion('a1')

    expect(NextDefenseQuestion).toHaveBeenCalledExactlyOnceWith('a1')
    expect(question).toEqual({ position: 2, text: 'Why a cache?', total: 5 })
  })

  it.each([true, false])('sends an answer and reports complete=%s', async (complete) => {
    vi.mocked(AnswerDefenseQuestion).mockResolvedValueOnce(
      desktop.DefenseDTO.createFrom({ total: 5, turns: [], complete }),
    )

    const progress = await answerDefenseQuestion('a1', 'To cut reads.')

    expect(AnswerDefenseQuestion).toHaveBeenCalledExactlyOnceWith('a1', 'To cut reads.')
    expect(progress).toEqual({ total: 5, complete })
  })

  it('evaluates the defense and maps the outcome with its reasons', async () => {
    vi.mocked(EvaluateDefense).mockResolvedValueOnce(
      desktop.OutcomeDTO.createFrom({
        result: 'FAILED',
        passScore: 80,
        diagramScore: 85,
        understandingScore: 60,
        reasons: [
          {
            kind: 'UNDERSTANDING_SCORE',
            evaluation: 'UNDERSTANDING',
            criterionKey: '',
            criterionName: '',
          },
          {
            kind: 'MANDATORY_CRITERION',
            evaluation: 'DIAGRAM',
            criterionKey: 'resilience',
            criterionName: 'Resilience',
          },
        ],
      }),
    )

    const outcome = await evaluateDefense('a1')

    expect(EvaluateDefense).toHaveBeenCalledExactlyOnceWith('a1')
    expect(outcome).toEqual({
      result: 'FAILED',
      passScore: 80,
      diagramScore: 85,
      understandingScore: 60,
      reasons: [
        { kind: 'UNDERSTANDING_SCORE', evaluation: 'UNDERSTANDING', criterionName: '' },
        { kind: 'MANDATORY_CRITERION', evaluation: 'DIAGRAM', criterionName: 'Resilience' },
      ],
    })
  })

  it.each([
    ['invalid_answer', 'The answer must be between 1 and 4,000 characters.'],
    ['invalid_evaluation', 'The tutor returned an invalid evaluation.'],
    ['invalid_diagram', 'The diagram could not be read.'],
    ['internal', 'The tutor could not continue the defense.'],
    ['', 'The tutor could not continue the defense.'],
  ])('explains a defense failure coded %s', (code, message) => {
    expect(defenseErrorMessage(JSON.stringify({ code }))).toBe(message)
  })
})
