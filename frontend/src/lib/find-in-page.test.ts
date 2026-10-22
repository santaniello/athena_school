import { describe, expect, it } from 'vitest'
import { findTextRanges, normalizeForSearch } from './find-in-page'

function rootWith(html: string): HTMLElement {
  const root = document.createElement('div')
  root.innerHTML = html
  document.body.append(root)
  return root
}

function texts(ranges: Range[]): string[] {
  return ranges.map((range) => range.toString())
}

describe('normalizeForSearch', () => {
  it('lowercases and strips accents keeping one output char per input char', () => {
    expect(normalizeForSearch('Ação')).toBe('acao')
  })
})

describe('findTextRanges', () => {
  it('returns no ranges for an empty query', () => {
    const root = rootWith('<p>hello</p>')
    expect(findTextRanges(root, '')).toEqual([])
  })

  it('finds every occurrence, case-insensitively', () => {
    const root = rootWith('<p>Cache cache CACHE</p>')
    expect(texts(findTextRanges(root, 'cache'))).toEqual(['Cache', 'cache', 'CACHE'])
  })

  it('ignores accents on both the query and the text', () => {
    const root = rootWith('<p>Ação e acao</p>')
    expect(texts(findTextRanges(root, 'acao'))).toEqual(['Ação', 'acao'])
    expect(texts(findTextRanges(root, 'AÇÃO'))).toEqual(['Ação', 'acao'])
  })

  it('finds matches across several text nodes in document order', () => {
    const root = rootWith('<p>one fifo</p><pre><code>fifo()</code></pre><p>FIFO</p>')
    expect(texts(findTextRanges(root, 'fifo'))).toEqual(['fifo', 'fifo', 'FIFO'])
  })

  it('finds overlapping-free consecutive matches in the same node', () => {
    const root = rootWith('<p>aaaa</p>')
    expect(findTextRanges(root, 'aa')).toHaveLength(2)
  })

  it('skips subtrees marked with data-find-ignore', () => {
    const root = rootWith(
      '<div><span data-find-ignore>Copy code</span><button>Copy code</button><p>code</p></div>',
    )
    expect(texts(findTextRanges(root, 'code'))).toEqual(['code'])
  })

  it('skips buttons and their descendants', () => {
    const root = rootWith('<button><span>Copy message</span></button><p>message</p>')
    expect(texts(findTextRanges(root, 'message'))).toEqual(['message'])
  })

  it('returns no ranges when nothing matches', () => {
    const root = rootWith('<p>hello</p>')
    expect(findTextRanges(root, 'zzz')).toEqual([])
  })
})
