import { beforeEach, describe, expect, it } from 'vitest'
import { clearDraft, getDraft, setDraft } from './study-drafts'

describe('study-drafts', () => {
  beforeEach(() => {
    clearDraft('a')
    clearDraft('b')
  })

  it('returns an empty string for an unknown session', () => {
    expect(getDraft('a')).toBe('')
  })

  it('keeps drafts isolated per session', () => {
    setDraft('a', 'hello')
    expect(getDraft('a')).toBe('hello')
    expect(getDraft('b')).toBe('')
  })

  it('clears a draft, and setting empty text removes it', () => {
    setDraft('a', 'x')
    clearDraft('a')
    expect(getDraft('a')).toBe('')
    setDraft('b', 'y')
    setDraft('b', '')
    expect(getDraft('b')).toBe('')
  })
})
