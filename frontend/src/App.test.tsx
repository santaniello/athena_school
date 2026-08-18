import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
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

// Opening a Radix Select makes it pull focus into its listbox. When a text
// input currently holds focus, that blur/focus pair lands outside React's
// act scope under jsdom, and React reports an un-acted update on SelectItem
// (its onFocus sets isFocused). Dropping focus first means Radix's move
// starts from a settled state — a real browser's click does this on its own,
// so this only compensates for jsdom, not for a product bug.
//
// Waiting for the trigger to show the label additionally asserts the pick
// actually took, which clicking an option alone does not.
async function pickOption(
  user: ReturnType<typeof userEvent.setup>,
  comboboxName: string,
  optionName: string,
) {
  ;(document.activeElement as HTMLElement | null)?.blur()
  const combobox = screen.getByRole('combobox', { name: comboboxName })
  await user.click(combobox)
  await user.click(await screen.findByRole('option', { name: optionName }))
  await waitFor(() => expect(combobox).toHaveTextContent(optionName))
}

vi.mock('../wailsjs/go/desktop/App', () => ({
  HasLocalSession: vi.fn(),
  // Onboarding is already complete by default, so the existing auth tests
  // below reach the app shell exactly as before; tests that care about
  // the key gate / onboarding routing override these per-call.
  HasOpenRouterKey: vi.fn().mockResolvedValue(true),
  HasUserProfile: vi.fn().mockResolvedValue(true),
  GetProfile: vi.fn().mockResolvedValue({
    name: 'Ana',
    assistantName: 'Athena',
    area: '',
    experienceLevel: '',
    goals: [],
    studyStyle: '',
    assistantLanguage: '',
  }),
  Login: vi.fn(),
  Logout: vi.fn(),
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
    expect(await screen.findByRole('heading', { name: 'Log in' })).toBeInTheDocument()
    expect(
      screen.queryByRole('heading', { name: 'Connect your OpenRouter key' }),
    ).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Home', level: 1 })).not.toBeInTheDocument()
  })

  it('skips straight to the app shell when a local session already exists', async () => {
    // Given a local session already exists on disk
    vi.mocked(HasLocalSession).mockResolvedValueOnce(true)

    // When the app mounts
    render(<App />)

    // Then it skips the login screen entirely, and no key gate is shown
    expect(await screen.findByRole('heading', { name: 'Home', level: 1 })).toBeInTheDocument()
    expect(
      screen.queryByRole('heading', { name: 'Connect your OpenRouter key' }),
    ).not.toBeInTheDocument()
  })

  it('takes a new user from registration straight into the app, no email-confirmation step', async () => {
    // Given no local session, and Register/Login both succeed
    vi.mocked(HasLocalSession).mockResolvedValueOnce(false)
    vi.mocked(Register).mockResolvedValueOnce(undefined)
    vi.mocked(Login).mockResolvedValueOnce({ accountId: 'acc-1', email: 'new@athena.dev' })
    const user = userEvent.setup()
    render(<App />)
    await screen.findByRole('heading', { name: 'Log in' })

    // When the user goes to the register screen and creates an account
    await user.click(screen.getByRole('button', { name: 'Create account' }))
    await user.type(screen.getByLabelText('Email'), 'new@athena.dev')
    await user.type(screen.getByLabelText('Password'), 's3cr3t-password')
    await user.type(screen.getByLabelText('Confirm password'), 's3cr3t-password')
    await user.click(screen.getByRole('button', { name: 'Create account' }))

    // Then the user lands directly in the app shell
    expect(await screen.findByRole('heading', { name: 'Home', level: 1 })).toBeInTheDocument()
  })

  it('logs an existing user in and reaches the app shell', async () => {
    // Given no local session, and Login succeeds
    vi.mocked(HasLocalSession).mockResolvedValueOnce(false)
    vi.mocked(Login).mockResolvedValueOnce({ accountId: 'acc-1', email: 'user@athena.dev' })
    const user = userEvent.setup()
    render(<App />)
    await screen.findByRole('heading', { name: 'Log in' })

    // When the user logs in
    await user.type(screen.getByLabelText('Email'), 'user@athena.dev')
    await user.type(screen.getByLabelText('Password'), 's3cr3t-password')
    await user.click(screen.getByRole('button', { name: 'Log in' }))

    // Then the user reaches the app shell
    expect(await screen.findByRole('heading', { name: 'Home', level: 1 })).toBeInTheDocument()
  })

  it('resets the local account and returns to the create-account screen', async () => {
    // Given no local session, and ResetLocalAccount succeeds
    vi.mocked(HasLocalSession).mockResolvedValueOnce(false)
    vi.mocked(ResetLocalAccount).mockResolvedValueOnce(undefined)
    const user = userEvent.setup()
    render(<App />)
    await screen.findByRole('heading', { name: 'Log in' })

    // When the user goes through the reset flow
    await user.click(screen.getByRole('button', { name: 'Forgot my password' }))
    await user.type(screen.getByLabelText('Email'), 'user@athena.dev')
    await user.click(screen.getByRole('button', { name: 'Delete local account' }))

    // Then the app returns to the create-account screen
    expect(await screen.findByRole('heading', { name: 'Create account' })).toBeInTheDocument()
  })

  it('shows the OpenRouter key gate before the app shell when no key is configured', async () => {
    // Given an existing local session but no OpenRouter key yet
    vi.mocked(HasLocalSession).mockResolvedValueOnce(true)
    vi.mocked(HasOpenRouterKey).mockResolvedValueOnce(false)

    // When the app mounts
    render(<App />)

    // Then the key gate is shown instead of the app shell
    expect(
      await screen.findByRole('heading', { name: 'Connect your OpenRouter key' }),
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
    await screen.findByRole('heading', { name: 'Connect your OpenRouter key' })

    // When submitting an invalid key
    await user.type(screen.getByLabelText('OpenRouter key'), 'sk-or-invalid')
    await user.click(screen.getByRole('button', { name: 'Connect' }))

    // Then the user stays on the key gate with an inline error
    expect(await screen.findByText('Invalid or unauthorized key.')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Connect your OpenRouter key' })).toBeInTheDocument()
  })

  it('shows onboarding after the key is saved when no profile exists yet', async () => {
    // Given an existing local session, no key configured yet, and no profile
    vi.mocked(HasLocalSession).mockResolvedValueOnce(true)
    vi.mocked(HasOpenRouterKey).mockResolvedValueOnce(false)
    vi.mocked(HasUserProfile).mockResolvedValueOnce(false)
    vi.mocked(SaveOpenRouterKey).mockResolvedValueOnce(undefined)
    const user = userEvent.setup()
    render(<App />)
    await screen.findByRole('heading', { name: 'Connect your OpenRouter key' })

    // When submitting a valid key
    await user.type(screen.getByLabelText('OpenRouter key'), 'sk-or-valid')
    await user.click(screen.getByRole('button', { name: 'Connect' }))

    // Then the onboarding form is shown next
    expect(
      await screen.findByRole('heading', { name: 'Tell us about yourself' }),
    ).toBeInTheDocument()
  })

  it('navigates from register back to login', async () => {
    // Given no local session
    vi.mocked(HasLocalSession).mockResolvedValueOnce(false)
    const user = userEvent.setup()
    render(<App />)
    await screen.findByRole('heading', { name: 'Log in' })

    // When going to the register screen and back to login
    await user.click(screen.getByRole('button', { name: 'Create account' }))
    await screen.findByRole('heading', { name: 'Create account' })
    await user.click(screen.getByRole('button', { name: 'I already have an account' }))

    // Then the login screen is shown again
    expect(await screen.findByRole('heading', { name: 'Log in' })).toBeInTheDocument()
  })

  it('cancels out of the reset flow back to login', async () => {
    // Given no local session
    vi.mocked(HasLocalSession).mockResolvedValueOnce(false)
    const user = userEvent.setup()
    render(<App />)
    await screen.findByRole('heading', { name: 'Log in' })

    // When going to the reset screen and cancelling
    await user.click(screen.getByRole('button', { name: 'Forgot my password' }))
    await screen.findByLabelText('Email')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    // Then the login screen is shown again
    expect(await screen.findByRole('heading', { name: 'Log in' })).toBeInTheDocument()
  })

  it('completes onboarding end-to-end and reaches the app shell', async () => {
    // Given an existing local session, no key, and no profile
    vi.mocked(HasLocalSession).mockResolvedValueOnce(true)
    vi.mocked(HasOpenRouterKey).mockResolvedValueOnce(false)
    vi.mocked(HasUserProfile).mockResolvedValueOnce(false)
    vi.mocked(SaveOpenRouterKey).mockResolvedValueOnce(undefined)
    vi.mocked(SaveProfile).mockResolvedValueOnce()
    const user = userEvent.setup()
    render(<App />)
    await screen.findByRole('heading', { name: 'Connect your OpenRouter key' })

    // When connecting the key and filling out onboarding
    // Free-text fields are filled by pasting rather than typing: this is
    // the longest test in the suite, and per-character typing spends a
    // macrotask and a React render on every keystroke for no added
    // coverage — the controlled inputs see the same change events either
    // way. Goals keeps type(), since its TagInput commits on {Enter}.
    await user.click(screen.getByLabelText('OpenRouter key'))
    await user.paste('sk-or-valid')
    await user.click(screen.getByRole('button', { name: 'Connect' }))
    await screen.findByRole('heading', { name: 'Tell us about yourself' })
    await user.click(screen.getByLabelText('Name'))
    await user.paste('Ana')
    await user.click(screen.getByLabelText('What would you like to call the assistant?'))
    await user.paste('Atena')
    await pickOption(user, 'Assistant language', 'English')
    await user.click(screen.getByLabelText('Area of study or work'))
    await user.paste('Software Engineering')
    await pickOption(user, 'Experience level', 'Intermediate')
    await user.type(screen.getByLabelText('Goals'), 'SQL{Enter}')
    await pickOption(user, 'Preferred study style', 'Lots of practical examples')
    await user.click(screen.getByRole('button', { name: 'Continue' }))
    await screen.findByRole('heading', { name: 'Confirm your profile' })
    await user.click(screen.getByRole('button', { name: 'Confirm and save' }))

    // Then the app reaches the app shell
    expect(await screen.findByRole('heading', { name: 'Home', level: 1 })).toBeInTheDocument()
  })

  it('skips the key gate and onboarding for a returning user with both already set up', async () => {
    // Given an existing local session, an existing key, and an existing profile
    vi.mocked(HasLocalSession).mockResolvedValueOnce(true)
    vi.mocked(HasOpenRouterKey).mockResolvedValueOnce(true)
    vi.mocked(HasUserProfile).mockResolvedValueOnce(true)

    // When the app mounts
    render(<App />)

    // Then it goes straight to the app shell
    expect(await screen.findByRole('heading', { name: 'Home', level: 1 })).toBeInTheDocument()
  })

  it('keeps the splash animation up for a minimum duration even when the session check resolves instantly', async () => {
    // Given no local session, resolved as fast as a mocked promise allows
    vi.mocked(HasLocalSession).mockResolvedValueOnce(false)

    // When the app mounts
    render(<App />)

    // Then the login screen hasn't taken over yet, well after the check has
    // resolved — a bare "await the check" would already have swapped it out
    await new Promise((resolve) => setTimeout(resolve, 100))
    expect(screen.queryByRole('heading', { name: 'Log in' })).not.toBeInTheDocument()

    // And the login screen only takes over once the minimum splash duration elapses
    expect(await screen.findByRole('heading', { name: 'Log in' })).toBeInTheDocument()
  })
})
