import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { FindBar } from './find-bar'

function renderBar(overrides: Partial<React.ComponentProps<typeof FindBar>> = {}) {
  const props = {
    query: '',
    matchCount: 0,
    activeIndex: -1,
    loadingHistory: false,
    inputRef: { current: null },
    onQueryChange: vi.fn(),
    onNext: vi.fn(),
    onPrev: vi.fn(),
    onClose: vi.fn(),
    ...overrides,
  }
  render(<FindBar {...props} />)
  return props
}

describe('FindBar', () => {
  it('shows nothing about results while the query is empty', () => {
    renderBar()
    expect(screen.queryByText(/No results/)).toBeNull()
    expect(screen.queryByText(/ of /)).toBeNull()
  })

  it('shows the position of the active match', () => {
    renderBar({ query: 'a', matchCount: 5, activeIndex: 1 })
    expect(screen.getByText('2 of 5')).toBeInTheDocument()
  })

  it('shows "No results" when the query matches nothing', () => {
    renderBar({ query: 'zzz' })
    expect(screen.getByText('No results')).toBeInTheDocument()
  })

  it('shows that the history is still loading', () => {
    renderBar({ loadingHistory: true })
    expect(screen.getByText('Loading history…')).toBeInTheDocument()
  })

  it('shows a warning when the history could not be fully loaded', () => {
    renderBar({ historyError: 'boom' })
    expect(screen.getByText('Could not load the full history')).toBeInTheDocument()
  })

  it('reports typed text', async () => {
    const props = renderBar()
    await userEvent.type(screen.getByRole('textbox', { name: 'Find in conversation' }), 'x')
    expect(props.onQueryChange).toHaveBeenCalledWith('x')
  })

  it('goes to the next match on Enter and to the previous on Shift+Enter', async () => {
    const props = renderBar()
    await userEvent.click(screen.getByRole('textbox', { name: 'Find in conversation' }))
    await userEvent.keyboard('{Enter}')
    expect(props.onNext).toHaveBeenCalledTimes(1)
    await userEvent.keyboard('{Shift>}{Enter}{/Shift}')
    expect(props.onPrev).toHaveBeenCalledTimes(1)
  })

  it('closes on Escape', async () => {
    const props = renderBar()
    await userEvent.click(screen.getByRole('textbox', { name: 'Find in conversation' }))
    await userEvent.keyboard('{Escape}')
    expect(props.onClose).toHaveBeenCalledTimes(1)
  })

  it('navigates and closes with its buttons', async () => {
    const props = renderBar({ query: 'a', matchCount: 2, activeIndex: 0 })
    await userEvent.click(screen.getByRole('button', { name: 'Next match' }))
    await userEvent.click(screen.getByRole('button', { name: 'Previous match' }))
    await userEvent.click(screen.getByRole('button', { name: 'Close find' }))
    expect(props.onNext).toHaveBeenCalledTimes(1)
    expect(props.onPrev).toHaveBeenCalledTimes(1)
    expect(props.onClose).toHaveBeenCalledTimes(1)
  })

  it('disables the navigation buttons without matches', () => {
    renderBar({ query: 'zzz' })
    expect(screen.getByRole('button', { name: 'Next match' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Previous match' })).toBeDisabled()
  })
})
