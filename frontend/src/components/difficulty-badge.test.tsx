import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { DifficultyBadge } from './difficulty-badge'

describe('DifficultyBadge', () => {
  it.each([
    ['BEGINNER', 'Beginner', 'text-emerald-700'],
    ['INTERMEDIATE', 'Intermediate', 'text-amber-700'],
    ['ADVANCED', 'Advanced', 'text-red-700'],
  ])('colors %s by level', (difficulty, label, colorClass) => {
    render(<DifficultyBadge difficulty={difficulty} />)

    expect(screen.getByText(label)).toHaveClass(colorClass)
  })

  it('falls back to a muted badge with the raw value for an unknown level', () => {
    render(<DifficultyBadge difficulty="EXPERT" />)

    expect(screen.getByText('EXPERT')).toHaveClass('bg-muted')
  })
})
