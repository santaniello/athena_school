import { describe, expect, it } from 'vitest'
import { DOCUMENTATION, plannedSectionIds } from './documentation'

describe('DOCUMENTATION', () => {
  it('opens with the system purpose, then study, then knowledge, then the reference', () => {
    // Given the in-app manual
    // When reading its section order
    const ids = DOCUMENTATION.map((section) => section.id)

    // Then it explains why Athena exists before explaining how to use it
    expect(ids).toEqual(['what-is-athena', 'study-sessions', 'knowledge-engine', 'reference'])
  })

  it('gives every section a unique anchor id', () => {
    // Given the section ids double as table-of-contents anchors
    const ids = DOCUMENTATION.map((section) => section.id)

    // Then none collide, or the contents links would jump to the wrong place
    expect(new Set(ids).size).toBe(ids.length)
  })

  it('gives every section a title, a summary and at least one paragraph', () => {
    // Given a section with no prose would render as an empty heading
    // Then every section carries real content
    for (const section of DOCUMENTATION) {
      expect(section.title.trim().length).toBeGreaterThan(0)
      expect(section.summary.trim().length).toBeGreaterThan(0)
      expect(section.body.length).toBeGreaterThan(0)
      expect(section.body.every((paragraph) => paragraph.trim().length > 0)).toBe(true)
    }
  })

  it('gives every topic both a term and a description', () => {
    // Given topics render as a definition list
    const topics = DOCUMENTATION.flatMap((section) => section.topics)

    // Then neither half is ever blank
    expect(topics.length).toBeGreaterThan(0)
    expect(topics.every((topic) => topic.term.trim().length > 0)).toBe(true)
    expect(topics.every((topic) => topic.description.trim().length > 0)).toBe(true)
  })

  it('marks the knowledge engine as planned and everything else as available', () => {
    // Given the Knowledge Engine is specified but not built, documenting it
    // as if it shipped would mislead the reader
    // When collecting the planned sections
    // Then only the knowledge engine is flagged
    expect(plannedSectionIds()).toEqual(['knowledge-engine'])
  })
})

describe('plannedSectionIds', () => {
  it('returns an empty list once nothing is planned', () => {
    // Given a manual whose sections have all shipped
    const shipped = DOCUMENTATION.filter((section) => section.status === 'available')

    // Then no section is reported as planned
    expect(shipped.every((section) => section.status !== 'planned')).toBe(true)
  })
})
