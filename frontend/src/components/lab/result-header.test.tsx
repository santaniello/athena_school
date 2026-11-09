import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import type { AttemptResult } from '@/lib/exercise'
import { ResultHeader } from './result-header'

function result(overrides: Partial<AttemptResult> = {}): AttemptResult {
  return {
    challenge: {
      id: 'c1',
      title: 'Cache and invalidation',
      topic: 'caching',
      difficulty: 'ADVANCED',
      statement: {
        scenario: '',
        functionalRequirements: [],
        nonFunctionalRequirements: [],
        constraints: [],
        objectives: [],
      },
    },
    attemptId: 'a1',
    startedAt: '2026-11-07T09:30:00Z',
    finishedAt: '2026-11-07T12:00:00Z',
    timeSpentSeconds: 1815,
    submitReason: 'USER',
    hintsUsed: 2,
    maxHints: 3,
    outcome: {
      result: 'PASSED',
      passScore: 80,
      diagramScore: 90,
      understandingScore: 85,
      reasons: [],
    },
    diagram: { score: 90, summary: '', strengths: [], gaps: [], recommendations: [], results: [] },
    understanding: {
      score: 85,
      summary: '',
      strengths: [],
      gaps: [],
      recommendations: [],
      results: [],
    },
    defense: [],
    submission: '',
    ...overrides,
  }
}

describe('ResultHeader', () => {
  it('shows the title, topic, difficulty, date, time spent, how it ended and the hints used', () => {
    render(<ResultHeader result={result()} />)

    expect(screen.getByRole('heading', { name: 'Cache and invalidation' })).toBeInTheDocument()
    expect(screen.getByText('caching')).toBeInTheDocument()
    expect(screen.getByText('Advanced')).toBeInTheDocument()
    expect(screen.getByText(/Nov 7, 2026/)).toBeInTheDocument()
    expect(screen.getByText(/Time spent 30:15/)).toBeInTheDocument()
    expect(screen.getByText(/Finished by you/)).toBeInTheDocument()
    expect(screen.getByText(/Hints 2 of 3/)).toBeInTheDocument()
  })

  it('says the exam ended by time', () => {
    render(<ResultHeader result={result({ submitReason: 'TIME_EXPIRED' })} />)

    expect(screen.getByText(/Ended by time/)).toBeInTheDocument()
    expect(screen.queryByText(/Finished by you/)).not.toBeInTheDocument()
  })

  it('keeps an unknown difficulty as it came', () => {
    const base = result()
    render(
      <ResultHeader result={result({ challenge: { ...base.challenge, difficulty: 'EXPERT' } })} />,
    )

    expect(screen.getByText('EXPERT')).toBeInTheDocument()
  })
})
