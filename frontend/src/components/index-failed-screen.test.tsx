import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { IndexFailedScreen } from './index-failed-screen'

describe('IndexFailedScreen', () => {
  it('shows the failure message and both recovery actions', () => {
    // Given the initial background load failed
    render(
      <IndexFailedScreen lastError="" retrying={false} onRetry={vi.fn()} onContinue={vi.fn()} />,
    )

    // Then the failure message and both choices are shown
    expect(screen.getByText('Knowledge index could not be loaded.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Continue without local search' }),
    ).toBeInTheDocument()
  })

  it('renders no error paragraph at all when there is no summary', () => {
    // Given a failure with no lastError (an empty string)
    const { container } = render(
      <IndexFailedScreen lastError="" retrying={false} onRetry={vi.fn()} onContinue={vi.fn()} />,
    )

    // Then there is exactly one paragraph — the fixed failure message —
    // not a second, empty one for the absent summary. querySelectorAll
    // (not getAllByText, which a truly empty <p> would never match either
    // way) is what actually distinguishes the two cases.
    expect(container.querySelectorAll('p')).toHaveLength(1)
  })

  it('shows the safe error summary when one is provided', () => {
    // Given a failure with a safe (non-raw) summary
    render(
      <IndexFailedScreen
        lastError="Could not load the knowledge index from the database."
        retrying={false}
        onRetry={vi.fn()}
        onContinue={vi.fn()}
      />,
    )

    // Then it is shown to the user
    expect(
      screen.getByText('Could not load the knowledge index from the database.'),
    ).toBeInTheDocument()
  })

  it('calls onRetry when Retry is clicked', async () => {
    // Given the failed screen with a retry handler
    const user = userEvent.setup()
    const onRetry = vi.fn()
    render(
      <IndexFailedScreen lastError="" retrying={false} onRetry={onRetry} onContinue={vi.fn()} />,
    )

    // When clicking Retry
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    // Then the handler is invoked
    expect(onRetry).toHaveBeenCalledOnce()
  })

  it('calls onContinue when Continue without local search is clicked', async () => {
    // Given the failed screen with a continue handler
    const user = userEvent.setup()
    const onContinue = vi.fn()
    render(
      <IndexFailedScreen lastError="" retrying={false} onRetry={vi.fn()} onContinue={onContinue} />,
    )

    // When clicking Continue without local search
    await user.click(screen.getByRole('button', { name: 'Continue without local search' }))

    // Then the handler is invoked
    expect(onContinue).toHaveBeenCalledOnce()
  })

  it('disables both actions while a retry is in flight', () => {
    // Given a retry currently running
    render(
      <IndexFailedScreen lastError="" retrying={true} onRetry={vi.fn()} onContinue={vi.fn()} />,
    )

    // Then both actions are disabled so the user can't fire a second retry
    // or bail out mid-attempt
    expect(screen.getByRole('button', { name: 'Retry' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Continue without local search' })).toBeDisabled()
  })
})
