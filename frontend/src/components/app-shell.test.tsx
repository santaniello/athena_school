import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { GetProfile, Logout, UpdateProfile } from '../../wailsjs/go/desktop/App'
import { listStudySessionsByFolder, requestOpeningTurn, startStudySession } from '@/lib/study'
import type { StudySession } from '@/lib/study'
import { AppShell } from './app-shell'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  GetProfile: vi.fn(),
  Logout: vi.fn(),
  UpdateProfile: vi.fn(),
  SaveOpenRouterKey: vi.fn(),
  HasOpenRouterKey: vi.fn().mockResolvedValue(true),
}))

// The Study section's sidebar tree (StudyFolderTree) fetches folders as
// soon as it mounts, and StudyChatScreen subscribes to study events as soon
// as it mounts — both need mocking here, or they reach the real
// (unavailable in jsdom) Wails runtime.
vi.mock('@/lib/study', () => ({
  startStudySession: vi.fn(),
  requestOpeningTurn: vi.fn(),
  sendStudyMessage: vi.fn(),
  deleteStudySession: vi.fn(),
  resumeStudySession: vi.fn(),
  moveStudySession: vi.fn(),
  listStudySessionsByFolder: vi.fn(),
  onStudyChunk: vi.fn(() => vi.fn()),
  onStudyDone: vi.fn(() => vi.fn()),
  onStudyError: vi.fn(() => vi.fn()),
}))

vi.mock('@/lib/folder', () => ({
  listFolders: vi.fn().mockResolvedValue([{ id: 'default', name: 'General', isDefault: true }]),
  createFolder: vi.fn(),
  renameFolder: vi.fn(),
  deleteFolder: vi.fn(),
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

    // Then it lands on the real Study screen (the empty state, since no
    // session is open yet — creating one now happens from the sidebar tree),
    // not a locked coming-soon panel
    expect(screen.getByText('No session open')).toBeInTheDocument()
  })

  it('shows the folder tree in the sidebar only while on the Study section', async () => {
    // Given the app shell mounts on Home, with one folder available
    const user = userEvent.setup()
    renderShell()
    await screen.findByText(/Felipe\./)

    // Then the tree is not shown yet
    expect(screen.queryByText('New folder')).not.toBeInTheDocument()

    // When selecting Study
    await user.click(screen.getByRole('button', { name: 'Study' }))

    // Then the tree appears, listing the fetched folder
    expect(await screen.findByText('General')).toBeInTheDocument()
    expect(screen.getByText('New folder')).toBeInTheDocument()
  })

  it('shows the session title and folder breadcrumb in the topbar, in place of the section label, once a session is open', async () => {
    // Given the app shell mounts on Study, with a folder ready to hold a
    // new session
    const user = userEvent.setup()
    renderShell()
    await screen.findByText(/Felipe\./)
    await user.click(screen.getByRole('button', { name: 'Study' }))
    await screen.findByText('General')

    // Then the topbar still shows the "Study" section label — no session
    // is open yet
    expect(screen.getByRole('heading', { name: 'Study', level: 1 })).toBeInTheDocument()

    // When starting a new session inside that folder
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([])
    const startedSession: StudySession = {
      id: 'session-1',
      topic: 'Distributed systems',
      folderId: 'default',
      startedAt: '2026-08-17T10:00:00Z',
    }
    vi.mocked(startStudySession).mockResolvedValueOnce(startedSession)
    vi.mocked(requestOpeningTurn).mockReturnValueOnce(new Promise(() => {}))
    await user.click(screen.getByText('General'))
    await user.click(await screen.findByText('New session'))
    await user.type(
      screen.getByPlaceholderText('What do you want to study?'),
      'Distributed systems{Enter}',
    )

    // Then the topbar swaps the section label for the session's breadcrumb
    // and title — no back arrow, since the sidebar tree is the only way
    // back to another session
    expect(await screen.findByText('Study / General')).toBeInTheDocument()
    expect(
      screen.getByRole('heading', { name: 'Distributed systems', level: 1 }),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Back to folders' })).not.toBeInTheDocument()
  })

  it("shows the account's name in the sidebar footer", async () => {
    // Given the app shell mounts
    renderShell()

    // Then the profile name appears, with no plan/trial tag (not commercialized)
    expect(await screen.findByText('Felipe')).toBeInTheDocument()
    expect(screen.queryByText('Free trial')).not.toBeInTheDocument()
  })

  it('opens the real Settings screen, not the coming-soon panel, when Settings is selected', async () => {
    // Given the app shell mounts
    const user = userEvent.setup()
    renderShell()
    await screen.findByText(/Felipe\./)

    // When selecting Settings
    await user.click(screen.getByRole('button', { name: 'Settings' }))

    // Then the real profile form appears, not the locked-section panel
    expect(screen.getByLabelText('Assistant name')).toBeInTheDocument()
    expect(screen.queryByText('Planned for Phase 1')).not.toBeInTheDocument()
  })

  it('updates the sidebar name immediately after a Settings profile save, with no refetch', async () => {
    // Given the app shell mounts and the user opens Settings
    const saved = { ...profileResult, name: 'Felipe Santaniello' }
    vi.mocked(UpdateProfile).mockResolvedValueOnce(saved)
    const user = userEvent.setup()
    renderShell()
    await screen.findByText(/Felipe\./)
    await user.click(screen.getByRole('button', { name: 'Settings' }))

    // When editing the name and saving
    const nameInput = screen.getByLabelText('Name')
    await user.clear(nameInput)
    await user.type(nameInput, 'Felipe Santaniello')
    await user.click(screen.getByRole('button', { name: 'Save changes' }))

    // Then the sidebar footer reflects the new name immediately, with no
    // second GetProfile call — the saved response is trusted directly
    await waitFor(() => expect(screen.getByText('Felipe Santaniello')).toBeInTheDocument())
    expect(GetProfile).toHaveBeenCalledOnce()
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
