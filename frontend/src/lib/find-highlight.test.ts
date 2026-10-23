import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import {
  ACTIVE_HIGHLIGHT_NAME,
  MATCH_HIGHLIGHT_NAME,
  clearFindHighlights,
  paintFindHighlights,
} from './find-highlight'

class FakeHighlight {
  readonly ranges: Range[]
  constructor(...ranges: Range[]) {
    this.ranges = ranges
  }
}

function makeRanges(count: number): Range[] {
  return Array.from({ length: count }, () => document.createRange())
}

describe('find highlights', () => {
  let registry: Map<string, FakeHighlight>

  beforeEach(() => {
    registry = new Map()
    Object.defineProperty(globalThis, 'CSS', {
      value: { highlights: registry },
      configurable: true,
    })
    Object.defineProperty(globalThis, 'Highlight', { value: FakeHighlight, configurable: true })
  })

  afterEach(() => {
    Reflect.deleteProperty(globalThis, 'CSS')
    Reflect.deleteProperty(globalThis, 'Highlight')
  })

  it('paints every match and the active one separately', () => {
    const ranges = makeRanges(3)

    paintFindHighlights(ranges, 1)

    expect(registry.get(MATCH_HIGHLIGHT_NAME)?.ranges).toEqual([ranges[0], ranges[2]])
    expect(registry.get(ACTIVE_HIGHLIGHT_NAME)?.ranges).toEqual([ranges[1]])
  })

  it('clears the previous paint before painting again', () => {
    paintFindHighlights(makeRanges(2), 0)
    paintFindHighlights([], -1)

    expect(registry.has(MATCH_HIGHLIGHT_NAME)).toBe(false)
    expect(registry.has(ACTIVE_HIGHLIGHT_NAME)).toBe(false)
  })

  it('removes both highlights on clear', () => {
    paintFindHighlights(makeRanges(2), 0)

    clearFindHighlights()

    expect(registry.size).toBe(0)
  })

  it('does nothing when the Custom Highlight API is unavailable', () => {
    Reflect.deleteProperty(globalThis, 'CSS')

    expect(() => paintFindHighlights(makeRanges(1), 0)).not.toThrow()
    expect(() => clearFindHighlights()).not.toThrow()
  })
})
