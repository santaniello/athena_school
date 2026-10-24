import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { EmbeddingChangeDialog } from './embedding-change-dialog'

function renderDialog(overrides: Partial<React.ComponentProps<typeof EmbeddingChangeDialog>> = {}) {
  const onCancel = vi.fn()
  const onConfirm = vi.fn()
  render(
    <EmbeddingChangeDialog
      open
      pending={false}
      from="openai/text-embedding-3-small"
      to="openai/text-embedding-3-large"
      onCancel={onCancel}
      onConfirm={onConfirm}
      {...overrides}
    />,
  )
  return { onCancel, onConfirm }
}

describe('EmbeddingChangeDialog', () => {
  it('renders nothing while closed', () => {
    renderDialog({ open: false })
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('names the model change and lists every consequence', () => {
    renderDialog()

    expect(
      screen.getByRole('alertdialog', { name: 'Change the embedding model?' }),
    ).toBeInTheDocument()
    expect(screen.getByText('openai/text-embedding-3-small')).toBeInTheDocument()
    expect(screen.getByText('openai/text-embedding-3-large')).toBeInTheDocument()
    expect(
      screen.getByText(
        'Retrieval will stop returning your existing notes until every note is re-imported.',
      ),
    ).toBeInTheDocument()
    expect(screen.getByText('Re-importing consumes API credits.')).toBeInTheDocument()
    expect(
      screen.getByText('Until then, answers will not be grounded in your notes.'),
    ).toBeInTheDocument()
    expect(
      screen.getByText('Switching back restores the previous index without re-importing.'),
    ).toBeInTheDocument()
  })

  it('confirms only through the explicit button', async () => {
    const user = userEvent.setup()
    const { onConfirm, onCancel } = renderDialog()

    await user.click(screen.getByRole('button', { name: 'Change model' }))

    expect(onConfirm).toHaveBeenCalledOnce()
    expect(onCancel).not.toHaveBeenCalled()
  })

  it('cancels with the Cancel button, leaving the old value to the caller', async () => {
    const user = userEvent.setup()
    const { onConfirm, onCancel } = renderDialog()

    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(onCancel).toHaveBeenCalledOnce()
    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('cancels on Escape', async () => {
    const user = userEvent.setup()
    const { onCancel } = renderDialog()

    await user.keyboard('{Escape}')

    expect(onCancel).toHaveBeenCalledOnce()
  })

  it('shows a save error inside the dialog', () => {
    renderDialog({ error: 'An error occurred. Please try again.' })

    expect(screen.getByRole('alert')).toHaveTextContent('An error occurred. Please try again.')
  })

  it('shows no error alert by default', () => {
    renderDialog()

    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('locks both buttons and cannot be dismissed while the change is being saved', async () => {
    const user = userEvent.setup()
    const { onCancel } = renderDialog({ pending: true })

    expect(screen.getByRole('button', { name: 'Changing...' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled()
    await user.keyboard('{Escape}')
    expect(onCancel).not.toHaveBeenCalled()
  })
})
