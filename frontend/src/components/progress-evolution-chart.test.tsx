import { render, screen, within } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { setLabLanguage } from '@/i18n'
import type { ProgressEvolution } from '@/lib/progress'
import { ProgressEvolutionChart } from './progress-evolution-chart'

const point = (order: number, score: number, itemNumber = 1, title = '') => ({
  order,
  itemNumber,
  title,
  score,
  at: '2026-11-02T10:00:00Z',
})

const EVOLUTION: ProgressEvolution = {
  quiz: [point(1, 50), point(2, 70, 2), point(3, 90)],
  lab: [point(1, 76, 1, 'Design a cache')],
  passScore: 80,
  enough: true,
}

describe('ProgressEvolutionChart', () => {
  beforeEach(() => setLabLanguage('en'))

  it('draws a point per attempt of each series and names them in the legend', () => {
    const { container } = render(<ProgressEvolutionChart evolution={EVOLUTION} />)

    expect(container.querySelectorAll('[data-series="quiz"] circle')).toHaveLength(3)
    expect(container.querySelectorAll('[data-series="lab"] circle')).toHaveLength(1)
    expect(screen.getByText('Quiz')).toBeInTheDocument()
    expect(screen.getByText('Arch Lab')).toBeInTheDocument()
  })

  it('names the item, the position, the score and the date in each tooltip', () => {
    render(<ProgressEvolutionChart evolution={EVOLUTION} />)

    const chart = screen.getByRole('img', { name: /evolution/i })
    expect(within(chart).getByText(/^Quiz 2 · Assessment 2 · 70% · /)).toBeInTheDocument()
    expect(within(chart).getByText(/^Design a cache · Assessment 1 · 76% · /)).toBeInTheDocument()
  })

  it('marks the minimum score with a dashed line and a legend entry', () => {
    render(<ProgressEvolutionChart evolution={EVOLUTION} />)

    expect(screen.getByTestId('pass-score-line')).toHaveAttribute('stroke-dasharray')
    expect(screen.getByText('Minimum 80%')).toBeInTheDocument()
  })

  it('places the minimum on a fixed 0-100 axis', () => {
    render(<ProgressEvolutionChart evolution={{ ...EVOLUTION, passScore: 100 }} />)

    // 100 is the top of the plot area; 0 would be the bottom
    expect(screen.getByTestId('pass-score-line')).toHaveAttribute('y1', '12')
  })

  it('leaves a series without points out of the legend', () => {
    render(<ProgressEvolutionChart evolution={{ ...EVOLUTION, lab: [] }} />)

    expect(screen.queryByText('Arch Lab')).not.toBeInTheDocument()
    expect(screen.getByText('Quiz')).toBeInTheDocument()
  })

  it('shows a message instead of the chart when there is not enough data', () => {
    render(
      <ProgressEvolutionChart
        evolution={{ ...EVOLUTION, quiz: [point(1, 50)], lab: [], enough: false }}
      />,
    )

    expect(screen.getByText('Not enough assessments yet')).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })
})
