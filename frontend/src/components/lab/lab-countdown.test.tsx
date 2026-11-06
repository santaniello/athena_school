import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, render, screen } from '@testing-library/react'
import { LabCountdown } from './lab-countdown'

const NOW = new Date('2026-11-06T12:00:00Z').getTime()
const MINUTE = 60 * 1000

function deadlineIn(msLeft: number): string {
  return new Date(NOW + msLeft).toISOString()
}

function renderCountdown(msLeft: number, onExpire = vi.fn()) {
  render(
    <LabCountdown
      deadline={new Date(NOW + msLeft).toISOString()}
      now={() => Date.now()}
      onExpire={onExpire}
    />,
  )
  return onExpire
}

describe('LabCountdown', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(NOW)
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows the time left until the deadline', () => {
    renderCountdown(44 * MINUTE + 12 * 1000)

    expect(screen.getByRole('timer')).toHaveTextContent('44:12')
  })

  it('counts down as time passes', () => {
    renderCountdown(10 * MINUTE)

    act(() => {
      vi.advanceTimersByTime(3000)
    })

    expect(screen.getByRole('timer')).toHaveTextContent('09:57')
  })

  it('has no pause control', () => {
    renderCountdown(10 * MINUTE)

    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('uses the normal style above five minutes', () => {
    renderCountdown(10 * MINUTE)

    expect(screen.getByRole('timer')).toHaveAttribute('data-tone', 'normal')
  })

  it('switches to the warning style at five minutes left', () => {
    renderCountdown(5 * MINUTE + 2000)

    act(() => {
      vi.advanceTimersByTime(2000)
    })

    expect(screen.getByRole('timer')).toHaveAttribute('data-tone', 'warning')
  })

  it('switches to the urgent style at one minute left', () => {
    renderCountdown(1 * MINUTE + 2000)

    act(() => {
      vi.advanceTimersByTime(2000)
    })

    expect(screen.getByRole('timer')).toHaveAttribute('data-tone', 'urgent')
  })

  it('calls onExpire once when the clock reaches zero and not again afterwards', () => {
    const onExpire = renderCountdown(3000)

    act(() => {
      vi.advanceTimersByTime(2000)
    })
    expect(onExpire).not.toHaveBeenCalled()

    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(onExpire).toHaveBeenCalledTimes(1)

    act(() => {
      vi.advanceTimersByTime(10_000)
    })
    expect(onExpire).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('timer')).toHaveTextContent('00:00')
  })

  it('calls onExpire once when the deadline has already passed at mount', () => {
    const onExpire = renderCountdown(-5000)

    expect(screen.getByRole('timer')).toHaveTextContent('00:00')
    expect(onExpire).toHaveBeenCalledTimes(1)
  })

  it('styles each tone differently', () => {
    const { unmount } = render(
      <LabCountdown deadline={deadlineIn(10 * MINUTE)} now={() => Date.now()} onExpire={vi.fn()} />,
    )
    expect(screen.getByRole('timer')).toHaveClass('text-foreground')
    unmount()

    const warning = render(
      <LabCountdown deadline={deadlineIn(4 * MINUTE)} now={() => Date.now()} onExpire={vi.fn()} />,
    )
    expect(screen.getByRole('timer')).toHaveClass('text-amber-600')
    warning.unmount()

    render(<LabCountdown deadline={deadlineIn(30_000)} now={() => Date.now()} onExpire={vi.fn()} />)
    expect(screen.getByRole('timer')).toHaveClass('text-destructive', 'font-bold')
  })

  it('calls the latest onExpire when the handler changes before the deadline', () => {
    const first = vi.fn()
    const second = vi.fn()
    const now = () => Date.now()
    const { rerender } = render(
      <LabCountdown deadline={deadlineIn(3000)} now={now} onExpire={first} />,
    )

    rerender(<LabCountdown deadline={deadlineIn(3000)} now={now} onExpire={second} />)
    act(() => {
      vi.advanceTimersByTime(3000)
    })

    expect(first).not.toHaveBeenCalled()
    expect(second).toHaveBeenCalledOnce()
  })

  it('follows a new deadline', () => {
    const now = () => Date.now()
    const { rerender } = render(
      <LabCountdown deadline={deadlineIn(10 * MINUTE)} now={now} onExpire={vi.fn()} />,
    )

    rerender(<LabCountdown deadline={deadlineIn(2 * MINUTE)} now={now} onExpire={vi.fn()} />)
    act(() => {
      vi.advanceTimersByTime(1000)
    })

    expect(screen.getByRole('timer')).toHaveTextContent('01:59')
  })

  it('stops ticking when it unmounts', () => {
    const { unmount } = render(
      <LabCountdown deadline={deadlineIn(10 * MINUTE)} now={() => Date.now()} onExpire={vi.fn()} />,
    )
    expect(vi.getTimerCount()).toBe(1)

    unmount()

    expect(vi.getTimerCount()).toBe(0)
  })
})
