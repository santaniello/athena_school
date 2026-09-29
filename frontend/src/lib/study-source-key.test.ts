import { describe, expect, it } from 'vitest'
import { sourceKey } from '@/lib/study-source-key'
import type { StudySource } from '@/lib/study'

// testSource fills in the fields this test doesn't care about with harmless
// defaults, so each case only spells out what it's actually asserting on.
function testSource(overrides: Partial<StudySource>): StudySource {
  return {
    chunkId: 'chunk-1',
    itemId: 'item-1',
    sourceType: 'imported_doc',
    filePath: '',
    heading: '',
    concept: '',
    score: 0,
    excerpt: '',
    ...overrides,
  }
}

describe('sourceKey', () => {
  it('does not collide when a delimiter character appears inside a field', () => {
    // Given two sources whose fields, naively joined with "|", would produce
    // the exact same string
    const a = testSource({ filePath: 'a|b', concept: 'c' })
    const b = testSource({ filePath: 'a', concept: 'b|c' })

    // Then their keys are still distinct
    expect(sourceKey(a)).not.toBe(sourceKey(b))
  })

  it('produces the same key for two sources with identical fields', () => {
    // Given the same source content twice
    const source = testSource({ sourceType: 'user_note', concept: 'Idempotency', score: 0.5 })

    // Then the key is stable
    expect(sourceKey(source)).toBe(sourceKey({ ...source }))
  })
})
