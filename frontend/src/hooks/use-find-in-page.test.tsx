import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useFindInPage } from './use-find-in-page'

function setup(html: string) {
  const container = document.createElement('div')
  container.innerHTML = html
  document.body.append(container)
  const onNavigate = vi.fn()
  const hook = renderHook(() => useFindInPage({ containerRef: { current: container }, onNavigate }))
  return { container, onNavigate, ...hook }
}

describe('useFindInPage', () => {
  let scrollIntoView: ReturnType<typeof vi.fn>

  beforeEach(() => {
    scrollIntoView = vi.fn()
    Element.prototype.scrollIntoView =
      scrollIntoView as unknown as typeof Element.prototype.scrollIntoView
  })

  afterEach(() => {
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

  it('scrolls to the active match and tells the screen to stop following the bottom', () => {
    const { result, onNavigate } = setup('<p>a a</p>')
    act(() => result.current.open())
    act(() => result.current.setQuery('a'))

    act(() => result.current.next())

    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'center' })
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
})
