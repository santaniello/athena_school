import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { SummaryMarker } from './summary-marker'

describe('SummaryMarker', () => {
  it('announces that earlier messages were summarized', () => {
    // Given the marker
    // When it renders
    render(<SummaryMarker />)

    // Then the divider text is shown
    expect(screen.getByText('Earlier messages were summarized')).toBeInTheDocument()
  })

  it('stands out in orange, text and line alike', () => {
    // Given the marker
    // When it renders
    const { container } = render(<SummaryMarker />)

    // Then the text and both lines use the orange accent
    expect(screen.getByText('Earlier messages were summarized')).toHaveClass('text-orange-600')
    const lines = container.querySelectorAll('[aria-hidden="true"]')
    expect(lines).toHaveLength(2)
    lines.forEach((line) => expect(line).toHaveClass('bg-orange-600/60'))
  })

  it('offers no action of its own', () => {
    // Given the marker
    // When it renders
    render(<SummaryMarker />)

    // Then viewing the summary is left to the session header
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })
})
