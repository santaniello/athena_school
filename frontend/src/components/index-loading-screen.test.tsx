import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { IndexLoadingScreen } from './index-loading-screen'

describe('IndexLoadingScreen', () => {
  it('shows the loading message while the knowledge index builds', () => {
    // Given the initial background load has not finished yet
    render(<IndexLoadingScreen />)

    // Then the loading message is shown
    expect(screen.getByText('Loading knowledge index...')).toBeInTheDocument()
  })
})
