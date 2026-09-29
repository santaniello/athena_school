import type { Data, Node } from 'unist'
import type { Root, Text } from 'mdast'
import { visit } from 'unist-util-visit'

// CITATION_TAG_NAME is the custom hast tag react-markdown renders a citation
// marker as — MessageBubble maps it to <CitationChip>. See
// specs/phases/phase-02-knowledge-engine/18-01-inline-citations-in-chat.md.
export const CITATION_TAG_NAME = 'citation-chip'

// citationPattern matches an inline citation marker, e.g. "[2]". Mirrors the
// backend's own citationMarker
// (internal/application/study/citations.go) — kept as a separate copy since
// the two run in different languages.
const citationPattern = /\[(\d+)\]/g

// CitationChipNode is a custom mdast node type standing in for one [n]
// marker found inside a text node. It carries no children of its own — the
// citation index lives entirely in hProperties, which react-markdown/hast
// forward to the custom element's props.
export interface CitationChipNode extends Node {
  type: 'citationChip'
  data: Data & { hName: typeof CITATION_TAG_NAME; hProperties: { citationIndex: number } }
  children: []
}

// mdast's RootContent/PhrasingContent unions are closed — augmenting both
// maps is required for `parent.children.splice(...)` below to type-check;
// PhrasingContentMap alone is not enough, since RootContentMap is checked
// separately by unist-util-visit's own typings.
declare module 'mdast' {
  interface PhrasingContentMap {
    citationChip: CitationChipNode
  }
  interface RootContentMap {
    citationChip: CitationChipNode
  }
}

function citationChipNode(citationIndex: number): CitationChipNode {
  return {
    type: 'citationChip',
    data: { hName: CITATION_TAG_NAME, hProperties: { citationIndex } },
    children: [],
  }
}

// remarkCitations rewrites every "[n]" inside a text node into a
// citationChip node, splitting the surrounding text around it. It never
// needs to special-case code spans, fenced code blocks, or "[n](url)" link
// syntax: mdast already represents the first two as a `value` string (never
// `text` children, so this text-only visitor never reaches inside them), and
// the third is already its own `link` node with a separate `text` child by
// the time this plugin runs.
export function remarkCitations() {
  return (tree: Root) => {
    visit(tree, 'text', (node: Text, index, parent) => {
      if (!parent || index === undefined) return undefined

      const value = node.value
      const replacement: (Text | CitationChipNode)[] = []
      let lastIndex = 0
      let matched = false
      citationPattern.lastIndex = 0
      let match: RegExpExecArray | null
      while ((match = citationPattern.exec(value)) !== null) {
        matched = true
        if (match.index > lastIndex) {
          replacement.push({ type: 'text', value: value.slice(lastIndex, match.index) })
        }
        replacement.push(citationChipNode(Number(match[1])))
        lastIndex = match.index + match[0].length
      }
      if (!matched) return undefined
      if (lastIndex < value.length) {
        replacement.push({ type: 'text', value: value.slice(lastIndex) })
      }

      parent.children.splice(index, 1, ...replacement)
      // Skip re-visiting the inserted plain-text pieces. Stryker disable
      // next-line ArithmeticOperator: a wrong skip index here only affects
      // traversal efficiency (unist-util-visit re-scanning nodes that,
      // being plain text with no bracketed number left in them, can never
      // match citationPattern again) — every case in citations.test.ts
      // produces the exact same resulting tree whether this returns
      // `index + replacement.length` or the mutant `index - replacement.length`.
      return index + replacement.length
    })
  }
}

// stripCitationMarkers removes every [n] marker from text — used by the
// Copy action so a copied reply never carries citation markers.
export function stripCitationMarkers(text: string): string {
  return text.replace(citationPattern, '')
}
