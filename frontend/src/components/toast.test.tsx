import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { Toast, ToastViewport, type ToastItem } from './toast'

describe('Toast', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('announces a warning as an alert and a notice as a status', () => {
    // Given one warning toast and one info toast
    render(
      <>
        <Toast tone="warning" message="It failed" durationMs={10_000} onDismiss={() => {}} />
        <Toast tone="info" message="It worked" durationMs={5_000} onDismiss={() => {}} />
      </>,
    )

    // Then the warning is exposed as an alert and the notice as a status
    expect(screen.getByRole('alert')).toHaveTextContent('It failed')
    expect(screen.getByRole('status')).toHaveTextContent('It worked')
  })

  it('renders the message at a readable size (at least 14px)', () => {
    // Given a toast
    render(<Toast tone="warning" message="Readable" durationMs={10_000} onDismiss={() => {}} />)

    // Then its text uses the 14px (text-sm) size
    expect(screen.getByRole('alert')).toHaveClass('text-sm')
  })

  it('stays until the user closes it when it has no duration', () => {
    // Given a toast without a duration
    const onDismiss = vi.fn()
    render(<Toast tone="info" message="x" onDismiss={onDismiss} />)

    // When a long time passes
    act(() => vi.advanceTimersByTime(10 * 60_000))

    // Then it is still there
    expect(onDismiss).not.toHaveBeenCalled()

    // When the user clicks the close button
    fireEvent.click(screen.getByRole('button', { name: /dismiss/i }))

    // Then it is dismissed
    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('dismisses itself after its duration', () => {
    // Given a 10s toast
    const onDismiss = vi.fn()
    render(<Toast tone="warning" message="x" durationMs={10_000} onDismiss={onDismiss} />)

    // When just under the duration passes
    act(() => vi.advanceTimersByTime(9_999))
    // Then it is still open
    expect(onDismiss).not.toHaveBeenCalled()

    // When the duration elapses
    act(() => vi.advanceTimersByTime(1))
    // Then it dismisses exactly once
    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('has a close button that dismisses it immediately', () => {
    // Given a toast
    const onDismiss = vi.fn()
    render(<Toast tone="warning" message="x" durationMs={10_000} onDismiss={onDismiss} />)

    // When clicking close
    fireEvent.click(screen.getByRole('button', { name: /dismiss/i }))

    // Then it dismisses
    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('pauses while hovered and resumes with the remaining time', () => {
    // Given a 10s toast that is hovered after 4s
    const onDismiss = vi.fn()
    render(<Toast tone="warning" message="x" durationMs={10_000} onDismiss={onDismiss} />)
    const toast = screen.getByRole('alert')
    act(() => vi.advanceTimersByTime(4_000))
    fireEvent.mouseEnter(toast)

    // When a long time passes while hovered
    act(() => vi.advanceTimersByTime(60_000))
    // Then it stays open
    expect(onDismiss).not.toHaveBeenCalled()

    // When the pointer leaves, only the remaining 6s count
    fireEvent.mouseLeave(toast)
    act(() => vi.advanceTimersByTime(5_999))
    expect(onDismiss).not.toHaveBeenCalled()
    act(() => vi.advanceTimersByTime(1))
    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('pauses while focused and stays paused until both hover and focus are gone', () => {
    // Given a toast that is both hovered and focused
    const onDismiss = vi.fn()
    render(<Toast tone="warning" message="x" durationMs={1_000} onDismiss={onDismiss} />)
    const toast = screen.getByRole('alert')
    fireEvent.mouseEnter(toast)
    fireEvent.focus(screen.getByRole('button', { name: /dismiss/i }))

    // When only the pointer leaves
    fireEvent.mouseLeave(toast)
    act(() => vi.advanceTimersByTime(5_000))
    // Then it is still paused by focus
    expect(onDismiss).not.toHaveBeenCalled()

    // When focus leaves too
    fireEvent.blur(screen.getByRole('button', { name: /dismiss/i }))
    act(() => vi.advanceTimersByTime(1_000))
    // Then it dismisses
    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('does not restart its timer when the parent re-renders with a new onDismiss', () => {
    // Given a toast re-rendered with a new callback after 6s
    const first = vi.fn()
    const second = vi.fn()
    const { rerender } = render(
      <Toast tone="warning" message="x" durationMs={10_000} onDismiss={first} />,
    )
    act(() => vi.advanceTimersByTime(6_000))
    rerender(<Toast tone="warning" message="x" durationMs={10_000} onDismiss={second} />)

    // When the original deadline passes
    act(() => vi.advanceTimersByTime(4_000))

    // Then the latest callback fires on the original schedule
    expect(first).not.toHaveBeenCalled()
    expect(second).toHaveBeenCalledTimes(1)
  })
})

describe('ToastViewport', () => {
  it('stacks toasts top-center in one column without overlap and dismisses by id', () => {
    // Given two toasts
    const toasts: ToastItem[] = [
      { id: 'a', tone: 'warning', message: 'First', durationMs: 10_000 },
      { id: 'b', tone: 'info', message: 'Second', durationMs: 5_000 },
    ]
    const onDismiss = vi.fn()
    render(<ToastViewport toasts={toasts} onDismiss={onDismiss} />)

    // Then they live in one fixed, top-centered flex column
    const region = screen.getByRole('region', { name: /notifications/i })
    expect(region).toHaveClass('fixed', 'top-4', 'flex-col')
    expect(region).toHaveTextContent('First')
    expect(region).toHaveTextContent('Second')

    // When closing the second one
    fireEvent.click(screen.getAllByRole('button', { name: /dismiss/i })[1])

    // Then its id is reported
    expect(onDismiss).toHaveBeenCalledWith('b')
  })

  it('renders nothing visible when there are no toasts', () => {
    // Given no toasts
    render(<ToastViewport toasts={[]} onDismiss={() => {}} />)

    // Then there is no alert or status
    expect(screen.queryByRole('alert')).toBeNull()
    expect(screen.queryByRole('status')).toBeNull()
  })
})
