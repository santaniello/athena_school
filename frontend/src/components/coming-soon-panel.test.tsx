import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ComingSoonPanel } from './coming-soon-panel'
import { NAVIGATION } from '@/lib/navigation'

const lockedItem = NAVIGATION.find((item) => item.id === 'progress')!

describe('ComingSoonPanel', () => {
  it('shows the section title, ship phase and description', () => {
    // Given a locked nav item
    render(<ComingSoonPanel item={lockedItem} />)

    // Then its title, phase and description are all shown
    expect(screen.getByRole('heading', { name: 'Progress' })).toBeInTheDocument()
    expect(screen.getByText('Planned for Phase 3')).toBeInTheDocument()
    expect(screen.getByText(lockedItem.description)).toBeInTheDocument()
  })
})
