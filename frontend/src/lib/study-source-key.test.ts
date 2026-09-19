import { describe, expect, it } from 'vitest'
import { sourceKey } from '@/lib/study-source-key'
import type { StudySource } from '@/lib/study'

describe('sourceKey', () => {
  it('does not collide when a delimiter character appears inside a field', () => {
    // Given two sources whose fields, naively joined with "|", would produce
    // the exact same string
    const a: StudySource = {
      sourceType: 'imported_doc',
      filePath: 'a|b',
      heading: '',
      concept: 'c',
      score: 0,
    }
    const b: StudySource = {
      sourceType: 'imported_doc',
      filePath: 'a',
      heading: '',
      concept: 'b|c',
      score: 0,
    }

    // Then their keys are still distinct
    expect(sourceKey(a)).not.toBe(sourceKey(b))
  })

  it('produces the same key for two sources with identical fields', () => {
    // Given the same source content twice
    const source: StudySource = {
      sourceType: 'user_note',
      filePath: '',
      heading: '',
      concept: 'Idempotency',
      score: 0.5,
    }

    // Then the key is stable
    expect(sourceKey(source)).toBe(sourceKey({ ...source }))
  })
})
