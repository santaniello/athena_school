import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Login } from '../../wailsjs/go/desktop/App'
import LoginScreen from './LoginScreen'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  Login: vi.fn(),
}))

describe('LoginScreen', () => {
  it('logs in with the entered credentials and reports success', async () => {
    // Given a Login call that succeeds
    const loginResult = { accountId: 'acc-1', email: 'user@athena.dev' }
    vi.mocked(Login).mockResolvedValueOnce(loginResult)
    const onSuccess = vi.fn()
    const user = userEvent.setup()
    render(
      <LoginScreen
        onSuccess={onSuccess}
        onNavigateToRegister={vi.fn()}
        onNavigateToReset={vi.fn()}
      />,
    )

    // When the user submits the login form
    await user.type(screen.getByLabelText('Email'), 'user@athena.dev')
    await user.type(screen.getByLabelText('Password'), 's3cr3t-password')
    await user.click(screen.getByRole('button', { name: 'Log in' }))

    // Then Login is called with the entered values and onSuccess fires
    expect(Login).toHaveBeenCalledWith('user@athena.dev', 's3cr3t-password')
    await waitFor(() => expect(onSuccess).toHaveBeenCalledWith(loginResult))
  })

  it('shows an inline error and no crash on invalid credentials', async () => {
    // Given a Login call that rejects with the invalid-credentials sentinel
    vi.mocked(Login).mockRejectedValueOnce(new Error('invalid credentials'))
    const user = userEvent.setup()
    render(
      <LoginScreen
        onSuccess={vi.fn()}
        onNavigateToRegister={vi.fn()}
        onNavigateToReset={vi.fn()}
      />,
    )

    // When the user submits the login form
    await user.type(screen.getByLabelText('Email'), 'user@athena.dev')
    await user.type(screen.getByLabelText('Password'), 'wrong-password')
    await user.click(screen.getByRole('button', { name: 'Log in' }))

    // Then an inline error message is shown
    expect(await screen.findByText('Invalid email or password.')).toBeInTheDocument()
  })

  it('navigates to the register and reset screens', async () => {
    // Given a rendered login screen
    const onNavigateToRegister = vi.fn()
    const onNavigateToReset = vi.fn()
    const user = userEvent.setup()
    render(
      <LoginScreen
        onSuccess={vi.fn()}
        onNavigateToRegister={onNavigateToRegister}
        onNavigateToReset={onNavigateToReset}
      />,
    )

    // When the user clicks the navigation actions
    await user.click(screen.getByRole('button', { name: 'Create account' }))
    await user.click(screen.getByRole('button', { name: 'Forgot my password' }))

    // Then the corresponding callbacks fire
    expect(onNavigateToRegister).toHaveBeenCalledOnce()
    expect(onNavigateToReset).toHaveBeenCalledOnce()
  })
})
