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

  describe('across inline formatting', () => {
    it('finds a phrase split by bold or italic text', () => {
      const root = rootWith('<p>Use a Cache <strong>eviction</strong> <em>policy</em> here</p>')
      expect(texts(findTextRanges(root, 'cache eviction policy'))).toEqual([
        'Cache eviction policy',
      ])
    })

    it('finds a phrase split into syntax-highlighter tokens', () => {
      const root = rootWith(
        '<div data-slot="code-block"><span class="token">package</span> <span class="token">main</span></div>',
      )
      expect(texts(findTextRanges(root, 'package main'))).toEqual(['package main'])
    })

    it('finds a phrase that ends inside a nested inline element', () => {
      const root = rootWith('<p>see <a href="#">the <code>cache</code></a>!</p>')
      expect(texts(findTextRanges(root, 'the cache'))).toEqual(['the cache'])
    })

    it('highlights across nodes with one range', () => {
      const root = rootWith('<p>ab<b>cd</b>ef</p>')
      const [range] = findTextRanges(root, 'bcde')
      expect(range.startContainer).not.toBe(range.endContainer)
    })

    it('does not join text across blocks', () => {
      const root = rootWith('<p>cache</p><p>eviction</p>')
      expect(findTextRanges(root, 'cacheeviction')).toEqual([])
      expect(findTextRanges(root, 'cache eviction')).toEqual([])
    })

    it('does not join text across list items or table cells', () => {
      const root = rootWith(
        '<ul><li>fifo</li><li>lru</li></ul><table><tr><td>a</td><td>b</td></tr></table>',
      )
      expect(findTextRanges(root, 'fifolru')).toEqual([])
      expect(findTextRanges(root, 'ab')).toEqual([])
    })

    it('does not join text across an ignored element', () => {
      const root = rootWith('<p>copy<span data-find-ignore>X</span>code</p>')
      expect(findTextRanges(root, 'copycode')).toEqual([])
      expect(findTextRanges(root, 'copyxcode')).toEqual([])
    })

    it('does not join text across a button', () => {
      const root = rootWith('<p>one<button>two</button>three</p>')
      expect(findTextRanges(root, 'onetwothree')).toEqual([])
      expect(texts(findTextRanges(root, 'three'))).toEqual(['three'])
    })

    it('finds several matches in one inline-formatted paragraph', () => {
      const root = rootWith('<p>a<b>b</b> ab a<i>b</i></p>')
      expect(findTextRanges(root, 'ab')).toHaveLength(3)
    })

    it('keeps accents mapped to the original offsets across nodes', () => {
      const root = rootWith('<p>Aç<b>ão</b> e mais</p>')
      expect(texts(findTextRanges(root, 'acao'))).toEqual(['Ação'])
    })
  })

  describe('message scopes', () => {
    it('does not join text across messages', () => {
      const root = rootWith(
        '<div data-slot="message-bubble"><span>cache</span></div><div data-slot="message-bubble"><span>eviction</span></div>',
      )
      expect(findTextRanges(root, 'cacheeviction')).toEqual([])
      expect(findTextRanges(root, 'cache eviction')).toEqual([])
      expect(texts(findTextRanges(root, 'eviction'))).toEqual(['eviction'])
    })

    it('ignores text outside the messages when the root has any', () => {
      const root = rootWith(
        '<div>divider cache</div><div data-slot="message-bubble"><p>cache</p></div>',
      )
      expect(findTextRanges(root, 'cache')).toHaveLength(1)
    })
  })
})
