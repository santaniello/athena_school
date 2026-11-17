import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  CancelQuizGeneration,
  AcceptQuizOffer,
  DeclineQuizOffer,
  DeleteQuiz,
  GenerateQuiz,
  GetPendingQuizOffer,
  GetProfile,
  GetQuiz,
  GetQuizAttemptResult,
  ListQuizAttempts,
  ListQuizzes,
  ListSessionQuizzes,
  SubmitQuizAttempt,
} from '../../wailsjs/go/desktop/App'
import { setLabLanguage } from '@/i18n'
import {
  acceptQuizOffer,
  cancelQuizGeneration,
  declineQuizOffer,
  countBlank,
  deleteQuiz,
  difficultyForExperience,
  formatQuizDate,
  formatScore,
  generateQuiz,
  getPendingQuizOffer,
  getQuiz,
  getQuizOfferDifficulty,
  getQuizAttemptResult,
  listQuizAttempts,
  listQuizzes,
  listSessionQuizzes,
  parseQuestionCount,
  quizDifficultyLabel,
  quizErrorCode,
  quizErrorMessage,
  scoreBand,
  submitQuizAttempt,
} from './quiz'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  AcceptQuizOffer: vi.fn(),
  CancelQuizGeneration: vi.fn(),
  DeclineQuizOffer: vi.fn(),
  DeleteQuiz: vi.fn(),
  GenerateQuiz: vi.fn(),
  GetPendingQuizOffer: vi.fn(),
  GetProfile: vi.fn(),
  GetQuiz: vi.fn(),
  GetQuizAttemptResult: vi.fn(),
  ListQuizAttempts: vi.fn(),
  ListQuizzes: vi.fn(),
  ListSessionQuizzes: vi.fn(),
  SubmitQuizAttempt: vi.fn(),
}))

describe('quiz bindings', () => {
  beforeEach(() => vi.clearAllMocks())

  it('lists the quizzes, turning an absent latest score into null', async () => {
    vi.mocked(ListQuizzes).mockResolvedValueOnce([
      { id: 'a', sessionTitle: 'Go', latestScore: 7.5 },
      { id: 'b', sessionTitle: 'Caching' },
    ] as never)

    const quizzes = await listQuizzes()

    expect(quizzes.map((quiz) => quiz.latestScore)).toEqual([7.5, null])
    expect(quizzes[1].sessionTitle).toBe('Caching')
  })

  it('keeps a latest score of zero (a quiz taken and failed is not "not taken")', async () => {
    vi.mocked(ListQuizzes).mockResolvedValueOnce([{ id: 'a', latestScore: 0 }] as never)

    const [quiz] = await listQuizzes()

    expect(quiz.latestScore).toBe(0)
  })

  it('reads a quiz to take', async () => {
    const quiz = { id: 'a', sessionTitle: 'Go', difficulty: 'EASY', questions: [] }
    vi.mocked(GetQuiz).mockResolvedValueOnce(quiz as never)

    await expect(getQuiz('a')).resolves.toEqual(quiz)
    expect(GetQuiz).toHaveBeenCalledWith('a')
  })

  it('generates a quiz through the binding', async () => {
    vi.mocked(GenerateQuiz).mockResolvedValueOnce({ id: 'new', latestScore: undefined } as never)

    const quiz = await generateQuiz('session-1', 8, 'HARD')

    expect(GenerateQuiz).toHaveBeenCalledWith('session-1', 8, 'HARD')
    expect(quiz.id).toBe('new')
    expect(quiz.latestScore).toBeNull()
  })

  it('cancels a generation', async () => {
    vi.mocked(CancelQuizGeneration).mockResolvedValueOnce(undefined)

    await cancelQuizGeneration()

    expect(CancelQuizGeneration).toHaveBeenCalledOnce()
  })

  it('submits the selections, a blank as no index', async () => {
    vi.mocked(SubmitQuizAttempt).mockResolvedValueOnce({ id: 'attempt-1' } as never)

    const attempt = await submitQuizAttempt('quiz-1', [
      { questionId: 'q1', selectedIndex: 2 },
      { questionId: 'q2', selectedIndex: null },
    ])

    const [, sent] = vi.mocked(SubmitQuizAttempt).mock.calls[0]
    expect(sent).toEqual([
      { questionId: 'q1', selectedIndex: 2 },
      { questionId: 'q2', selectedIndex: undefined },
    ])
    expect(sent[1].selectedIndex).toBeUndefined()
    expect(attempt.id).toBe('attempt-1')
  })

  it('lists the attempts of a quiz', async () => {
    vi.mocked(ListQuizAttempts).mockResolvedValueOnce([{ id: 'a1' }] as never)

    await expect(listQuizAttempts('quiz-1')).resolves.toEqual([{ id: 'a1' }])
    expect(ListQuizAttempts).toHaveBeenCalledWith('quiz-1')
  })

  it('reads a result, turning an absent selected index into null', async () => {
    vi.mocked(GetQuizAttemptResult).mockResolvedValueOnce({
      attempt: { id: 'a1' },
      sessionTitle: 'Go',
      difficulty: 'EASY',
      questions: [
        { id: 'q1', selectedIndex: 0, correct: false },
        { id: 'q2', correct: false },
      ],
    } as never)

    const result = await getQuizAttemptResult('a1')

    expect(GetQuizAttemptResult).toHaveBeenCalledWith('a1')
    expect(result.questions.map((question) => question.selectedIndex)).toEqual([0, null])
  })

  it('deletes a quiz', async () => {
    vi.mocked(DeleteQuiz).mockResolvedValueOnce(undefined)

    await deleteQuiz('quiz-1')

    expect(DeleteQuiz).toHaveBeenCalledWith('quiz-1')
  })
})

