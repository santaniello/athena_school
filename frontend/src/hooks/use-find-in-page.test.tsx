import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { clearFindHighlights, paintFindHighlights } from '@/lib/find-highlight'
import { useFindInPage } from './use-find-in-page'

vi.mock('@/lib/find-highlight', () => ({
  paintFindHighlights: vi.fn(),
  clearFindHighlights: vi.fn(),
}))

function setup(html: string) {
  const container = document.createElement('div')
  container.innerHTML = html
  document.body.append(container)
  const onNavigate = vi.fn()
  const hook = renderHook(() => useFindInPage({ containerRef: { current: container }, onNavigate }))
  return { container, onNavigate, ...hook }
}

describe('useFindInPage', () => {
  beforeEach(() => {
    vi.mocked(paintFindHighlights).mockClear()
    vi.mocked(clearFindHighlights).mockClear()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
    document.body.innerHTML = ''
  })

  it('starts closed with no matches', () => {
    const { result } = setup('<p>cache</p>')
    expect(result.current.isOpen).toBe(false)
    expect(result.current.matchCount).toBe(0)
  })

  it('counts matches as the query changes and selects the first', () => {
    const { result } = setup('<p>cache one</p><p>Cache two</p>')
    act(() => result.current.open())
    act(() => result.current.setQuery('cache'))
    expect(result.current.matchCount).toBe(2)
    expect(result.current.activeIndex).toBe(0)
  })

  it('moves to the next and previous match, wrapping around', () => {
    const { result } = setup('<p>a a a</p>')
    act(() => result.current.open())
    act(() => result.current.setQuery('a'))

    act(() => result.current.next())
    expect(result.current.activeIndex).toBe(1)
    act(() => result.current.next())
    act(() => result.current.next())
    expect(result.current.activeIndex).toBe(0)
    act(() => result.current.prev())
    expect(result.current.activeIndex).toBe(2)
  })

  it('centres the active match in the container, even deep inside a tall block', () => {
    const { result, container } = setup('<p>a a</p>')
    // The container is 400px tall, starts at y=100 and is scrolled by 50px.
    container.getBoundingClientRect = () => ({ top: 100, height: 400 }) as DOMRect
    Object.defineProperty(container, 'clientHeight', { value: 400, configurable: true })
    container.scrollTop = 50
    // The next match sits 1000px below the container's top edge, 20px tall.
    vi.spyOn(Range.prototype, 'getBoundingClientRect').mockReturnValue({
      top: 1100,
      height: 20,
    } as DOMRect)
    act(() => result.current.open())
    act(() => result.current.setQuery('a'))

    act(() => result.current.next())

    // Centred: 50 + (1100 - 100) - (400 - 20) / 2 = 860
    expect(container.scrollTop).toBe(860)
  })

  it('tells the screen to stop following the bottom when jumping to a match', () => {
    const { result, onNavigate } = setup('<p>a a</p>')
    act(() => result.current.open())
    act(() => result.current.setQuery('a'))

    act(() => result.current.next())

    expect(onNavigate).toHaveBeenCalled()
  })

  it('does nothing on next/prev without matches', () => {
    const { result, onNavigate } = setup('<p>a</p>')
    act(() => result.current.open())
    act(() => result.current.setQuery('zzz'))

    act(() => result.current.next())
    act(() => result.current.prev())

    expect(result.current.activeIndex).toBe(-1)
    expect(onNavigate).not.toHaveBeenCalled()
  })

  it('closes, clearing the query and the matches', () => {
    const { result } = setup('<p>a</p>')
    act(() => result.current.open())
    act(() => result.current.setQuery('a'))

    act(() => result.current.close())

    expect(result.current.isOpen).toBe(false)
    expect(result.current.query).toBe('')
    expect(result.current.matchCount).toBe(0)
    expect(result.current.activeIndex).toBe(-1)
    expect(clearFindHighlights).toHaveBeenCalled()
  })

  it('ignores next after closing, as the old matches are gone', () => {
    const { result, onNavigate } = setup('<p>a a</p>')
    act(() => result.current.open())
    act(() => result.current.setQuery('a'))
    act(() => result.current.close())

    act(() => result.current.next())

    expect(result.current.activeIndex).toBe(-1)
    expect(onNavigate).not.toHaveBeenCalled()
  })

  it('recomputes the matches when the content changes, keeping the active match', async () => {
    const { result, container } = setup('<p>a</p><p id="second">a</p>')
    act(() => result.current.open())
    act(() => result.current.setQuery('a'))
    act(() => result.current.next())
    expect(result.current.activeIndex).toBe(1)

    // A page prepended above: the active match is now the third one.
    act(() => {
      const first = document.createElement('p')
      first.textContent = 'a a'
      container.prepend(first)
    })

    await waitFor(() => expect(result.current.matchCount).toBe(4))
    expect(result.current.activeIndex).toBe(3)
  })

  it('paints every match with the active one when the query or the position changes', () => {
    const { result } = setup('<p>a a</p>')
    act(() => result.current.open())
    act(() => result.current.setQuery('a'))
    expect(paintFindHighlights).toHaveBeenLastCalledWith(expect.any(Array), 0)
    expect(vi.mocked(paintFindHighlights).mock.lastCall?.[0]).toHaveLength(2)

    act(() => result.current.next())

    expect(paintFindHighlights).toHaveBeenLastCalledWith(expect.any(Array), 1)
  })

  it('clears the highlights when the screen unmounts', () => {
    const { unmount } = setup('<p>a</p>')
    vi.mocked(clearFindHighlights).mockClear()

    unmount()

    expect(clearFindHighlights).toHaveBeenCalled()
  })

  it('searches nothing and does not throw once the container is gone', () => {
    const container = document.createElement('div')
    container.innerHTML = '<p>a a</p>'
    const containerRef = { current: container as HTMLElement | null }
    const { result } = renderHook(() => useFindInPage({ containerRef }))
    act(() => result.current.open())
    act(() => result.current.setQuery('a'))
    containerRef.current = null

    expect(() => act(() => result.current.next())).not.toThrow()
    act(() => result.current.setQuery('a'))

    expect(result.current.matchCount).toBe(0)
  })

  it('moves between matches without an onNavigate callback', () => {
    const container = document.createElement('div')
    container.innerHTML = '<p>a a</p>'
    const { result } = renderHook(() => useFindInPage({ containerRef: { current: container } }))
    act(() => result.current.open())
    act(() => result.current.setQuery('a'))

    expect(() => act(() => result.current.next())).not.toThrow()
    expect(result.current.activeIndex).toBe(1)
  })

  it('focuses and selects the input each time the bar is opened, even when already open', () => {
    const { result } = setup('<p>a</p>')
    const input = document.createElement('input')
    input.value = 'cache'
    document.body.append(input)
    result.current.inputRef.current = input
    const select = vi.spyOn(input, 'select')

    act(() => result.current.open())
    expect(document.activeElement).toBe(input)
    expect(select).toHaveBeenCalledTimes(1)

    input.blur()
    act(() => result.current.open())

    expect(document.activeElement).toBe(input)
    expect(select).toHaveBeenCalledTimes(2)
  })

  it('keeps the active match by position, not just by text node', async () => {
    const { result, container } = setup('<p>a a a</p>')
    act(() => result.current.open())
    act(() => result.current.setQuery('a'))
    act(() => result.current.next())
    act(() => result.current.next())
    expect(result.current.activeIndex).toBe(2)

    act(() => {
      ;(container.querySelector('p')!.firstChild as Text).appendData(' a')
    })

    await waitFor(() => expect(result.current.matchCount).toBe(4))
    expect(result.current.activeIndex).toBe(2)
  })

  it('clamps the active match when the content shrinks past it', async () => {
    const { result, container } = setup('<p>a</p><p class="gone">a</p><p class="gone">a</p>')
    act(() => result.current.open())
    act(() => result.current.setQuery('a'))
    act(() => result.current.next())
    act(() => result.current.next())
    expect(result.current.activeIndex).toBe(2)

    act(() => {
      container.querySelectorAll('.gone').forEach((node) => node.remove())
    })

    await waitFor(() => expect(result.current.matchCount).toBe(1))
    expect(result.current.activeIndex).toBe(0)
  })

  describe('watching the content', () => {
    // Flushes the MutationObserver microtask while the timers are faked.
    async function mutate(change: () => void) {
      change()
      await Promise.resolve()
    }

    function setupWithFakeTimers(html: string) {
      vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
      const hook = setup(html)
      act(() => hook.result.current.open())
      act(() => hook.result.current.setQuery('a'))
      vi.mocked(paintFindHighlights).mockClear()
      return hook
    }

    it('coalesces a burst of changes into one recompute after the debounce', async () => {
      const { container } = setupWithFakeTimers('<p>a</p>')

      await mutate(() => container.append(document.createElement('p')))
      act(() => {
        vi.advanceTimersByTime(60)
      })
      await mutate(() => container.append(document.createElement('p')))
      act(() => {
        vi.advanceTimersByTime(60)
      })
      expect(paintFindHighlights).not.toHaveBeenCalled()

      act(() => {
        vi.advanceTimersByTime(40)
      })
      expect(paintFindHighlights).toHaveBeenCalledTimes(1)
    })

    it('recomputes when a nested element is added', async () => {
      const { result, container } = setupWithFakeTimers('<div><section></section></div>')

      await mutate(() => {
        const nested = document.createElement('p')
        nested.textContent = 'a'
        container.querySelector('section')!.append(nested)
      })
      act(() => {
        vi.advanceTimersByTime(100)
      })

      expect(result.current.matchCount).toBe(1)
    })

    it('recomputes when only the text of an existing node changes', async () => {
      const { result, container } = setupWithFakeTimers('<div><p>b</p></div>')

      await mutate(() => {
        container.querySelector('p')!.firstChild!.nodeValue = 'a'
      })
      act(() => {
        vi.advanceTimersByTime(100)
      })

      expect(result.current.matchCount).toBe(1)
    })

    it('stops watching once the bar is closed', async () => {
      const { result, container } = setupWithFakeTimers('<p>a</p>')
      const disconnect = vi.spyOn(MutationObserver.prototype, 'disconnect')

      await mutate(() => container.append(document.createElement('p')))
      act(() => result.current.close())
      act(() => {
        vi.advanceTimersByTime(200)
      })

      expect(disconnect).toHaveBeenCalled()
      expect(paintFindHighlights).not.toHaveBeenCalled()
    })
  })
})
