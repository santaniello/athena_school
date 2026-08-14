import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { HasLocalSession, Login, Register, ResetLocalAccount } from '../wailsjs/go/desktop/App'
import App from './App'

vi.mock('../wailsjs/go/desktop/App', () => ({
  HasLocalSession: vi.fn(),
  Login: vi.fn(),
  Register: vi.fn(),
  ResetLocalAccount: vi.fn(),
}))

describe('App', () => {
  it('shows the login screen when no local session exists', async () => {
    // Given no local session on disk
    vi.mocked(HasLocalSession).mockResolvedValueOnce(false)

    // When the app mounts
    render(<App />)

    // Then the login screen is shown
    expect(await screen.findByRole('heading', { name: 'Entrar' })).toBeInTheDocument()
  })

  it('skips straight to the main screen when a local session already exists', async () => {
    // Given a local session already exists on disk
    vi.mocked(HasLocalSession).mockResolvedValueOnce(true)

    // When the app mounts
    render(<App />)

    // Then it skips the login screen entirely
    expect(await screen.findByRole('heading', { name: /bem-vindo/i })).toBeInTheDocument()
  })

  it('takes a new user from registration straight into the app, no email-confirmation step', async () => {
    // Given no local session, and Register/Login both succeed
    vi.mocked(HasLocalSession).mockResolvedValueOnce(false)
    vi.mocked(Register).mockResolvedValueOnce(undefined)
    vi.mocked(Login).mockResolvedValueOnce({ accountId: 'acc-1', email: 'new@athena.dev' })
    const user = userEvent.setup()
    render(<App />)
    await screen.findByRole('heading', { name: 'Entrar' })

    // When the user goes to the register screen and creates an account
    await user.click(screen.getByRole('button', { name: 'Criar conta' }))
    await user.type(screen.getByLabelText('E-mail'), 'new@athena.dev')
    await user.type(screen.getByLabelText('Senha'), 's3cr3t-password')
    await user.type(screen.getByLabelText('Confirmar senha'), 's3cr3t-password')
    await user.click(screen.getByRole('button', { name: 'Criar conta' }))

    // Then the user lands directly on the main screen
    expect(await screen.findByText('new@athena.dev')).toBeInTheDocument()
  })

  it('logs an existing user in and reaches the main screen', async () => {
    // Given no local session, and Login succeeds
    vi.mocked(HasLocalSession).mockResolvedValueOnce(false)
    vi.mocked(Login).mockResolvedValueOnce({ accountId: 'acc-1', email: 'user@athena.dev' })
    const user = userEvent.setup()
    render(<App />)
    await screen.findByRole('heading', { name: 'Entrar' })

    // When the user logs in
    await user.type(screen.getByLabelText('E-mail'), 'user@athena.dev')
    await user.type(screen.getByLabelText('Senha'), 's3cr3t-password')
    await user.click(screen.getByRole('button', { name: 'Entrar' }))

    // Then the user reaches the main screen
    expect(await screen.findByText('user@athena.dev')).toBeInTheDocument()
  })

  it('resets the local account and returns to the create-account screen', async () => {
    // Given no local session, and ResetLocalAccount succeeds
    vi.mocked(HasLocalSession).mockResolvedValueOnce(false)
    vi.mocked(ResetLocalAccount).mockResolvedValueOnce(undefined)
    const user = userEvent.setup()
    render(<App />)
    await screen.findByRole('heading', { name: 'Entrar' })

    // When the user goes through the reset flow
    await user.click(screen.getByRole('button', { name: 'Esqueci minha senha' }))
    await user.type(screen.getByLabelText('E-mail'), 'user@athena.dev')
    await user.click(screen.getByRole('button', { name: 'Excluir conta local' }))

    // Then the app returns to the create-account screen
    expect(await screen.findByRole('heading', { name: 'Criar conta' })).toBeInTheDocument()
  })
})