describe('quiz offers of the study chat', () => {
  beforeEach(() => vi.clearAllMocks())

  it('reads the pending offer of a session', async () => {
    vi.mocked(GetPendingQuizOffer).mockResolvedValueOnce({
      id: 'offer-1',
      sessionId: 'session-1',
      messageId: 'message-1',
    } as never)

    const offer = await getPendingQuizOffer('session-1')

    expect(GetPendingQuizOffer).toHaveBeenCalledWith('session-1')
    expect(offer).toEqual({ id: 'offer-1', sessionId: 'session-1', messageId: 'message-1' })
  })

  it('reads no offer as null', async () => {
    vi.mocked(GetPendingQuizOffer).mockResolvedValueOnce(null as never)

    await expect(getPendingQuizOffer('session-1')).resolves.toBeNull()
  })

  it('accepts an offer with the chosen size and difficulty and returns the card', async () => {
    vi.mocked(AcceptQuizOffer).mockResolvedValueOnce({
      id: 'quiz-1',
      messageId: 'message-1',
      latestScore: undefined,
    } as never)

    const quiz = await acceptQuizOffer('offer-1', 8, 'HARD')

    expect(AcceptQuizOffer).toHaveBeenCalledWith('offer-1', 8, 'HARD')
    expect(quiz.id).toBe('quiz-1')
    expect(quiz.latestScore).toBeNull()
  })

  it('declines an offer', async () => {
    vi.mocked(DeclineQuizOffer).mockResolvedValueOnce(undefined)

    await declineQuizOffer('offer-1')

    expect(DeclineQuizOffer).toHaveBeenCalledWith('offer-1')
  })

  it('lists the quizzes of a session, turning an absent latest score into null', async () => {
    vi.mocked(ListSessionQuizzes).mockResolvedValueOnce([
      { id: 'a', messageId: 'message-1', latestScore: 6 },
      { id: 'b', messageId: 'message-2' },
    ] as never)

    const quizzes = await listSessionQuizzes('session-1')

    expect(ListSessionQuizzes).toHaveBeenCalledWith('session-1')
    expect(quizzes.map((quiz) => [quiz.messageId, quiz.latestScore])).toEqual([
      ['message-1', 6],
      ['message-2', null],
    ])
  })

  it('starts the offer from the difficulty of the experience level in the profile', async () => {
    vi.mocked(GetProfile).mockResolvedValueOnce({ experienceLevel: 'advanced' } as never)
    await expect(getQuizOfferDifficulty()).resolves.toBe('HARD')

    vi.mocked(GetProfile).mockResolvedValueOnce({ experienceLevel: 'beginner' } as never)
    await expect(getQuizOfferDifficulty()).resolves.toBe('EASY')
  })

  it('falls back to Medium when the profile cannot be read', async () => {
    vi.mocked(GetProfile).mockRejectedValueOnce(new Error('boom'))

    await expect(getQuizOfferDifficulty()).resolves.toBe('MEDIUM')
  })

  it('writes a message for an offer that is gone, in the profile language', () => {
    expect(quizErrorMessage(JSON.stringify({ code: 'offer_not_found' }))).toBe(
      'This test offer is no longer available.',
    )
    setLabLanguage('pt')
    expect(quizErrorMessage(JSON.stringify({ code: 'offer_not_found' }))).toBe(
      'Esta oferta de teste não está mais disponível.',
    )
    setLabLanguage('en')
  })
})

describe('quiz error messages for the offer', () => {
  it.each([
    ['offer_pending', 'A test offer is already waiting for an answer.'],
    ['cancelled', 'The test generation was cancelled.'],
  ])('writes a message for %s, in the profile language', (code, message) => {
    expect(quizErrorMessage(JSON.stringify({ code }))).toBe(message)
    setLabLanguage('pt')
    expect(quizErrorMessage(JSON.stringify({ code }))).not.toBe(message)
    expect(quizErrorMessage(JSON.stringify({ code }))).not.toBe('Algo deu errado. Tente novamente.')
    setLabLanguage('en')
  })
})

