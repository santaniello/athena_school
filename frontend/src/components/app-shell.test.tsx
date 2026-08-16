import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { GetProfile, Logout } from '../../wailsjs/go/desktop/App'
import { AppShell } from './app-shell'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  GetProfile: vi.fn(),
  Logout: vi.fn(),
}))

const profileResult = {
  name: 'Felipe',
  assistantName: 'Athena',
  area: 'Software Engineering',
  experienceLevel: 'intermediate',
  goals: ['System Design'],
  studyStyle: 'practical_examples',
  assistantLanguage: 'en',
}

function renderShell() {
  vi.mocked(GetProfile).mockResolvedValueOnce(profileResult)
  const onLogout = vi.fn()
  const utils = render(<AppShell onLogout={onLogout} />)
  return { ...utils, onLogout }
}

describe('AppShell', () => {
  it('lands on Home and greets the user once the profile resolves', async () => {
    // Given the app shell mounts
    renderShell()

    // Then Home is the active section, greeting the user by name
    expect(await screen.findByText(/Felipe\./)).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Home', level: 1 })).toBeInTheDocument()
  })

  it('lists every roadmap section in the sidebar', async () => {
    // Given the app shell mounts
    renderShell()
    await screen.findByText(/Felipe\./)

    // Then every section from the manifest appears as a nav row
    for (const label of [
      'Home',
      'Study',
      'Knowledge',
      'Challenge',
      'Progress',
      'Flashcards',
      'Interview',
      'Settings',
    ]) {
      expect(screen.getByRole('button', { name: label })).toBeInTheDocument()
    }
  })

  it('opens the coming-soon panel when a locked section is selected', async () => {
    // Given the app shell mounts
    const user = userEvent.setup()
    renderShell()
    await screen.findByText(/Felipe\./)

    // When selecting a locked section
    await user.click(screen.getByRole('button', { name: 'Knowledge' }))

    // Then the topbar and content reflect that section, still locked
    expect(screen.getByRole('heading', { name: 'Knowledge', level: 1 })).toBeInTheDocument()
    expect(screen.getByText('Planned for Phase 2')).toBeInTheDocument()
  })

  it('routes the Home CTA to the Study screen, same as the Study nav row', async () => {
    // Given the app shell mounts on Home
    const user = userEvent.setup()
    renderShell()
    await screen.findByText(/Felipe\./)

    // When starting a study session from the CTA
    await user.click(screen.getByRole('button', { name: 'Start a study session' }))

    // Then it lands on the real Study screen (topic selection), not a
    // locked coming-soon panel
    expect(screen.getByLabelText(/study today/i)).toBeInTheDocument()
  })

  it("shows the account's name in the sidebar footer", async () => {
    // Given the app shell mounts
    renderShell()

    // Then the profile name appears, with no plan/trial tag (not commercialized)
    expect(await screen.findByText('Felipe')).toBeInTheDocument()
    expect(screen.queryByText('Free trial')).not.toBeInTheDocument()
  })

  it('clears the session and calls onLogout when logging out', async () => {
    // Given the app shell mounts
    vi.mocked(Logout).mockResolvedValueOnce()
    const user = userEvent.setup()
    const { onLogout } = renderShell()
    await screen.findByText(/Felipe\./)

    // When logging out
    await user.click(screen.getByRole('button', { name: 'Log out' }))

    // Then the local session is cleared and the caller is notified
    await waitFor(() => expect(onLogout).toHaveBeenCalledOnce())
    expect(Logout).toHaveBeenCalledOnce()
  })
})
