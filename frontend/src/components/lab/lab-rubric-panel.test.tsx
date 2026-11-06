import { describe, expect, it } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import type { LabCriterion } from '@/lib/lab'
import { LabRubricPanel } from './lab-rubric-panel'

const criteria: LabCriterion[] = [
  {
    id: 'k1',
    name: 'Overall architecture',
    description: 'Components and their responsibilities are clear.',
    weight: 20,
    mandatory: false,
  },
  {
    id: 'k2',
    name: 'Consistency',
    description: 'Cache invalidation keeps reads correct.',
    weight: 25,
    mandatory: true,
  },
]

describe('LabRubricPanel', () => {
  it('shows one row per criterion with its name, description and weight', () => {
    render(<LabRubricPanel criteria={criteria} />)

    const rows = screen.getAllByRole('listitem')
    expect(rows).toHaveLength(2)
    expect(within(rows[0]).getByText('Overall architecture')).toBeInTheDocument()
    expect(
      within(rows[0]).getByText('Components and their responsibilities are clear.'),
    ).toBeInTheDocument()
    expect(within(rows[0]).getByText('20')).toBeInTheDocument()
    expect(within(rows[1]).getByText('25')).toBeInTheDocument()
  })

  it('marks only the mandatory criterion', () => {
    render(<LabRubricPanel criteria={criteria} />)

    const rows = screen.getAllByRole('listitem')
    expect(within(rows[0]).queryByLabelText('Mandatory')).not.toBeInTheDocument()
    expect(within(rows[1]).getByLabelText('Mandatory')).toBeInTheDocument()
  })

  it('never shows a percentage, level or score while the exam runs', () => {
    render(<LabRubricPanel criteria={criteria} />)

    expect(screen.queryByText(/%/)).not.toBeInTheDocument()
    expect(screen.queryByText(/score/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/level/i)).not.toBeInTheDocument()
  })

  it('has a Rubric heading', () => {
    render(<LabRubricPanel criteria={criteria} />)

    expect(screen.getByRole('heading', { name: 'Rubric' })).toBeInTheDocument()
  })
})
