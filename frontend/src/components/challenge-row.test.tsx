import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { CatalogItem } from '@/lib/exercise'
import { ChallengeRow } from './challenge-row'

const attempted: CatalogItem = {
  id: 'c1',
  title: 'Cache and invalidation in a catalog',
  topic: 'caching',
  difficulty: 'INTERMEDIATE',
  timeLimitMinutes: 45,
  status: 'NEEDS_IMPROVEMENT',
  attempts: 2,
  bestDiagramScore: 86,
}
const fresh: CatalogItem = {
  id: 'c2',
  title: 'Rate limiter for a public API',
  topic: 'rate limiting',
  difficulty: 'BEGINNER',
  timeLimitMinutes: 30,
  status: 'NOT_STARTED',
  attempts: 0,
  bestDiagramScore: 0,
}

function renderRow(item: CatalogItem, highlighted = false, onStart = vi.fn()) {
  render(
    <ul>
      <ChallengeRow item={item} highlighted={highlighted} onStart={onStart} />
    </ul>,
  )
  return onStart
}

describe('ChallengeRow', () => {
  it('shows the title, difficulty, topic, time limit, attempts, best score and status', () => {
    renderRow(attempted)

    expect(screen.getByText('Cache and invalidation in a catalog')).toBeInTheDocument()
    expect(screen.getByText('Intermediate')).toBeInTheDocument()
    expect(screen.getByText('caching · 45 min · 2 attempts')).toBeInTheDocument()
    expect(screen.getByText('Best 86 · Needs improvement')).toBeInTheDocument()
  })

  it('says "not started" and shows no score for a challenge nobody tried', () => {
    renderRow(fresh)

    expect(screen.getByText('rate limiting · 30 min · not started')).toBeInTheDocument()
    expect(screen.queryByText(/Best/)).not.toBeInTheDocument()
    expect(screen.queryByText('Not started')).not.toBeInTheDocument()
  })

  it('uses the singular for a single attempt', () => {
    renderRow({ ...attempted, attempts: 1 })

    expect(screen.getByText('caching · 45 min · 1 attempt')).toBeInTheDocument()
  })

  it('shows the status alone while the challenge has no evaluated attempt', () => {
    renderRow({ ...attempted, status: 'IN_PROGRESS', attempts: 1, bestDiagramScore: 0 })

    expect(screen.getByText('In progress')).toBeInTheDocument()
    expect(screen.queryByText(/Best/)).not.toBeInTheDocument()
  })

  it('offers Start for a challenge nobody tried', () => {
    renderRow(fresh)
    expect(screen.getByRole('button', { name: 'Start' })).toBeInTheDocument()
  })

  it('offers Start again for a challenge already tried', () => {
    renderRow(attempted)
    expect(screen.getByRole('button', { name: 'Start again' })).toBeInTheDocument()
  })

  it('asks to start the exam of the row when Start is clicked', async () => {
    const user = userEvent.setup()
    const onStart = renderRow(fresh)

    expect(screen.getByRole('button', { name: 'Start' })).toBeEnabled()
    await user.click(screen.getByRole('button', { name: 'Start' }))

    expect(onStart).toHaveBeenCalledExactlyOnceWith(fresh)
  })

  it('asks to start again from Start again', async () => {
    const user = userEvent.setup()
    const onStart = renderRow(attempted)

    await user.click(screen.getByRole('button', { name: 'Start again' }))

    expect(onStart).toHaveBeenCalledExactlyOnceWith(attempted)
  })

  it('marks the row that was just generated', () => {
    renderRow(fresh, true)

    expect(screen.getByRole('listitem')).toHaveAttribute('aria-current', 'true')
    expect(screen.getByRole('listitem')).toHaveClass('bg-primary/10', 'ring-1')
  })

  it('does not mark an ordinary row', () => {
    renderRow(fresh)

    expect(screen.getByRole('listitem')).not.toHaveAttribute('aria-current')
    expect(screen.getByRole('listitem')).toHaveClass('flex')
    expect(screen.getByRole('listitem')).not.toHaveClass('bg-primary/10')
  })

  it('falls back to the raw value for a status or difficulty it does not know', () => {
    renderRow({ ...fresh, status: 'MYSTERY', difficulty: 'EXPERT', attempts: 1 })

    expect(screen.getByText('EXPERT')).toBeInTheDocument()
    expect(screen.getByText('MYSTERY')).toBeInTheDocument()
  })
})
