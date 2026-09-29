import { describe, expect, it } from 'vitest'
import { unified } from 'unified'
import remarkParse from 'remark-parse'
import remarkGfm from 'remark-gfm'
import type { Root, Text, Link } from 'mdast'
import {
  CITATION_TAG_NAME,
  remarkCitations,
  stripCitationMarkers,
  type CitationChipNode,
} from './citations'

// runSync (not .process/.processSync, which would need a stringifier that
// knows how to serialize the custom citationChip node) — this only ever
// needs to inspect the resulting tree.
function parse(markdown: string): Root {
  const processor = unified().use(remarkParse).use(remarkGfm).use(remarkCitations)
  return processor.runSync(processor.parse(markdown)) as Root
}

function paragraphChildren(tree: Root) {
  const paragraph = tree.children[0]
  if (paragraph.type !== 'paragraph') throw new Error('expected a paragraph')
  return paragraph.children
}

describe('CITATION_TAG_NAME', () => {
  it('is the fixed custom tag name MessageBubble maps to CitationChip', () => {
    // MessageBubble's components map keys off this exact string (as an
    // object computed-property key), so its literal value matters, not
    // just its identity.
    expect(CITATION_TAG_NAME).toBe('citation-chip')
  })
})

describe('remarkCitations', () => {
  it('converts a single marker into a citationChip node', () => {
    // Given a sentence with one citation marker
    const tree = parse('Channels are typed pipes [1].')

    // Then the text is split into a text piece, a chip, and a text piece
    const children = paragraphChildren(tree)
    expect(children).toHaveLength(3)
    expect(children[0]).toMatchObject({ type: 'text', value: 'Channels are typed pipes ' })
    expect(children[1].type).toBe('citationChip')
    expect(children[2]).toMatchObject({ type: 'text', value: '.' })
  })

  it('converts multiple markers, in document order', () => {
    // Given a sentence citing two different passages
    const tree = parse('Use select [1] to multiplex [2].')

    const children = paragraphChildren(tree)
    const chips = children.filter((child) => child.type === 'citationChip') as CitationChipNode[]
    expect(chips).toHaveLength(2)
    expect(chips[0].data.hProperties.citationIndex).toBe(1)
    expect(chips[1].data.hProperties.citationIndex).toBe(2)
  })

  it('converts adjacent markers with no text between them', () => {
    // Given two markers with nothing separating them
    const tree = parse('[1][2]')

    const children = paragraphChildren(tree)
    expect(children).toHaveLength(2)
    expect(children[0].type).toBe('citationChip')
    expect(children[1].type).toBe('citationChip')
  })

  it('sets hName and hProperties.citationIndex on the chip node', () => {
    // Given a marker citing passage 7
    const tree = parse('As shown [7].')

    const chip = paragraphChildren(tree).find(
      (child) => child.type === 'citationChip',
    ) as CitationChipNode

    expect(chip.data.hName).toBe(CITATION_TAG_NAME)
    expect(chip.data.hProperties.citationIndex).toBe(7)
  })

  it('leaves text with no markers unchanged', () => {
    // Given a sentence with no citation markers
    const tree = parse('Channels are typed pipes.')

    const children = paragraphChildren(tree)
    expect(children).toHaveLength(1)
    expect(children[0]).toMatchObject({ type: 'text', value: 'Channels are typed pipes.' })
  })

  it('does not convert a marker inside an inline code span', () => {
    // Given a code span containing a bracketed number
    const tree = parse('Run `foo[1]bar`.')

    const children = paragraphChildren(tree)
    expect(children.some((child) => child.type === 'citationChip')).toBe(false)
    expect(children.some((child) => child.type === 'inlineCode')).toBe(true)
  })

  it('does not convert a marker inside a fenced code block', () => {
    // Given a fenced code block containing a bracketed number
    const tree = parse('```\nfoo[1]bar\n```')

    expect(tree.children[0].type).toBe('code')
    // No paragraph is produced at all, so there is nothing to walk for chips.
    expect(tree.children.some((child) => child.type === 'paragraph')).toBe(false)
  })

  it('does not convert "[n](url)" link syntax into a citation', () => {
    // Given a real Markdown link whose text is a bracketed number
    const tree = parse('See [1](https://example.com).')

    const children = paragraphChildren(tree)
    expect(children.some((child) => child.type === 'citationChip')).toBe(false)
    const link = children.find((child) => child.type === 'link') as Link
    expect(link).toBeDefined()
    expect((link.children[0] as Text).value).toBe('1')
  })
})

describe('stripCitationMarkers', () => {
  it('removes a single marker', () => {
    expect(stripCitationMarkers('Channels are typed pipes [1].')).toBe('Channels are typed pipes .')
  })

  it('removes multiple markers', () => {
    expect(stripCitationMarkers('[1] and [2] and [12]')).toBe(' and  and ')
  })

  it('leaves text with no markers unchanged', () => {
    expect(stripCitationMarkers('No markers here.')).toBe('No markers here.')
  })

  it('leaves a bracketed non-digit word alone', () => {
    expect(stripCitationMarkers('See [todo] before shipping.')).toBe('See [todo] before shipping.')
  })
})
