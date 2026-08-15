import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SaveOpenRouterKey } from '../../wailsjs/go/desktop/App'
import { OpenRouterKeyForm } from './openrouter-key-form'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  SaveOpenRouterKey: vi.fn(),
}))

describe('OpenRouterKeyForm', () => {
  it('shows no error alert before any submission', () => {
    // Given a freshly rendered form
    render(<OpenRouterKeyForm onSaved={vi.fn()} />)

    // Then no error alert is shown
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('clears a previous error once a later submission succeeds', async () => {
    // Given a first submission that fails, followed by one that succeeds
    vi.mocked(SaveOpenRouterKey)
      .mockRejectedValueOnce(new Error('openrouter key is invalid or unauthorized'))
      .mockResolvedValueOnce()
    const user = userEvent.setup()
    render(<OpenRouterKeyForm onSaved={vi.fn()} />)
    await user.type(screen.getByLabelText('OpenRouter key'), 'sk-or-invalid')
    await user.click(screen.getByRole('button', { name: 'Connect' }))
    expect(await screen.findByRole('alert')).toBeInTheDocument()

    // When submitting again
    await user.click(screen.getByRole('button', { name: 'Connect' }))

    // Then the error alert is gone
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
  })

  it('saves the entered key and reports success', async () => {
    // Given a SaveOpenRouterKey call that succeeds
    vi.mocked(SaveOpenRouterKey).mockResolvedValueOnce()
    const onSaved = vi.fn()
    const user = userEvent.setup()
    render(<OpenRouterKeyForm onSaved={onSaved} />)

    // When the user submits a key
    await user.type(screen.getByLabelText('OpenRouter key'), 'sk-or-valid')
    await user.click(screen.getByRole('button', { name: 'Connect' }))

    // Then the binding is called with that key and onSaved fires
    expect(SaveOpenRouterKey).toHaveBeenCalledWith('sk-or-valid')
    await waitFor(() => expect(onSaved).toHaveBeenCalledOnce())
  })

  it('disables the submit button and shows a validating label while pending', async () => {
    // Given a SaveOpenRouterKey call that never resolves during the assertion
    let resolveCall: () => void = () => {}
    vi.mocked(SaveOpenRouterKey).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveCall = resolve
      }),
    )
    const user = userEvent.setup()
    render(<OpenRouterKeyForm onSaved={vi.fn()} />)

    // When the user submits a key
    await user.type(screen.getByLabelText('OpenRouter key'), 'sk-or-valid')
    await user.click(screen.getByRole('button', { name: 'Connect' }))

    // Then the button is disabled and shows a validating label
    const button = await screen.findByRole('button', { name: 'Validating...' })
    expect(button).toBeDisabled()

    resolveCall()
  })

  it('shows an inline error and does not call onSaved on an invalid key', async () => {
    // Given a SaveOpenRouterKey call that rejects
    vi.mocked(SaveOpenRouterKey).mockRejectedValueOnce(
      new Error('openrouter key is invalid or unauthorized'),
    )
    const onSaved = vi.fn()
    const user = userEvent.setup()
    render(<OpenRouterKeyForm onSaved={onSaved} />)

    // When the user submits an invalid key
    await user.type(screen.getByLabelText('OpenRouter key'), 'sk-or-invalid')
    await user.click(screen.getByRole('button', { name: 'Connect' }))

    // Then an inline error is shown and onSaved never fires
    expect(await screen.findByText('Invalid or unauthorized key.')).toBeInTheDocument()
    expect(onSaved).not.toHaveBeenCalled()
  })
})
