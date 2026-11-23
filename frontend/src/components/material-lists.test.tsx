import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { LabList, MindMapList, QuizList } from './material-lists'
import type { MaterialList } from '@/hooks/use-session-materials'
import type { MindMapSummary } from '@/lib/mindmap'
import type { QuizSummary } from '@/lib/quiz'
import type { SessionExerciseCard } from '@/lib/exercise'

function list<T>(items: T[], status: MaterialList<T>['status'] = 'ready'): MaterialList<T> {
  return { items, status, retry: vi.fn() }
}

const mindMap: MindMapSummary = {
  id: 'm1',
  sessionId: 's1',
  sessionTitle: 'Caching',
  title: 'Eviction overview',
  createdAt: '2026-03-03T10:00:00Z',
  updatedAt: '2026-03-05T10:00:00Z',
  messageId: '',
}

function quiz(overrides: Partial<QuizSummary> = {}): QuizSummary {
  return {
    id: 'q1',
    sessionId: 's1',
    sessionTitle: 'Caching',
    difficulty: 'MEDIUM',
    questionCount: 10,
    createdAt: '2026-03-03T10:00:00Z',
    attemptCount: 0,
    latestScore: null,
    messageId: '',
    ...overrides,
  }
}

function lab(overrides: Partial<SessionExerciseCard> = {}): SessionExerciseCard {
  return {
    sessionExerciseId: 'e1',
    messageId: 'msg',
    challengeId: 'c1',
    kind: 'EXAM',
    title: 'Design a distributed LRU cache',
    difficulty: 'INTERMEDIATE',
    timeLimitMinutes: 45,
    status: 'NOT_STARTED',
    attempts: 0,
    latest: null,
    ...overrides,
  }
}

describe('shared list states', () => {
  it('shows a loading message', () => {
    render(<MindMapList list={list<MindMapSummary>([], 'loading')} onOpen={vi.fn()} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows an error with a retry action', async () => {
    const failed = list<MindMapSummary>([], 'error')
    render(<MindMapList list={failed} onOpen={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))

    expect(screen.getByText('Could not load this list.')).toBeInTheDocument()
    expect(failed.retry).toHaveBeenCalledOnce()
  })

  it('shows an empty message per kind', () => {
    const { unmount } = render(<MindMapList list={list<MindMapSummary>([])} onOpen={vi.fn()} />)
    expect(screen.getByText('No mind maps in this session yet.')).toBeInTheDocument()
    unmount()
    const quizzes = render(<QuizList list={list<QuizSummary>([])} onOpen={vi.fn()} />)
    expect(screen.getByText('No quizzes in this session yet.')).toBeInTheDocument()
    quizzes.unmount()
    render(<LabList list={list<SessionExerciseCard>([])} onOpen={vi.fn()} />)
    expect(screen.getByText('No Arch Labs in this session yet.')).toBeInTheDocument()
  })
})

describe('MindMapList', () => {
  it('shows the title and when it was updated, and opens the map on click', async () => {
    const onOpen = vi.fn()
    render(<MindMapList list={list([mindMap])} onOpen={onOpen} />)

    expect(screen.getByText(/Updated/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /Eviction overview/ }))

    expect(onOpen).toHaveBeenCalledWith(mindMap)
  })
})

describe('QuizList', () => {
  it('numbers quizzes by creation order with their difficulty, and marks an untaken one as new', () => {
    render(
      <QuizList
        list={list([quiz({ id: 'a' }), quiz({ id: 'b', difficulty: 'HARD' })])}
        onOpen={vi.fn()}
      />,
    )

    expect(screen.getByText('Quiz 1 · Medium')).toBeInTheDocument()
    expect(screen.getByText('Quiz 2 · Hard')).toBeInTheDocument()
    expect(screen.getAllByText('New')).toHaveLength(2)
    expect(screen.getAllByText(/10 questions/)).toHaveLength(2)
    expect(screen.getAllByText(/not taken/)).toHaveLength(2)
  })

  it('shows the attempts and the latest score of a taken quiz', () => {
    render(<QuizList list={list([quiz({ attemptCount: 2, latestScore: 7.5 })])} onOpen={vi.fn()} />)

    expect(screen.getByText('7.5')).toBeInTheDocument()
    expect(screen.getByText(/2 attempts/)).toBeInTheDocument()
    expect(screen.queryByText('New')).not.toBeInTheDocument()
  })

  it('opens the quiz on click', async () => {
    const onOpen = vi.fn()
    const q = quiz()
    render(<QuizList list={list([q])} onOpen={onOpen} />)

    await userEvent.click(screen.getByRole('button', { name: /Quiz 1/ }))

    expect(onOpen).toHaveBeenCalledWith(q)
  })
})

describe('LabList', () => {
  it('shows the challenge with difficulty, time limit and attempts, as new when never attempted', () => {
    render(<LabList list={list([lab()])} onOpen={vi.fn()} />)

    expect(screen.getByText('Design a distributed LRU cache')).toBeInTheDocument()
    expect(screen.getByText(/45 min/)).toBeInTheDocument()
    expect(screen.getByText(/not attempted/)).toBeInTheDocument()
    expect(screen.getByText('New')).toBeInTheDocument()
  })

  it('shows the latest outcome of an attempted challenge', () => {
    render(
      <LabList
        list={list([
          lab({
            attempts: 2,
            latest: { outcome: 'PASSED', diagramScore: 8, understandingScore: 7 },
          }),
          lab({
            sessionExerciseId: 'e2',
            title: 'Second',
            attempts: 1,
            latest: { outcome: 'FAILED', diagramScore: 3, understandingScore: 3 },
          }),
        ])}
        onOpen={vi.fn()}
      />,
    )

    expect(screen.getByText('Passed')).toBeInTheDocument()
    expect(screen.getByText('Failed')).toBeInTheDocument()
    expect(screen.getByText(/2 attempts/)).toBeInTheDocument()
  })

  it('opens the challenge on click', async () => {
    const onOpen = vi.fn()
    const card = lab()
    render(<LabList list={list([card])} onOpen={onOpen} />)

    await userEvent.click(screen.getByRole('button', { name: /Design a distributed/ }))

    expect(onOpen).toHaveBeenCalledWith(card)
  })
})