describe('quiz errors', () => {
  it('reads the code of a coded error, from an Error or a string', () => {
    const raw = JSON.stringify({ code: 'quiz_not_found', message: 'x' })

    expect(quizErrorCode(new Error(raw))).toBe('quiz_not_found')
    expect(quizErrorCode(raw)).toBe('quiz_not_found')
  })

  it.each([
    undefined,
    42,
    'plain text',
    new Error('boom'),
    '{"code":5}',
    '{"nope":1}',
    'null',
    '"quiz_not_found"',
    '7',
    '["code"]',
  ])('has no code for %s', (err) => {
    expect(quizErrorCode(err)).toBe('')
  })

  it.each([
    ['invalid_generation', 'The generated test was not valid. Try again.'],
    ['generation_in_progress', 'A test is already being generated.'],
    ['source_session_not_found', 'That study session no longer exists.'],
    ['quiz_not_found', 'That test no longer exists.'],
    ['selection_invalid', 'Some answers were not valid. Reopen the test and try again.'],
    ['invalid_input', 'The test request is not valid. Check the number of questions.'],
    ['attempt_not_found', 'That attempt no longer exists.'],
  ])('writes a message for %s', (code, message) => {
    expect(quizErrorMessage(JSON.stringify({ code }))).toBe(message)
  })

  it('writes a generic message for an unknown code or plain text', () => {
    expect(quizErrorMessage(JSON.stringify({ code: 'internal' }))).toBe(
      'Something went wrong. Please try again.',
    )
    expect(quizErrorMessage('boom')).toBe('Something went wrong. Please try again.')
  })

  it('writes the message in the profile language', () => {
    setLabLanguage('pt')

    expect(quizErrorMessage(JSON.stringify({ code: 'quiz_not_found' }))).toBe(
      'Esse teste não existe mais.',
    )
  })
})

describe('quiz helpers', () => {
  it('formats a date and time in the profile language', () => {
    const iso = '2026-11-16T09:30:00Z'

    expect(formatQuizDate(iso)).toMatch(/Nov/)
    setLabLanguage('pt')
    expect(formatQuizDate(iso)).toMatch(/nov/)
  })

  it.each([
    [0, '0'],
    [10, '10'],
    [7.5, '7.5'],
    [6.7, '6.7'],
    [3, '3'],
  ])('formats the score %s as %s', (score, text) => {
    expect(formatScore(score)).toBe(text)
  })

  it.each([
    [0, 'low'],
    [4.9, 'low'],
    [5, 'mid'],
    [7, 'mid'],
    [7.1, 'high'],
    [10, 'high'],
  ])('puts the score %s in the %s band', (score, band) => {
    expect(scoreBand(score)).toBe(band)
  })

  it('counts the blank selections', () => {
    expect(
      countBlank([
        { questionId: 'a', selectedIndex: 0 },
        { questionId: 'b', selectedIndex: null },
        { questionId: 'c', selectedIndex: 2 },
        { questionId: 'd', selectedIndex: null },
      ]),
    ).toBe(2)
    expect(countBlank([])).toBe(0)
  })

  it.each([
    ['beginner', 'EASY'],
    ['intermediate', 'MEDIUM'],
    ['advanced', 'HARD'],
    ['expert', 'MEDIUM'],
    [undefined, 'MEDIUM'],
  ])('preselects the difficulty for the experience level %s', (level, difficulty) => {
    expect(difficultyForExperience(level)).toBe(difficulty)
  })

  it('labels a difficulty in the profile language and shows an unknown one as it came', () => {
    expect(quizDifficultyLabel('HARD')).toBe('Hard')
    expect(quizDifficultyLabel('WEIRD')).toBe('WEIRD')
    setLabLanguage('pt')
    expect(quizDifficultyLabel('HARD')).toBe('Difícil')
  })
})

describe('parseQuestionCount', () => {
  it('accepts a whole number inside the backend range, bounds included', () => {
    expect(parseQuestionCount('3')).toBe(3)
    expect(parseQuestionCount('10')).toBe(10)
    expect(parseQuestionCount('20')).toBe(20)
  })

  it('refuses a count outside the range', () => {
    expect(parseQuestionCount('2')).toBeNull()
    expect(parseQuestionCount('21')).toBeNull()
    expect(parseQuestionCount('0')).toBeNull()
  })

  it('refuses text that is not a whole number', () => {
    expect(parseQuestionCount('')).toBeNull()
    expect(parseQuestionCount('abc')).toBeNull()
    expect(parseQuestionCount('7.5')).toBeNull()
    expect(parseQuestionCount('-5')).toBeNull()
    expect(parseQuestionCount(' 7')).toBeNull()
  })
})
