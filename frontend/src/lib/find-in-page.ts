const FIND_IGNORE_ATTRIBUTE = 'data-find-ignore'
// Spread onto an element to keep its text out of the search.
export const FIND_IGNORE_PROPS = { [FIND_IGNORE_ATTRIBUTE]: '' }

const IGNORED_SELECTOR = `button, [${FIND_IGNORE_ATTRIBUTE}]`
const MESSAGE_SELECTOR = '[data-slot="message-bubble"]'

// Elements whose text flows into their parent's, so a phrase may run across
// them ("Cache **eviction**", syntax-highlighter tokens). Any other element is
// a block: a match never crosses its edge.
const INLINE_TAGS = new Set([
  'A',
  'ABBR',
  'B',
  'CODE',
  'DEL',
  'EM',
  'I',
  'INS',
  'KBD',
  'MARK',
  'S',
  'SMALL',
  'SPAN',
  'STRONG',
  'SUB',
  'SUP',
  'U',
])

// Stands between pieces of text that must not be joined into one match; the
// query never contains it.
const BREAK = '\n'

function normalizeChar(char: string): string {
  return char.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase()
}

// Lowercases and strips accents, so "Ação" and "acao" compare equal.
export function normalizeForSearch(text: string): string {
  let normalized = ''
  for (const char of text) normalized += normalizeChar(char)
  return normalized
}

// Where one char of the searchable text came from; null for a break.
interface Origin {
  node: Text
  start: number
  end: number
}

interface SearchableText {
  value: string
  // One entry per char of value.
  origins: Array<Origin | null>
}

function isIgnored(node: Node): boolean {
  return node.parentElement?.closest(IGNORED_SELECTOR) != null
}

// The closest non-inline ancestor: text nodes sharing it are one run of text.
function blockOf(node: Node): Element | null {
  let element = node.parentElement
  while (element && INLINE_TAGS.has(element.tagName)) element = element.parentElement
  return element
}

// Joins the text nodes of scope into one string, remembering the source of
// every char. A block edge or an ignored element breaks the text.
function searchableTextOf(scope: Element): SearchableText {
  let value = ''
  const origins: Array<Origin | null> = []
  const addBreak = () => {
    if (value !== '' && value[value.length - 1] !== BREAK) {
      value += BREAK
      origins.push(null)
    }
  }

  let previousBlock: Element | null = null
  const walker = document.createTreeWalker(scope, NodeFilter.SHOW_TEXT)
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    if (isIgnored(node)) {
      addBreak()
      continue
    }
    const block = blockOf(node)
    if (block !== previousBlock) addBreak()
    previousBlock = block

    const text = node as Text
    let offset = 0
    for (const char of text.data) {
      const piece = normalizeChar(char)
      for (let i = 0; i < piece.length; i++) {
        origins.push({ node: text, start: offset, end: offset + char.length })
      }
      value += piece
      offset += char.length
    }
  }
  return { value, origins }
}

// Each message is searched on its own, so a phrase never spans two messages.
// A root with no message in it (a plain container) is one scope.
function scopesOf(root: HTMLElement): Element[] {
  const messages = Array.from(root.querySelectorAll(MESSAGE_SELECTOR))
  return messages.length > 0 ? messages : [root]
}

// findTextRanges returns one Range per occurrence of query in root's messages,
// in document order. Matching is case- and accent-insensitive; a match may run
// across inline formatting but not across blocks, buttons or ignored elements.
export function findTextRanges(root: HTMLElement, query: string): Range[] {
  const needle = normalizeForSearch(query)
  if (needle === '') return []

  const ranges: Range[] = []
  for (const scope of scopesOf(root)) {
    const { value, origins } = searchableTextOf(scope)
    for (
      let from = value.indexOf(needle);
      from !== -1;
      from = value.indexOf(needle, from + needle.length)
    ) {
      // The needle holds no break, so both ends are real chars.
      const first = origins[from] as Origin
      const last = origins[from + needle.length - 1] as Origin
      const range = document.createRange()
      range.setStart(first.node, first.start)
      range.setEnd(last.node, last.end)
      ranges.push(range)
    }
  }
  return ranges
}
