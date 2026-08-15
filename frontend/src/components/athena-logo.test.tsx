import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { AthenaLogo } from './athena-logo'

describe('AthenaLogo', () => {
  it('renders an accessible svg monogram', () => {
    // Given no extra className
    render(<AthenaLogo />)

    // When inspecting the rendered element
    const logo = screen.getByRole('img', { name: 'Athena' })

    // Then it's the svg monogram, addressable by its accessible name
    expect(logo.tagName.toLowerCase()).toBe('svg')
    expect(logo).toHaveAttribute('viewBox', '0 0 120 120')
  })

  it('merges a custom className onto the default classes', () => {
    // Given a caller-supplied className (e.g. sizing per screen)
    render(<AthenaLogo className="h-12 w-12" />)

    // When inspecting the rendered element
    const logo = screen.getByRole('img', { name: 'Athena' })

    // Then the custom class is present alongside the default ones
    expect(logo.getAttribute('class')).toContain('h-12')
    expect(logo.getAttribute('class')).toContain('w-12')
  })
})
