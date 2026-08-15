import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { SplashScreen } from './splash-screen'

describe('SplashScreen', () => {
  it('renders the Athena monogram centered over a four-sided frame', () => {
    // Given the boot animation is mounted (App shows it while resolving the
    // post-auth view)
    render(<SplashScreen />)

    // When inspecting the rendered element
    const frameStrips = screen.getAllByRole('presentation')
    const logo = screen.getByRole('img', { name: 'Athena' })

    // Then the Greek key frame has exactly one strip per screen edge, and
    // the logo is present
    expect(frameStrips).toHaveLength(4)
    expect(logo).toBeInTheDocument()
  })
})
