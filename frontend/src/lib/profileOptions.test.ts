import { describe, expect, it } from 'vitest'
import { EXPERIENCE_LEVELS, labelFor } from './profileOptions'

describe('labelFor', () => {
  it('resolves the label for a known value', () => {
    // Given a known experience level value
    // When resolving its label
    // Then it returns the matching label
    expect(labelFor(EXPERIENCE_LEVELS, 'intermediate')).toBe('Intermediate')
  })

  it('falls back to the raw value when it is unknown', () => {
    // Given a value with no matching option
    // When resolving its label
    // Then it returns the raw value unchanged
    expect(labelFor(EXPERIENCE_LEVELS, 'expert')).toBe('expert')
  })
})
