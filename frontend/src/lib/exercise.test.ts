import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  CancelChallengeGeneration,
  GenerateChallenge,
  ListChallengeCatalog,
  ListChallengeTopics,
  ListSessionsForGeneration,
} from '../../wailsjs/go/desktop/App'
import { desktop } from '../../wailsjs/go/models'
import {
  DIFFICULTY_LABELS,
  STATUS_LABELS,
  cancelChallengeGeneration,
  exerciseErrorCode,
  exerciseErrorMessage,
  generateChallenge,
  listChallengeCatalog,
  listChallengeTopics,
  listSessionsForGeneration,
} from './exercise'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  CancelChallengeGeneration: vi.fn(),
  GenerateChallenge: vi.fn(),
  ListChallengeCatalog: vi.fn(),
  ListChallengeTopics: vi.fn(),
  ListSessionsForGeneration: vi.fn(),
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
