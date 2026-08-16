import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ComingSoonPanel } from './coming-soon-panel'
import { NAVIGATION } from '@/lib/navigation'

const knowledgeItem = NAVIGATION.find((item) => item.id === 'knowledge')!

describe('ComingSoonPanel', () => {
  it('shows the section title, ship phase and description', () => {
    // Given a locked nav item
    render(<ComingSoonPanel item={knowledgeItem} />)

    // Then its title, phase and description are all shown
    expect(screen.getByRole('heading', { name: 'Knowledge' })).toBeInTheDocument()
    expect(screen.getByText('Planned for Phase 2')).toBeInTheDocument()
    expect(screen.getByText(knowledgeItem.description)).toBeInTheDocument()
  })
})
