import { describe, expect, it, vi } from 'vitest'
import { act, render, screen, waitFor, within } from '@testing-library/react'
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
import {
  getKnowledgeIndexStatus,
  onKnowledgeIndexStatus,
  retryKnowledgeIndex,
} from '@/lib/knowledge-index'
import type { IndexStatus } from '@/lib/knowledge-index'
import { AppShell } from './app-shell'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  GetProfile: vi.fn(),
  Logout: vi.fn(),
  UpdateProfile: vi.fn(),
  SaveOpenRouterKey: vi.fn(),
  HasOpenRouterKey: vi.fn().mockResolvedValue(true),
  GetKnowledgeExtractionSettings: vi.fn().mockResolvedValue({ maxKnowledgeExtractionItems: 8 }),
  UpdateKnowledgeExtractionSettings: vi.fn(),
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

// The Knowledge section's sidebar tree and Explorer screen both fetch as
// soon as they mount, same reasoning as the Study mocks above. SettingsScreen
// also imports from this module (getKnowledgeExtractionSettings), so this
// keeps the rest of the module real rather than replacing it wholesale.
vi.mock('@/lib/knowledge', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/knowledge')>()
  return {
    ...original,
    listKnowledgeItems: vi.fn().mockResolvedValue([]),
    listKnowledgeTopics: vi.fn().mockResolvedValue([]),
    approveKnowledgeItem: vi.fn(),
    deprecateKnowledgeItem: vi.fn(),
    updateKnowledgeItem: vi.fn(),
    deleteKnowledgeItem: vi.fn(),
  }
})

vi.mock('@/lib/ingest', () => ({
  pickNotesFolder: vi.fn(),
  importNotes: vi.fn(),
  onIngestProgress: vi.fn(() => vi.fn()),
  onIngestDone: vi.fn(() => vi.fn()),
  onIngestError: vi.fn(() => vi.fn()),
}))

