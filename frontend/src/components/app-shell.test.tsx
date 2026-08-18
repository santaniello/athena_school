import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { GetProfile, Logout, UpdateProfile } from '../../wailsjs/go/desktop/App'
import {
  deleteStudySession,
  listStudySessionsByFolder,
  requestOpeningTurn,
  resumeStudySession,
  startStudySession,
} from '@/lib/study'
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
      'Documentation',
      'Settings',
    ]) {
      expect(screen.getByRole('button', { name: label })).toBeInTheDocument()
    }
  })

  it('opens the real Documentation screen, not the coming-soon panel', async () => {
    // Given the app shell mounts
    const user = userEvent.setup()
    renderShell()
    await screen.findByText(/Felipe\./)

    // When selecting Documentation
    await user.click(screen.getByRole('button', { name: 'Documentation' }))

    // Then the manual renders, with its contents list
    expect(screen.getByRole('navigation', { name: 'Contents' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Why Athena exists' })).toBeInTheDocument()
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

  it('opens an existing session picked from the sidebar tree in resume mode, wiring it into the topbar immediately and highlighting it in the tree', async () => {
    // Given Study is open, with a folder holding one previously-created
    // session
    const user = userEvent.setup()
    renderShell()
    await screen.findByText(/Felipe\./)
    await user.click(screen.getByRole('button', { name: 'Study' }))
    await screen.findByText('General')

    const existingSession: StudySession = {
      id: 'session-existing',
      topic: 'Existing topic',
      folderId: 'default',
      startedAt: '2026-08-10T10:00:00Z',
    }
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([existingSession])
    vi.mocked(resumeStudySession).mockReturnValueOnce(new Promise(() => {}))

    // When picking that session from the tree
    await user.click(screen.getByText('General'))
    await user.click(await screen.findByText('Existing topic'))

    // Then the topbar reflects it immediately — id/topic/folder are set
    // synchronously, only the deeper history load is async — through the
    // resume path (resumeStudySession), not the "new session" opening-turn
    // path, and the tree highlights it as the selected row
    expect(await screen.findByText('Study / General')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Existing topic', level: 1 })).toBeInTheDocument()
    expect(resumeStudySession).toHaveBeenCalledWith('session-existing')
    expect(requestOpeningTurn).not.toHaveBeenCalled()
    const sessionRow = screen
      .getByText('Existing topic', { selector: 'span.truncate' })
      .closest('div')
    expect(sessionRow).toHaveClass('bg-secondary')
  })

  it('clears the active session and reverts the topbar/main pane when that same session is deleted from the tree', async () => {
    // Given a session is open in Study, picked from the sidebar tree
    const user = userEvent.setup()
    renderShell()
    await screen.findByText(/Felipe\./)
    await user.click(screen.getByRole('button', { name: 'Study' }))
    await screen.findByText('General')

    const session: StudySession = {
      id: 'session-a',
      topic: 'Session A',
      folderId: 'default',
      startedAt: '2026-08-10T10:00:00Z',
    }
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([session])
    vi.mocked(resumeStudySession).mockReturnValue(new Promise(() => {}))
    await user.click(screen.getByText('General'))
    await user.click(await screen.findByText('Session A'))
    expect(await screen.findByText('Study / General')).toBeInTheDocument()

    // When deleting that same session from its tree row
    vi.mocked(deleteStudySession).mockResolvedValueOnce()
    await user.click(screen.getByRole('button', { name: 'Session A options' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Delete' }))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Delete session' }))

    // Then the topbar reverts to the section label, and the main pane
    // reverts to the empty state
    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'Study', level: 1 })).toBeInTheDocument(),
    )
    expect(screen.getByText('No session open')).toBeInTheDocument()
  })

  it('deletes a session without crashing when no session was ever opened', async () => {
    // Given Study is open with one existing session, but none selected yet
    const user = userEvent.setup()
    renderShell()
    await screen.findByText(/Felipe\./)
    await user.click(screen.getByRole('button', { name: 'Study' }))
    await screen.findByText('General')

    const orphanSession: StudySession = {
      id: 'session-orphan',
      topic: 'Orphan session',
      folderId: 'default',
      startedAt: '2026-08-10T10:00:00Z',
    }
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([orphanSession])
    vi.mocked(deleteStudySession).mockResolvedValueOnce()
    await user.click(screen.getByText('General'))
    await screen.findByText('Orphan session')

    // When deleting it without ever opening it
    await user.click(screen.getByRole('button', { name: 'Orphan session options' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Delete' }))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Delete session' }))

    // Then the shell keeps working normally: still on Study, still with no
    // session open
    await waitFor(() => expect(screen.queryByText('Orphan session')).not.toBeInTheDocument())
    expect(screen.getByRole('heading', { name: 'Study', level: 1 })).toBeInTheDocument()
    expect(screen.getByText('No session open')).toBeInTheDocument()
  })

  it("updates the topbar title when a resumed session's real topic differs from the sidebar's cached copy", async () => {
    // Given an existing session picked from the tree, whose cached topic
    // is stale
    const user = userEvent.setup()
    renderShell()
    await screen.findByText(/Felipe\./)
    await user.click(screen.getByRole('button', { name: 'Study' }))
    await screen.findByText('General')

    const cachedSession: StudySession = {
      id: 'session-stale',
      topic: 'Cached topic',
      folderId: 'default',
      startedAt: '2026-08-10T10:00:00Z',
    }
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([cachedSession])
    vi.mocked(resumeStudySession).mockResolvedValueOnce({
      session: { ...cachedSession, topic: 'Real topic' },
      messages: [],
    })
    await user.click(screen.getByText('General'))
    await user.click(await screen.findByText('Cached topic'))

    // When the resumed session's history resolves with its real topic
    // Then the topbar title updates to match, in place of the stale cache
    // (resumeStudySession may resolve before this assertion even runs, so
    // this only checks the eventual, settled state rather than racing an
    // intermediate render)
    expect(await screen.findByRole('heading', { name: 'Real topic', level: 1 })).toBeInTheDocument()
    expect(
      screen.queryByRole('heading', { name: 'Cached topic', level: 1 }),
    ).not.toBeInTheDocument()
  })

  it('reverts the topbar to the section label when navigating away from an open study session', async () => {
    // Given a session is open in Study
    const user = userEvent.setup()
    renderShell()
    await screen.findByText(/Felipe\./)
    await user.click(screen.getByRole('button', { name: 'Study' }))
    await screen.findByText('General')
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([])
    const startedSession: StudySession = {
      id: 'session-nav',
      topic: 'Nav test topic',
      folderId: 'default',
      startedAt: '2026-08-17T10:00:00Z',
    }
    vi.mocked(startStudySession).mockResolvedValueOnce(startedSession)
    vi.mocked(requestOpeningTurn).mockReturnValueOnce(new Promise(() => {}))
    await user.click(screen.getByText('General'))
    await user.click(await screen.findByText('New session'))
    await user.type(
      screen.getByPlaceholderText('What do you want to study?'),
      'Nav test topic{Enter}',
    )
    expect(await screen.findByText('Study / General')).toBeInTheDocument()

    // When navigating to Home while that session stays open in the
    // background
    await user.click(screen.getByRole('button', { name: 'Home' }))

    // Then the topbar shows the Home section label, not the stale session
    // breadcrumb/title
    expect(screen.getByRole('heading', { name: 'Home', level: 1 })).toBeInTheDocument()
    expect(screen.queryByText('Study / General')).not.toBeInTheDocument()
    expect(screen.queryByText('Nav test topic')).not.toBeInTheDocument()
  })

  it('applies the required inline layout styles to the resizable panels and the sidebar scroll container', async () => {
    // Given the app shell mounts
    const { container } = renderShell()
    await screen.findByText(/Felipe\./)

    // Then the panel group forces full-viewport height, since #root has no
    // height of its own to inherit
    const group = container.querySelector('[data-slot="resizable-panel-group"]') as HTMLElement
    expect(group.style.height).toBe('100vh')

    // ...both panels hide their own overflow, so the library's own
    // scrollbar doesn't double up with the sidebar's or the chat
    // transcript's own scroll areas
    const panels = container.querySelectorAll('[data-slot="resizable-panel"]')
    expect(panels).toHaveLength(2)
    const sidebarPanelContent = panels[0]?.firstElementChild as HTMLElement
    const mainPanelContent = panels[1]?.firstElementChild as HTMLElement
    expect(sidebarPanelContent.style.overflow).toBe('hidden')
    expect(mainPanelContent.style.overflow).toBe('hidden')

    // ...and the sidebar's nav list reserves room for its scrollbar so
    // content doesn't shift horizontally when it appears
    const navList = container.querySelector('.thin-scroll.mt-2') as HTMLElement
    expect(navList.style.scrollbarGutter).toBe('stable')
  })

  it('marks the active nav item with aria-current, in both the primary list and the footer', async () => {
    // Given the app shell mounts on Home (default section)
    const user = userEvent.setup()
    renderShell()
    await screen.findByText(/Felipe\./)

    // Then Home (primary, active) carries aria-current, while Study
    // (primary, inactive) and Settings (footer, inactive) do not
    expect(screen.getByRole('button', { name: 'Home' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('button', { name: 'Study' })).not.toHaveAttribute('aria-current')
    expect(screen.getByRole('button', { name: 'Settings' })).not.toHaveAttribute('aria-current')

    // When selecting Settings (footer)
    await user.click(screen.getByRole('button', { name: 'Settings' }))

    // Then Settings gains aria-current and Home loses it
    expect(screen.getByRole('button', { name: 'Settings' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('button', { name: 'Home' })).not.toHaveAttribute('aria-current')
  })

  it('renders the sidebar avatar initial as the uppercase first letter of the profile name', async () => {
    // Given the app shell mounts with a profile named "Felipe"
    const { container } = renderShell()
    await screen.findByText(/Felipe\./)

    // Then the avatar shows just "F" — not the lowercase initial, and not
    // the full name
    const avatar = container.querySelector('.text-primary-foreground') as HTMLElement
    expect(avatar.textContent).toBe('F')
  })

  it('does not show the Study locked-note on Home, since Study is an unlocked Phase 1 section', async () => {
    // Given the app shell mounts on Home
    renderShell()
    await screen.findByText(/Felipe\./)

    // Then the CTA has no "locked" caption
    expect(screen.queryByText('Locked until Study Mode ships')).not.toBeInTheDocument()
  })
})
