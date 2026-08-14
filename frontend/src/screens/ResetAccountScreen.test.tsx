import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ResetLocalAccount } from '../../wailsjs/go/desktop/App'
import ResetAccountScreen from './ResetAccountScreen'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  ResetLocalAccount: vi.fn(),
}))

describe('ResetAccountScreen', () => {
  it('explains that this deletes the local account, not a real recovery', () => {
    // Given the reset screen is rendered
    render(<ResetAccountScreen onDone={vi.fn()} onCancel={vi.fn()} />)

    // Then it explains the destructive, non-recovery nature of the action
    expect(screen.getByText(/apaga a conta local/i)).toBeInTheDocument()
    expect(screen.getByText(/não é uma recuperação/i)).toBeInTheDocument()
  })

  it('deletes the local account and returns to the create-account screen', async () => {
    // Given a ResetLocalAccount call that succeeds
    vi.mocked(ResetLocalAccount).mockResolvedValueOnce(undefined)
    const onDone = vi.fn()
    const user = userEvent.setup()
    render(<ResetAccountScreen onDone={onDone} onCancel={vi.fn()} />)

    // When the user confirms the reset for their email
    await user.type(screen.getByLabelText('E-mail'), 'user@athena.dev')
    await user.click(screen.getByRole('button', { name: 'Excluir conta local' }))

    // Then the account is removed and the caller is told to go back to Register
    expect(ResetLocalAccount).toHaveBeenCalledWith('user@athena.dev')
    await waitFor(() => expect(onDone).toHaveBeenCalledOnce())
  })

  it('shows an inline error when the account does not exist', async () => {
    // Given a ResetLocalAccount call that rejects with the account-not-found sentinel
    vi.mocked(ResetLocalAccount).mockRejectedValueOnce(new Error('account not found'))
    const user = userEvent.setup()
    render(<ResetAccountScreen onDone={vi.fn()} onCancel={vi.fn()} />)

    // When the user confirms the reset for an unknown email
    await user.type(screen.getByLabelText('E-mail'), 'missing@athena.dev')
    await user.click(screen.getByRole('button', { name: 'Excluir conta local' }))

    // Then an inline PT-BR error message is shown
    expect(await screen.findByText('Nenhuma conta encontrada com este e-mail.')).toBeInTheDocument()
  })

  it('cancels back to the login screen', async () => {
    // Given a rendered reset screen
    const onCancel = vi.fn()
    const user = userEvent.setup()
    render(<ResetAccountScreen onDone={vi.fn()} onCancel={onCancel} />)

    // When the user clicks cancel
    await user.click(screen.getByRole('button', { name: 'Cancelar' }))

    // Then the callback fires
    expect(onCancel).toHaveBeenCalledOnce()
  })
})
