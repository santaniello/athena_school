import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { LabHint } from '@/lib/lab'
import { LabTutorPanel } from './lab-tutor-panel'

const hints: LabHint[] = [
  { id: 'h1', text: 'Think about who owns the cache.' },
  { id: 'h2', text: 'What happens on a write?' },
]

function renderPanel(overrides: Partial<Parameters<typeof LabTutorPanel>[0]> = {}) {
  const onAskHint = vi.fn()
  render(<LabTutorPanel hints={[]} hintsLeft={3} onAskHint={onAskHint} {...overrides} />)
  return onAskHint
}

describe('LabTutorPanel', () => {
  it('is empty until asked, with a short sentence', () => {
    renderPanel()

    expect(screen.getByRole('heading', { name: 'Tutor Athena' })).toBeInTheDocument()
    expect(screen.getByText('Ask for a hint if you get stuck.')).toBeInTheDocument()
    expect(screen.queryByRole('log')).not.toBeInTheDocument()
  })

  it('shows the hints left out of three', () => {
    renderPanel({ hintsLeft: 2 })

    expect(screen.getByText('Hints left: 2 of 3')).toBeInTheDocument()
  })

  it('calls onAskHint when Ask for a hint is clicked', async () => {
    const user = userEvent.setup()
    const onAskHint = renderPanel()

    await user.click(screen.getByRole('button', { name: 'Ask for a hint' }))

    expect(onAskHint).toHaveBeenCalledOnce()
  })

  it('shows the hints as messages, in order', () => {
    renderPanel({ hints, hintsLeft: 1 })

    expect(screen.getByRole('log', { name: 'Hints' })).toBeInTheDocument()
    const messages = screen.getAllByRole('article')
    expect(messages.map((message) => message.textContent)).toEqual([
      'Think about who owns the cache.',
      'What happens on a write?',
    ])
  })

  it('removes the button and says none are left at zero hints', () => {
    renderPanel({ hints, hintsLeft: 0 })

    expect(screen.queryByRole('button', { name: 'Ask for a hint' })).not.toBeInTheDocument()
    expect(screen.getByText('No hints left')).toBeInTheDocument()
  })

  it('has no free-text input', () => {
    renderPanel()

    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  })
})
