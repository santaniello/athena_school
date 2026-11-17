import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { QuizDifficultyBadge } from './quiz-difficulty-badge'

describe('QuizDifficultyBadge', () => {
  it.each([
    ['EASY', 'Easy', 'text-emerald'],
    ['MEDIUM', 'Medium', 'text-amber'],
    ['HARD', 'Hard', 'text-red'],
    ['WEIRD', 'WEIRD', 'text-muted'],
  ])('shows %s as %s in its own color', (difficulty, label, color) => {
    render(<QuizDifficultyBadge difficulty={difficulty} />)

    expect(screen.getByText(label).className).toContain(color)
  })
})
