import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ComingSoonPanel } from './coming-soon-panel'
import { NAVIGATION } from '@/lib/navigation'

const challengeItem = NAVIGATION.find((item) => item.id === 'challenge')!

describe('ComingSoonPanel', () => {
  it('shows the section title, ship phase and description', () => {
    // Given a locked nav item
    render(<ComingSoonPanel item={challengeItem} />)

    // Then its title, phase and description are all shown
    expect(screen.getByRole('heading', { name: 'Challenge' })).toBeInTheDocument()
    expect(screen.getByText('Planned for Phase 3')).toBeInTheDocument()
    expect(screen.getByText(challengeItem.description)).toBeInTheDocument()
  })
})
