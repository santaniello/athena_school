import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { LabConfirmDialog } from './lab-confirm-dialog'

function renderDialog(open = true) {
  const handlers = { onCancel: vi.fn(), onConfirm: vi.fn() }
  render(
    <LabConfirmDialog
      open={open}
      title="Finish the exam?"
      description="Finish now? You cannot edit the diagram after this."
      confirmLabel="Finish"
      {...handlers}
    />,
  )
  return handlers
}

describe('LabConfirmDialog', () => {
  it('is closed when open is false', () => {
    renderDialog(false)

    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('shows the title, the description and the two buttons', () => {
    renderDialog()

    expect(screen.getByRole('alertdialog', { name: 'Finish the exam?' })).toBeInTheDocument()
    expect(
      screen.getByText('Finish now? You cannot edit the diagram after this.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Finish' })).toBeInTheDocument()
  })

  it('confirms only from the confirm button', async () => {
    const user = userEvent.setup()
    const handlers = renderDialog()

    await user.click(screen.getByRole('button', { name: 'Finish' }))

    expect(handlers.onConfirm).toHaveBeenCalledOnce()
    expect(handlers.onCancel).not.toHaveBeenCalled()
  })

  it('cancels from Cancel and from Escape, never confirming', async () => {
    const user = userEvent.setup()
    const handlers = renderDialog()

    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    await user.keyboard('{Escape}')

    expect(handlers.onCancel).toHaveBeenCalledTimes(2)
    expect(handlers.onConfirm).not.toHaveBeenCalled()
  })
})
