import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Login, Register } from '../../wailsjs/go/desktop/App'
import RegisterScreen from './RegisterScreen'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  Register: vi.fn(),
  Login: vi.fn(),
}))

describe('RegisterScreen', () => {
  it('registers and logs in with the entered credentials, landing directly in the app', async () => {
    // Given a Register call that succeeds, followed by a successful Login
    vi.mocked(Register).mockResolvedValueOnce(undefined)
    const loginResult = { accountId: 'acc-1', email: 'new@athena.dev' }
    vi.mocked(Login).mockResolvedValueOnce(loginResult)
    const onSuccess = vi.fn()
    const user = userEvent.setup()
    render(<RegisterScreen onSuccess={onSuccess} onNavigateToLogin={vi.fn()} />)

    // When the user submits the register form with matching passwords
    await user.type(screen.getByLabelText('Email'), 'new@athena.dev')
    await user.type(screen.getByLabelText('Password'), 's3cr3t-password')
    await user.type(screen.getByLabelText('Confirm password'), 's3cr3t-password')
    await user.click(screen.getByRole('button', { name: 'Create account' }))

    // Then the account is created, the user is logged in, and no email-confirmation step is shown
    expect(Register).toHaveBeenCalledWith('new@athena.dev', 's3cr3t-password')
    await waitFor(() => expect(Login).toHaveBeenCalledWith('new@athena.dev', 's3cr3t-password'))
    await waitFor(() => expect(onSuccess).toHaveBeenCalledWith(loginResult))
  })

  it('shows an inline error and does not call Register when passwords do not match', async () => {
    // Given a rendered register screen
    const user = userEvent.setup()
    render(<RegisterScreen onSuccess={vi.fn()} onNavigateToLogin={vi.fn()} />)

    // When the user submits mismatched passwords
    await user.type(screen.getByLabelText('Email'), 'new@athena.dev')
    await user.type(screen.getByLabelText('Password'), 's3cr3t-password')
    await user.type(screen.getByLabelText('Confirm password'), 'different-password')
    await user.click(screen.getByRole('button', { name: 'Create account' }))

    // Then an inline error is shown and Register is never called
    expect(await screen.findByText("Passwords don't match.")).toBeInTheDocument()
    expect(Register).not.toHaveBeenCalled()
  })

  it('shows an inline error on duplicate email', async () => {
    // Given a Register call that rejects with the duplicate-email sentinel
    vi.mocked(Register).mockRejectedValueOnce(new Error('email already exists'))
    const user = userEvent.setup()
    render(<RegisterScreen onSuccess={vi.fn()} onNavigateToLogin={vi.fn()} />)

    // When the user submits the register form
    await user.type(screen.getByLabelText('Email'), 'dup@athena.dev')
    await user.type(screen.getByLabelText('Password'), 's3cr3t-password')
    await user.type(screen.getByLabelText('Confirm password'), 's3cr3t-password')
    await user.click(screen.getByRole('button', { name: 'Create account' }))

    // Then an inline error message is shown
    expect(await screen.findByText('An account with this email already exists.')).toBeInTheDocument()
  })

  it('navigates back to the login screen', async () => {
    // Given a rendered register screen
    const onNavigateToLogin = vi.fn()
    const user = userEvent.setup()
    render(<RegisterScreen onSuccess={vi.fn()} onNavigateToLogin={onNavigateToLogin} />)

    // When the user clicks the back-to-login action
    await user.click(screen.getByRole('button', { name: 'I already have an account' }))

    // Then the callback fires
    expect(onNavigateToLogin).toHaveBeenCalledOnce()
  })
})
