import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { CanonicalProblem } from '@/lib/exercise'
import { CanonicalProblemRow } from './canonical-problem-row'

const fresh: CanonicalProblem = {
  id: 'url-shortener',
  name: 'URL shortener',
  summary: 'Turn long URLs into short links.',
  attempts: 0,
  hasHistory: false,
  difficulties: [
    { difficulty: 'BEGINNER', generated: false, challengeId: '', status: '' },
    { difficulty: 'INTERMEDIATE', generated: false, challengeId: '', status: '' },
    { difficulty: 'ADVANCED', generated: false, challengeId: '', status: '' },
  ],
}

const attempted: CanonicalProblem = {
  ...fresh,
  attempts: 3,
  hasHistory: true,
  difficulties: [
    { difficulty: 'BEGINNER', generated: true, challengeId: 'c1', status: 'COMPLETED' },
    { difficulty: 'INTERMEDIATE', generated: true, challengeId: 'c2', status: 'IN_PROGRESS' },
    { difficulty: 'ADVANCED', generated: false, challengeId: '', status: '' },
  ],
}

function renderRow(problem: CanonicalProblem, onStart = vi.fn(), onHistory = vi.fn()) {
  render(
    <ul>
      <CanonicalProblemRow problem={problem} onStart={onStart} onHistory={onHistory} />
    </ul>,
  )
  return { onStart, onHistory }
}

describe('CanonicalProblemRow', () => {
  it('shows the name, the description, nothing generated and no attempt yet', () => {
    renderRow(fresh)

    expect(screen.getByText('URL shortener')).toBeInTheDocument()
    expect(screen.getByText('Turn long URLs into short links.')).toBeInTheDocument()
    expect(screen.getByText('0 of 3 generated · not started')).toBeInTheDocument()
  })

  it('counts the generated difficulties and the attempts', () => {
    renderRow(attempted)

    expect(screen.getByText('2 of 3 generated · 3 attempts')).toBeInTheDocument()
  })

  it('starts the problem', async () => {
    const { onStart } = renderRow(fresh)

    await userEvent.click(screen.getByRole('button', { name: 'Start' }))

    expect(onStart).toHaveBeenCalledWith(fresh)
  })

  it('offers History only when there is a finished attempt', async () => {
    const withoutHistory = renderRow(fresh)
    expect(screen.queryByRole('button', { name: 'History' })).not.toBeInTheDocument()
    expect(withoutHistory.onHistory).not.toHaveBeenCalled()
  })

  it('opens the history of the problem', async () => {
    const { onHistory } = renderRow(attempted)

    await userEvent.click(screen.getByRole('button', { name: 'History' }))

    expect(onHistory).toHaveBeenCalledWith(attempted)
  })

  it('shows the three levels, each with its state, dimming the ones not generated', () => {
    renderRow(attempted)

    const beginner = screen.getByText('Beginner').parentElement
    const advanced = screen.getByText('Advanced').parentElement
    expect(screen.getByText('Intermediate')).toBeInTheDocument()
    expect(beginner).toHaveAttribute('title', 'Completed')
    expect(beginner).not.toHaveClass('opacity-40')
    expect(advanced).toHaveAttribute('title', 'Not generated')
    expect(advanced).toHaveClass('opacity-40')
  })
})
