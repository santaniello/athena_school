import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { LabPreview } from './lab-preview'

describe('LabPreview', () => {
  it('renders the lab from fixtures: an Intermediate challenge with a 45-minute deadline', () => {
    render(<LabPreview />)

    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      'Cache and invalidation in a catalog',
    )
    expect(screen.getByText('Intermediate')).toBeInTheDocument()
    expect(screen.getByRole('timer')).toHaveTextContent(/^4[45]:\d\d$/)
  })

  it('shows five base and two challenge criteria, one of them mandatory', () => {
    render(<LabPreview />)

    expect(
      screen
        .getAllByRole('listitem')
        .filter((item) => item.closest('section')?.querySelector('h3')?.textContent === 'Rubric'),
    ).toHaveLength(7)
    expect(screen.getAllByLabelText('Mandatory')).toHaveLength(1)
  })

  it('starts with three hints left and reveals one fixture hint per request', async () => {
    const user = userEvent.setup()
    render(<LabPreview />)
    expect(screen.getByText('Hints left: 3 of 3')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Ask for a hint' }))

    expect(screen.getByText('Hints left: 2 of 3')).toBeInTheDocument()
    expect(screen.getAllByRole('article')).toHaveLength(1)
  })

  it('has no way to finish an exam: the board is empty', () => {
    render(<LabPreview />)

    expect(screen.getByRole('button', { name: 'Finish and evaluate' })).toBeDisabled()
  })
})
