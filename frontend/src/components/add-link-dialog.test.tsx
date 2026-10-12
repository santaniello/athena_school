import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AddLinkDialog } from './add-link-dialog'

describe('AddLinkDialog', () => {
  it('renders nothing while closed', () => {
    // Given a closed dialog
    // When it renders
    render(<AddLinkDialog open={false} onCancel={vi.fn()} onSubmit={vi.fn()} />)

    // Then no URL field is on screen
    expect(screen.queryByLabelText('Link URL')).not.toBeInTheDocument()
  })

  it('keeps Continue disabled until the text looks like an http(s) URL', async () => {
    // Given an open dialog
    const user = userEvent.setup()
    render(<AddLinkDialog open onCancel={vi.fn()} onSubmit={vi.fn()} />)
    const continueButton = screen.getByRole('button', { name: 'Continue' })
    expect(continueButton).toBeDisabled()

    // When typing something that is not a URL
    await user.type(screen.getByLabelText('Link URL'), 'not a link')

    // Then Continue stays disabled
    expect(continueButton).toBeDisabled()

    // And when the field holds an http(s) URL
    await user.clear(screen.getByLabelText('Link URL'))
    await user.type(screen.getByLabelText('Link URL'), 'https://example.com/article')

    // Then Continue is enabled
    expect(continueButton).toBeEnabled()
  })

  it('submits the trimmed URL when Continue is clicked', async () => {
    // Given an open dialog with a pasted URL surrounded by spaces
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<AddLinkDialog open onCancel={vi.fn()} onSubmit={onSubmit} />)
    await user.type(screen.getByLabelText('Link URL'), '  https://example.com/article  ')

    // When clicking Continue
    await user.click(screen.getByRole('button', { name: 'Continue' }))

    // Then the trimmed URL is submitted
    expect(onSubmit).toHaveBeenCalledExactlyOnceWith('https://example.com/article')
  })

  it('submits with the Enter key', async () => {
    // Given an open dialog with a valid URL typed
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<AddLinkDialog open onCancel={vi.fn()} onSubmit={onSubmit} />)

    // When pressing Enter in the field
    await user.type(screen.getByLabelText('Link URL'), 'https://example.com/a{Enter}')

    // Then the URL is submitted
    expect(onSubmit).toHaveBeenCalledExactlyOnceWith('https://example.com/a')
  })

  it('does not submit an invalid URL with Enter', async () => {
    // Given an open dialog with an invalid value typed
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<AddLinkDialog open onCancel={vi.fn()} onSubmit={onSubmit} />)

    // When pressing Enter
    await user.type(screen.getByLabelText('Link URL'), 'nope{Enter}')

    // Then nothing is submitted
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('calls onCancel when Cancel is clicked', async () => {
    // Given an open dialog
    const user = userEvent.setup()
    const onCancel = vi.fn()
    render(<AddLinkDialog open onCancel={onCancel} onSubmit={vi.fn()} />)

    // When clicking Cancel
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    // Then the dialog asks to close
    expect(onCancel).toHaveBeenCalledOnce()
  })
})
