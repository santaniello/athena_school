import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ThinkingIndicator } from './thinking-indicator'

describe('ThinkingIndicator', () => {
  it('announces itself to assistive tech as a status region', () => {
    // Given the indicator is rendered
    render(<ThinkingIndicator />)

    // Then it exposes a status role with a descriptive label, so screen
    // readers announce it without extra markup
    expect(screen.getByRole('status', { name: /thinking/i })).toBeInTheDocument()
  })

  it('renders three animated laurel leaves, matching the app logo motif', () => {
    // Given the indicator is rendered
    const { container } = render(<ThinkingIndicator />)

    // Then three separately-animated leaf shapes are shown, not a generic
    // spinner or an external image/gif
    const leaves = container.querySelectorAll('[data-slot="thinking-leaf"]')
    expect(leaves).toHaveLength(3)
    for (const leaf of leaves) {
      expect(leaf).toHaveClass('animate-leaf-bounce')
    }
  })

  it('staggers each leaf so they bounce in sequence, not all at once', () => {
    // Given the indicator is rendered
    const { container } = render(<ThinkingIndicator />)

    // Then each leaf has a distinct animation-delay
    const leaves = Array.from(container.querySelectorAll('[data-slot="thinking-leaf"]'))
    const delays = leaves.map((leaf) => (leaf as HTMLElement).style.animationDelay)
    expect(new Set(delays).size).toBe(3)
  })
})