// Defaults every test to a fully ready index, so the shell renders past the
// knowledge-index gate immediately — tests specifically about the gate's
// own states (loading/failed/ready_with_warnings/retry) override these.
vi.mock('@/lib/knowledge-index', () => ({
  getKnowledgeIndexStatus: vi
    .fn()
    .mockResolvedValue({ state: 'ready', hasSnapshot: true, issues: [], lastError: '' }),
  retryKnowledgeIndex: vi.fn(),
  onKnowledgeIndexStatus: vi.fn(() => vi.fn()),
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

    // When selecting a locked section (challenge stays locked for the
    // whole of Phase 2, unlike knowledge)
    await user.click(screen.getByRole('button', { name: 'Challenge' }))

    // Then the topbar and content reflect that section, still locked
    expect(screen.getByRole('heading', { name: 'Challenge', level: 1 })).toBeInTheDocument()
    expect(screen.getByText('Planned for Phase 3')).toBeInTheDocument()
  })

  it('opens the real Knowledge Explorer, not the coming-soon panel', async () => {
    // Given the app shell mounts
    const user = userEvent.setup()
    renderShell()
    await screen.findByText(/Felipe\./)

    // When selecting Knowledge
    await user.click(screen.getByRole('button', { name: 'Knowledge' }))

    // Then the Explorer/Review tabs render instead of the coming-soon panel
    expect(screen.getByRole('heading', { name: 'Knowledge', level: 1 })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Explorer' })).toBeInTheDocument()
    expect(screen.queryByText('Planned for Phase 2')).not.toBeInTheDocument()
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

function pendingStatus(): Promise<IndexStatus> {
  return new Promise(() => {})
}

describe('AppShell knowledge index lifecycle', () => {
  it('subscribes to knowledge-index:status before querying the initial status, and shows the loading screen until it resolves', () => {
    // Given the initial status query never resolves during this assertion
    const order: string[] = []
    vi.mocked(onKnowledgeIndexStatus).mockImplementationOnce((handler) => {
      order.push('subscribe')
      void handler
      return vi.fn()
    })
    vi.mocked(getKnowledgeIndexStatus).mockImplementationOnce(() => {
      order.push('query')
      return pendingStatus()
    })

    // When the shell mounts
    renderShell()

    // Then the event listener was registered before the initial query fired
    // — closing the race where a fast background load finishes before the
    // listener subscribes — and the loading screen blocks everything else
    expect(order).toEqual(['subscribe', 'query'])
    expect(screen.getByText('Loading knowledge index...')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Home' })).not.toBeInTheDocument()
  })

  it('releases the app once the initial load resolves ready', async () => {
    // Given the default ready mock (see the module-level vi.mock above)
    renderShell()

    // Then the shell renders normally once the status resolves
    expect(await screen.findByRole('button', { name: 'Home' })).toBeInTheDocument()
  })

  it('shows the failure screen when the initial load fails, and Continue reveals the app with a persistent warning', async () => {
    // Given an initial load that fails outright
    vi.mocked(getKnowledgeIndexStatus).mockResolvedValueOnce({
      state: 'failed',
      hasSnapshot: false,
      issues: [],
      lastError: 'Could not load the knowledge index from the database.',
    })
    const user = userEvent.setup()
    renderShell()

    // Then the failure screen blocks the app
    expect(await screen.findByText('Knowledge index could not be loaded.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Home' })).not.toBeInTheDocument()

    // When continuing without local search
    await user.click(screen.getByRole('button', { name: 'Continue without local search' }))

    // Then the app opens, with a persistent unavailable warning
    expect(await screen.findByRole('button', { name: 'Home' })).toBeInTheDocument()
    expect(screen.getByText(/Local search is unavailable/)).toBeInTheDocument()
  })

  it('retries from the failure screen and releases the app on success', async () => {
    // Given an initial load that fails outright
    vi.mocked(getKnowledgeIndexStatus).mockResolvedValueOnce({
      state: 'failed',
      hasSnapshot: false,
      issues: [],
      lastError: 'disk full',
    })
    vi.mocked(retryKnowledgeIndex).mockResolvedValueOnce({
      state: 'ready',
      hasSnapshot: true,
      issues: [],
      lastError: '',
    })
    const user = userEvent.setup()
    renderShell()
    await screen.findByText('Knowledge index could not be loaded.')

    // When retrying
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    // Then the app opens, with no lingering failure screen or warning
    expect(await screen.findByRole('button', { name: 'Home' })).toBeInTheDocument()
    expect(screen.queryByText(/Local search is unavailable/)).not.toBeInTheDocument()
  })

  it('shows a warning banner with the affected count when ready_with_warnings, and opens the review dialog', async () => {
    // Given a load that isolated one chunk
    vi.mocked(getKnowledgeIndexStatus).mockResolvedValueOnce({
      state: 'ready_with_warnings',
      hasSnapshot: true,
      issues: [
        {
          chunkId: 'chunk-1',
          itemId: 'item-1',
          source: 'imported_doc',
          filePath: 'notes/go.md',
          reason: 'missing_item',
        },
      ],
      lastError: '',
    })
    const user = userEvent.setup()
    renderShell()

    // Then the app opens with a warning banner reporting the affected count
    expect(await screen.findByText(/1 item/)).toBeInTheDocument()

    // When opening Review
    await user.click(screen.getByRole('button', { name: 'Review' }))

    // Then the isolated chunk's file and guidance appear
    expect(await screen.findByText('notes/go.md')).toBeInTheDocument()
    expect(
      screen.getByText('The knowledge item this content belonged to no longer exists.'),
    ).toBeInTheDocument()
  })

  it('closes the review dialog when dismissed', async () => {
    // Given the app is open with the review dialog showing one issue
    vi.mocked(getKnowledgeIndexStatus).mockResolvedValueOnce({
      state: 'ready_with_warnings',
      hasSnapshot: true,
      issues: [
        {
          chunkId: 'chunk-1',
          itemId: 'item-1',
          source: 'imported_doc',
          filePath: 'notes/go.md',
          reason: 'missing_item',
        },
      ],
      lastError: '',
    })
    const user = userEvent.setup()
    renderShell()
    await screen.findByText(/1 item/)
    await user.click(screen.getByRole('button', { name: 'Review' }))
    await screen.findByText('notes/go.md')

    // When closing it
    const closeButtons = await screen.findAllByRole('button', { name: 'Close' })
    await user.click(closeButtons[0])

    // Then it is dismissed
    await waitFor(() => expect(screen.queryByText('notes/go.md')).not.toBeInTheDocument())
  })

  it('disables Retry/Continue while a retry is in flight, and releases the app once it resolves', async () => {
    // Given the failure screen, with a retry that stays pending until released
    vi.mocked(getKnowledgeIndexStatus).mockResolvedValueOnce({
      state: 'failed',
      hasSnapshot: false,
      issues: [],
      lastError: 'disk full',
    })
    let resolveRetry: (status: IndexStatus) => void = () => {}
    vi.mocked(retryKnowledgeIndex).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveRetry = resolve
      }),
    )
    const user = userEvent.setup()
    renderShell()
    await screen.findByText('Knowledge index could not be loaded.')

    // When retrying
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    // Then both actions are disabled while the retry is still pending
    expect(screen.getByRole('button', { name: 'Retry' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Continue without local search' })).toBeDisabled()

    // When the retry resolves
    resolveRetry({ state: 'ready', hasSnapshot: true, issues: [], lastError: '' })

    // Then the app opens — no failure screen, no lingering disabled state
    expect(await screen.findByRole('button', { name: 'Home' })).toBeInTheDocument()
  })

  it('retrying from the persistent banner does not silently re-block the app when it fails again', async () => {
    // Given the user already continued past a failed load with no snapshot
    vi.mocked(getKnowledgeIndexStatus).mockResolvedValueOnce({
      state: 'failed',
      hasSnapshot: false,
      issues: [],
      lastError: 'disk full',
    })
    const user = userEvent.setup()
    renderShell()
    await screen.findByText('Knowledge index could not be loaded.')
    await user.click(screen.getByRole('button', { name: 'Continue without local search' }))
    await screen.findByText(/Local search is unavailable/)

    // When retrying from the banner and it fails again (still no snapshot)
    vi.mocked(retryKnowledgeIndex).mockResolvedValueOnce({
      state: 'failed',
      hasSnapshot: false,
      issues: [],
      lastError: 'disk full again',
    })
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    // Then the app stays open (does not silently re-block behind the
    // failure screen) and the persistent warning is still shown
    await waitFor(() => expect(retryKnowledgeIndex).toHaveBeenCalledOnce())
    expect(screen.getByRole('button', { name: 'Home' })).toBeInTheDocument()
    expect(screen.getByText(/Local search is unavailable/)).toBeInTheDocument()
  })

  it('actually resets the "continued without search" opt-in on a successful retry, re-blocking on a later failure', async () => {
    // Given the user already continued past a failed load with no snapshot,
    // with the status-event handler captured so a later event can be
    // simulated directly (the banner disappearing on success alone can't
    // distinguish "opt-in was reset" from "opt-in is just masked by state"
    // — both look identical once state moves off 'failed')
    let statusHandler: (status: IndexStatus) => void = () => {}
    vi.mocked(onKnowledgeIndexStatus).mockImplementationOnce((handler) => {
      statusHandler = handler
      return vi.fn()
    })
    vi.mocked(getKnowledgeIndexStatus).mockResolvedValueOnce({
      state: 'failed',
      hasSnapshot: false,
      issues: [],
      lastError: 'disk full',
    })
    const user = userEvent.setup()
    renderShell()
    await screen.findByText('Knowledge index could not be loaded.')
    await user.click(screen.getByRole('button', { name: 'Continue without local search' }))
    await screen.findByText(/Local search is unavailable/)

    // When retrying from the banner and it succeeds this time
    vi.mocked(retryKnowledgeIndex).mockResolvedValueOnce({
      state: 'ready',
      hasSnapshot: true,
      issues: [],
      lastError: '',
    })
    await user.click(screen.getByRole('button', { name: 'Retry' }))
    await waitFor(() =>
      expect(screen.queryByText(/Local search is unavailable/)).not.toBeInTheDocument(),
    )

    // And a later load genuinely fails again (e.g. a subsequent retry
    // elsewhere, or a fresh OnDomReady on the next launch, reported via the
    // same status event)
    act(() => {
      statusHandler({
        state: 'failed',
        hasSnapshot: false,
        issues: [],
        lastError: 'disk full again',
      })
    })

    // Then the app re-blocks behind the failure screen — proving the opt-in
    // was truly reset to false, not left stuck true from the earlier choice
    expect(await screen.findByText('Knowledge index could not be loaded.')).toBeInTheDocument()
  })

  it('resolves out of the loading screen when the initial status query rejects', async () => {
    // Given the initial query fails outright (e.g. the backend never
    // responds to the Wails call)
    vi.mocked(getKnowledgeIndexStatus).mockRejectedValueOnce(new Error('boom'))
    renderShell()

    // Then the app falls back to the failure screen instead of hanging
    // behind IndexLoadingScreen forever
    expect(await screen.findByText('Knowledge index could not be loaded.')).toBeInTheDocument()
  })

  it('ignores a delayed initial response that resolves after a newer status event already arrived', async () => {
    // Given the initial query stays pending, with its resolver captured
    let resolveInitial: (status: IndexStatus) => void = () => {}
    vi.mocked(getKnowledgeIndexStatus).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveInitial = resolve
      }),
    )
    let statusHandler: (status: IndexStatus) => void = () => {}
    vi.mocked(onKnowledgeIndexStatus).mockImplementationOnce((handler) => {
      statusHandler = handler
      return vi.fn()
    })
    renderShell()
    expect(screen.getByText('Loading knowledge index...')).toBeInTheDocument()

    // When a newer status event arrives first
    act(() => {
      statusHandler({ state: 'ready', hasSnapshot: true, issues: [], lastError: '' })
    })
    expect(await screen.findByRole('button', { name: 'Home' })).toBeInTheDocument()

    // And the stale initial query resolves afterward with an older status
    await act(async () => {
      resolveInitial({ state: 'loading', hasSnapshot: false, issues: [], lastError: '' })
      await Promise.resolve()
    })

    // Then the newer event's state is not overwritten by the late response
    expect(screen.getByRole('button', { name: 'Home' })).toBeInTheDocument()
  })

  it('does not block the app when a retry fails but a previous snapshot is preserved', async () => {
    // Given a ready index whose retry then fails without losing its snapshot
    vi.mocked(getKnowledgeIndexStatus).mockResolvedValueOnce({
      state: 'ready',
      hasSnapshot: true,
      issues: [],
      lastError: '',
    })
    vi.mocked(retryKnowledgeIndex).mockResolvedValueOnce({
      state: 'failed',
      hasSnapshot: true,
      issues: [],
      lastError: 'disk full',
    })
    let statusHandler: (status: IndexStatus) => void = () => {}
    vi.mocked(onKnowledgeIndexStatus).mockImplementationOnce((handler) => {
      statusHandler = handler
      return vi.fn()
    })
    renderShell()
    await screen.findByRole('button', { name: 'Home' })

    // When a retry fails, reported through the same status event app-shell
    // subscribes to for every retry outcome
    act(() => {
      statusHandler({ state: 'failed', hasSnapshot: true, issues: [], lastError: 'disk full' })
    })

    // Then the app never falls back to the blocking failure screen — the
    // preserved snapshot means search still works — and the banner offers Retry
    expect(screen.queryByText('Knowledge index could not be loaded.')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Home' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
  })

  it('queries and subscribes to the knowledge index status exactly once, even across section navigation', async () => {
    // Given the app is open
    const user = userEvent.setup()
    renderShell()
    await screen.findByRole('button', { name: 'Home' })
    expect(getKnowledgeIndexStatus).toHaveBeenCalledOnce()
    expect(onKnowledgeIndexStatus).toHaveBeenCalledOnce()

    // When navigating to another section (a re-render, not a remount)
    await user.click(screen.getByRole('button', { name: 'Settings' }))

    // Then the subscription/query effect never re-ran
    expect(getKnowledgeIndexStatus).toHaveBeenCalledOnce()
    expect(onKnowledgeIndexStatus).toHaveBeenCalledOnce()
  })
})
