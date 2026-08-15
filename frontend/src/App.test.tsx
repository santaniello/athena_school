import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  HasLocalSession,
  HasOpenRouterKey,
  HasUserProfile,
  Login,
  Register,
  ResetLocalAccount,
  SaveOpenRouterKey,
  SaveProfile,
} from '../wailsjs/go/desktop/App'
import App from './App'

vi.mock('../wailsjs/go/desktop/App', () => ({
  HasLocalSession: vi.fn(),
  // Onboarding is already complete by default, so the existing auth tests
  // below reach the main screen exactly as before; tests that care about
  // the key gate / onboarding routing override these per-call.
  HasOpenRouterKey: vi.fn().mockResolvedValue(true),
  HasUserProfile: vi.fn().mockResolvedValue(true),
  Login: vi.fn(),
  Register: vi.fn(),
  ResetLocalAccount: vi.fn(),
  SaveOpenRouterKey: vi.fn(),
  SaveProfile: vi.fn(),
}))

describe('App', () => {
  it('shows the login screen when no local session exists', async () => {
    // Given no local session on disk
    vi.mocked(HasLocalSession).mockResolvedValueOnce(false)

    // When the app mounts
    render(<App />)

    // Then the login screen is shown, and no other screen is rendered alongside it
    expect(await screen.findByRole('heading', { name: 'Entrar' })).toBeInTheDocument()
    expect(
      screen.queryByRole('heading', { name: 'Conecte sua chave OpenRouter' }),
    ).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: /bem-vindo/i })).not.toBeInTheDocument()
  })

  it('skips straight to the main screen when a local session already exists', async () => {
    // Given a local session already exists on disk
    vi.mocked(HasLocalSession).mockResolvedValueOnce(true)

    // When the app mounts
    render(<App />)

    // Then it skips the login screen entirely, and no key gate is shown
    expect(await screen.findByRole('heading', { name: /bem-vindo/i })).toBeInTheDocument()
    expect(
      screen.queryByRole('heading', { name: 'Conecte sua chave OpenRouter' }),
    ).not.toBeInTheDocument()
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

  it('shows the OpenRouter key gate before the main screen when no key is configured', async () => {
    // Given an existing local session but no OpenRouter key yet
    vi.mocked(HasLocalSession).mockResolvedValueOnce(true)
    vi.mocked(HasOpenRouterKey).mockResolvedValueOnce(false)

    // When the app mounts
    render(<App />)

    // Then the key gate is shown instead of the main screen
    expect(
      await screen.findByRole('heading', { name: 'Conecte sua chave OpenRouter' }),
    ).toBeInTheDocument()
  })

  it('keeps the user on the key gate when the entered key is invalid', async () => {
    // Given an existing local session, no key configured yet, and a rejected key
    vi.mocked(HasLocalSession).mockResolvedValueOnce(true)
    vi.mocked(HasOpenRouterKey).mockResolvedValueOnce(false)
    vi.mocked(SaveOpenRouterKey).mockRejectedValueOnce(
      new Error('openrouter key is invalid or unauthorized'),
    )
    const user = userEvent.setup()
    render(<App />)
    await screen.findByRole('heading', { name: 'Conecte sua chave OpenRouter' })

    // When submitting an invalid key
    await user.type(screen.getByLabelText('Chave da OpenRouter'), 'sk-or-invalid')
    await user.click(screen.getByRole('button', { name: 'Conectar' }))

    // Then the user stays on the key gate with an inline error
    expect(await screen.findByText('Chave inválida ou não autorizada.')).toBeInTheDocument()
    expect(
      screen.getByRole('heading', { name: 'Conecte sua chave OpenRouter' }),
    ).toBeInTheDocument()
  })

  it('shows onboarding after the key is saved when no profile exists yet', async () => {
    // Given an existing local session, no key configured yet, and no profile
    vi.mocked(HasLocalSession).mockResolvedValueOnce(true)
    vi.mocked(HasOpenRouterKey).mockResolvedValueOnce(false)
    vi.mocked(HasUserProfile).mockResolvedValueOnce(false)
    vi.mocked(SaveOpenRouterKey).mockResolvedValueOnce(undefined)
    const user = userEvent.setup()
    render(<App />)
    await screen.findByRole('heading', { name: 'Conecte sua chave OpenRouter' })

    // When submitting a valid key
    await user.type(screen.getByLabelText('Chave da OpenRouter'), 'sk-or-valid')
    await user.click(screen.getByRole('button', { name: 'Conectar' }))

    // Then the onboarding form is shown next
    expect(await screen.findByRole('heading', { name: 'Conte sobre você' })).toBeInTheDocument()
  })

  it('navigates from register back to login', async () => {
    // Given no local session
    vi.mocked(HasLocalSession).mockResolvedValueOnce(false)
    const user = userEvent.setup()
    render(<App />)
    await screen.findByRole('heading', { name: 'Entrar' })

    // When going to the register screen and back to login
    await user.click(screen.getByRole('button', { name: 'Criar conta' }))
    await screen.findByRole('heading', { name: 'Criar conta' })
    await user.click(screen.getByRole('button', { name: 'Já tenho conta' }))

    // Then the login screen is shown again
    expect(await screen.findByRole('heading', { name: 'Entrar' })).toBeInTheDocument()
  })

  it('cancels out of the reset flow back to login', async () => {
    // Given no local session
    vi.mocked(HasLocalSession).mockResolvedValueOnce(false)
    const user = userEvent.setup()
    render(<App />)
    await screen.findByRole('heading', { name: 'Entrar' })

    // When going to the reset screen and cancelling
    await user.click(screen.getByRole('button', { name: 'Esqueci minha senha' }))
    await screen.findByLabelText('E-mail')
    await user.click(screen.getByRole('button', { name: 'Cancelar' }))

    // Then the login screen is shown again
    expect(await screen.findByRole('heading', { name: 'Entrar' })).toBeInTheDocument()
  })

  it('completes onboarding end-to-end and reaches the main screen', async () => {
    // Given an existing local session, no key, and no profile
    vi.mocked(HasLocalSession).mockResolvedValueOnce(true)
    vi.mocked(HasOpenRouterKey).mockResolvedValueOnce(false)
    vi.mocked(HasUserProfile).mockResolvedValueOnce(false)
    vi.mocked(SaveOpenRouterKey).mockResolvedValueOnce(undefined)
    vi.mocked(SaveProfile).mockResolvedValueOnce()
    const user = userEvent.setup()
    render(<App />)
    await screen.findByRole('heading', { name: 'Conecte sua chave OpenRouter' })

    // When connecting the key and filling out onboarding
    await user.type(screen.getByLabelText('Chave da OpenRouter'), 'sk-or-valid')
    await user.click(screen.getByRole('button', { name: 'Conectar' }))
    await screen.findByRole('heading', { name: 'Conte sobre você' })
    await user.type(screen.getByLabelText('Nome'), 'Ana')
    await user.type(screen.getByLabelText('Como quer chamar o assistente?'), 'Atena')
    await user.type(screen.getByLabelText('Área de atuação ou estudo'), 'Engenharia de Software')
    await user.type(screen.getByLabelText('Foco específico'), 'Backend')
    await user.click(screen.getByRole('combobox', { name: 'Nível de experiência' }))
    await user.click(await screen.findByRole('option', { name: 'Intermediário' }))
    await user.type(screen.getByLabelText('Objetivos'), 'SQL{Enter}')
    await user.type(screen.getByLabelText('Estilo de estudo preferido'), 'Prática')
    await user.click(screen.getByRole('button', { name: 'Continuar' }))
    await screen.findByRole('heading', { name: 'Confirme seu perfil' })
    await user.click(screen.getByRole('button', { name: 'Confirmar e salvar' }))

    // Then the app reaches the main screen
    expect(await screen.findByRole('heading', { name: /bem-vindo/i })).toBeInTheDocument()
  })

  it('skips the key gate and onboarding for a returning user with both already set up', async () => {
    // Given an existing local session, an existing key, and an existing profile
    vi.mocked(HasLocalSession).mockResolvedValueOnce(true)
    vi.mocked(HasOpenRouterKey).mockResolvedValueOnce(true)
    vi.mocked(HasUserProfile).mockResolvedValueOnce(true)

    // When the app mounts
    render(<App />)

    // Then it goes straight to the main screen
    expect(await screen.findByRole('heading', { name: /bem-vindo/i })).toBeInTheDocument()
  })
})
