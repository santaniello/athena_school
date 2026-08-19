import { describe, expect, it } from 'vitest'
import { NAVIGATION, type AppSection } from './navigation'

// The full roadmap IA from specs/phases/phase-01-desktop-mvp/03-home-screen.md
// — built once, in this order, so later phases only flip a status flag.
const EXPECTED_IDS: AppSection[] = [
  'home',
  'study',
  'knowledge',
  'challenge',
  'progress',
  'flashcards',
  'interview',
  'documentation',
  'settings',
]

describe('NAVIGATION', () => {
  it('has exactly one entry per roadmap section, in sidebar order', () => {
    expect(NAVIGATION.map((item) => item.id)).toEqual(EXPECTED_IDS)
  })

  it('unlocks home, study, knowledge, documentation and settings', () => {
    const unlocked = NAVIGATION.filter((item) => item.status === 'unlocked').map((item) => item.id)
    expect(unlocked).toEqual(['home', 'study', 'knowledge', 'documentation', 'settings'])
  })

  it('pins documentation and settings to the sidebar footer group', () => {
    const footer = NAVIGATION.filter((item) => item.group === 'footer').map((item) => item.id)
    expect(footer).toEqual(['documentation', 'settings'])
  })

  it('gives every locked item a non-empty description for the coming-soon panel', () => {
    const locked = NAVIGATION.filter((item) => item.status === 'locked')
    expect(locked.every((item) => item.description.trim().length > 0)).toBe(true)
  })
})
