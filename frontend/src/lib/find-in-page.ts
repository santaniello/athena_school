// Marks a subtree whose text the find bar must not search (UI chrome).
const FIND_IGNORE_ATTRIBUTE = 'data-find-ignore'
// Spread onto an element to keep its text out of the search.
export const FIND_IGNORE_PROPS = { [FIND_IGNORE_ATTRIBUTE]: '' }

const IGNORED_SELECTOR = `button, [${FIND_IGNORE_ATTRIBUTE}]`

function normalizeChar(char: string): string {
  return char.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase()
}

// Lowercases and strips accents, so "Ação" and "acao" compare equal.
export function normalizeForSearch(text: string): string {
  let normalized = ''
  for (const char of text) normalized += normalizeChar(char)
  return normalized
}

interface NormalizedText {
  value: string
  // For each normalized char, the [start, end) offsets of its source char.
  spans: Array<[number, number]>
}

// Normalizes keeping a map back to the original offsets: a char can change
// length when normalized, so indexes of the result cannot be reused as is.
function normalizeWithSpans(text: string): NormalizedText {
  let value = ''
  const spans: Array<[number, number]> = []
  let offset = 0
  for (const char of text) {
    const piece = normalizeChar(char)
    for (let i = 0; i < piece.length; i++) spans.push([offset, offset + char.length])
    value += piece
    offset += char.length
  }
  return { value, spans }
}

function isIgnored(node: Node): boolean {
  return node.parentElement?.closest(IGNORED_SELECTOR) != null
}

// findTextRanges returns one Range per occurrence of query inside root's text
// nodes, in document order. Matching is case- and accent-insensitive and does
// not cross text-node boundaries.
export function findTextRanges(root: HTMLElement, query: string): Range[] {
  const needle = normalizeForSearch(query)
  if (needle === '') return []

  const ranges: Range[] = []
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT)
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    if (isIgnored(node)) continue
    const { value, spans } = normalizeWithSpans(node.textContent ?? '')
    for (
      let from = value.indexOf(needle);
      from !== -1;
      from = value.indexOf(needle, from + needle.length)
    ) {
      const range = document.createRange()
      range.setStart(node, spans[from][0])
      range.setEnd(node, spans[from + needle.length - 1][1])
      ranges.push(range)
    }
  }
  return ranges
}
