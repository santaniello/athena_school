import { afterEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Folder } from '@/lib/folder'
import { createFolder, deleteFolder, listFolders, renameFolder } from '@/lib/folder'
import type { StudySession } from '@/lib/study'
import {
  deleteStudySession,
  listStudySessionsByFolder,
  moveStudySession,
  startStudySession,
} from '@/lib/study'
import { StudyFolderTree } from './study-folder-tree'

vi.mock('@/lib/folder', () => ({
  listFolders: vi.fn(),
  createFolder: vi.fn(),
  renameFolder: vi.fn(),
  deleteFolder: vi.fn(),
}))

vi.mock('@/lib/study', () => ({
  listStudySessionsByFolder: vi.fn(),
  startStudySession: vi.fn(),
  moveStudySession: vi.fn(),
  deleteStudySession: vi.fn(),
}))

// The project's global `clearMocks` only clears call history between tests,
// not any implementation queued via mockResolvedValueOnce/mockImplementation.
// Several tests here trigger incidental extra calls to these mocks (e.g.
// opening a folder's dropdown menu bubbles a click up through the header,
// per DroppableFolderHeader/DraggableSessionRow's handlers not all calling
// stopPropagation), so an unconsumed queued value from one test could
// otherwise leak into and skew the next. A full reset between tests keeps
// each test's mocks isolated regardless of how many incidental calls happen.
afterEach(() => {
  vi.resetAllMocks()
})

const GENERAL: Folder = { id: 'default', name: 'General', isDefault: true }
const SYSTEM_DESIGN: Folder = { id: 'folder-1', name: 'System Design', isDefault: false }

const CACHE_SESSION: StudySession = {
  id: 'session-1',
  topic: 'Cache invalidation',
  folderId: 'folder-1',
  startedAt: '2026-08-16T10:00:00Z',
}
const LOAD_BALANCING_SESSION: StudySession = {
  id: 'session-3',
  topic: 'Load balancing',
  folderId: 'folder-1',
  startedAt: '2026-08-16T09:00:00Z',
}

function renderTree(props?: {
  selectedSessionId?: string | null
  onSelectSession?: (session: StudySession, folderName: string) => void
  onSessionStarted?: (session: StudySession, folderName: string) => void
  onSessionDeleted?: (sessionId: string) => void
}) {
  const onSelectSession = props?.onSelectSession ?? vi.fn()
  const onSessionStarted = props?.onSessionStarted ?? vi.fn()
  const onSessionDeleted = props?.onSessionDeleted ?? vi.fn()
  render(
    <StudyFolderTree
      selectedSessionId={props?.selectedSessionId ?? null}
      onSelectSession={onSelectSession}
      onSessionStarted={onSessionStarted}
      onSessionDeleted={onSessionDeleted}
    />,
  )
  return { onSelectSession, onSessionStarted, onSessionDeleted }
}

// jsdom has no layout engine, so every element's getBoundingClientRect()
// normally returns a zero-size rect at the origin, which starves dnd-kit's
// (real, math-based) rectangle-intersection collision detection of anything
// to intersect — the "over" droppable never resolves. Stubbing every
// element to report the same non-zero rect for the duration of a simulated
// drag gives dnd-kit legitimate, self-consistent geometry to detect a
// collision with, without faking any of the component's own logic: the
// PointerSensor's activation (pointerdown + pointermove past the 4px
// threshold), the resulting onDragStart/onDragEnd callbacks, and the
// collision math itself all still run for real.
async function dragSessionOnto(sessionRow: Element, targetHeader: Element) {
  const original = Element.prototype.getBoundingClientRect
  Element.prototype.getBoundingClientRect = function () {
    return {
      x: 0,
      y: 0,
      left: 0,
      top: 0,
      right: 100,
      bottom: 20,
      width: 100,
      height: 20,
      toJSON() {},
    } as DOMRect
  }
  try {
    fireEvent.pointerDown(sessionRow, {
      pointerId: 1,
      clientX: 5,
      clientY: 5,
      isPrimary: true,
      button: 0,
    })
    fireEvent.pointerMove(document, {
      pointerId: 1,
      clientX: 20,
      clientY: 5,
      isPrimary: true,
      button: 0,
    })
    await waitFor(() => expect(sessionRow).toHaveClass('opacity-40'))
    fireEvent.pointerMove(document, {
      pointerId: 1,
      clientX: 30,
      clientY: 5,
      isPrimary: true,
      button: 0,
    })
    await waitFor(() => expect(targetHeader).toHaveClass('bg-accent'))
    fireEvent.pointerUp(document, {
      pointerId: 1,
      clientX: 30,
      clientY: 5,
      isPrimary: true,
      button: 0,
    })
  } finally {
    Element.prototype.getBoundingClientRect = original
  }
}

// dnd-kit schedules some of its own internal cleanup (detaching the
// activated sensor's document-level listeners, its drop-animation frame)
// slightly after the React state settles. Ending a test right after that
// state settles — but before dnd-kit's own cleanup has actually run — can
// leave stale listeners behind when the component unmounts, which has been
// observed to interfere with a *later* test's plain clicks. Giving it a
// real macrotask before moving on avoids that.
async function settleDrag() {
  await new Promise((resolve) => setTimeout(resolve, 100))
}

describe('StudyFolderTree', () => {
  it('lists every folder on mount', async () => {
    // Given two folders
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL, SYSTEM_DESIGN])

    // When the tree mounts
    renderTree()

    // Then both folder names appear
    expect(await screen.findByText('General')).toBeInTheDocument()
    expect(screen.getByText('System Design')).toBeInTheDocument()
  })

  it('renders no folder rows before the initial fetch resolves', () => {
    // Given a listFolders call that never resolves during this test
    vi.mocked(listFolders).mockReturnValueOnce(new Promise(() => {}))

    // When the tree mounts
    renderTree()

    // Then no folder row is rendered — the initial state is an empty list,
    // not a placeholder entry
    expect(screen.queryAllByRole('button', { name: /options$/ })).toHaveLength(0)
  })

  it('lazily loads and shows a folder’s sessions when expanded', async () => {
    // Given a folder with one session
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([CACHE_SESSION])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')

    // When expanding the folder
    await user.click(screen.getByText('System Design'))

    // Then the session is fetched and shown
    expect(await screen.findByText('Cache invalidation')).toBeInTheDocument()
    expect(listStudySessionsByFolder).toHaveBeenCalledWith('folder-1')
  })

  it('does not refetch a folder’s sessions on subsequent expands, and hides them while collapsed', async () => {
    // Given an already-expanded folder
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([CACHE_SESSION])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    await screen.findByText('Cache invalidation')

    // When collapsing it
    await user.click(screen.getByText('System Design'))

    // Then its sessions are hidden again
    expect(screen.queryByText('Cache invalidation')).not.toBeInTheDocument()

    // When expanding it again
    await user.click(screen.getByText('System Design'))

    // Then the sessions reappear and the list is only fetched once
    expect(await screen.findByText('Cache invalidation')).toBeInTheDocument()
    expect(listStudySessionsByFolder).toHaveBeenCalledTimes(1)
  })

  it('only loads the sessions of the folder being expanded, leaving other folders unloaded', async () => {
    // Given two folders, neither expanded yet
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL, SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockImplementation((folderId: string) =>
      Promise.resolve(folderId === 'folder-1' ? [CACHE_SESSION] : []),
    )
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')

    // When expanding only System Design
    await user.click(screen.getByText('System Design'))
    await screen.findByText('Cache invalidation')

    // Then General's sessions are still unloaded — expanding it still
    // triggers its own fetch, proving System Design's load didn't touch it
    await user.click(screen.getByText('General'))
    await waitFor(() => expect(listStudySessionsByFolder).toHaveBeenCalledWith('default'))
  })

  it('calls onSelectSession when a session is clicked', async () => {
    // Given an expanded folder with one session
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([CACHE_SESSION])
    const user = userEvent.setup()
    const { onSelectSession } = renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    await screen.findByText('Cache invalidation')

    // When clicking the session
    await user.click(screen.getByText('Cache invalidation'))

    // Then it is reported to the parent, along with its folder's name
    expect(onSelectSession).toHaveBeenCalledWith(CACHE_SESSION, 'System Design')
  })

  it('lights up the dot and row only for the currently selected active session', async () => {
    // Given an expanded folder with two active sessions, one selected
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([
      CACHE_SESSION,
      LOAD_BALANCING_SESSION,
    ])
    const user = userEvent.setup()
    renderTree({ selectedSessionId: CACHE_SESSION.id })
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    await screen.findByText('Cache invalidation')

    const selectedRow = screen.getByText('Cache invalidation').closest('div')
    const otherRow = screen.getByText('Load balancing').closest('div')
    const selectedDot = selectedRow?.querySelector('span[aria-hidden="true"]')
    const otherDot = otherRow?.querySelector('span[aria-hidden="true"]')

    // Then only the selected session's row and dot carry the selected
    // styling, but the base styling (present on both) survives
    expect(selectedRow).toHaveClass('bg-secondary')
    expect(otherRow).not.toHaveClass('bg-secondary')
    expect(selectedRow).toHaveClass('hover:bg-accent')
    expect(otherRow).toHaveClass('hover:bg-accent')

    expect(selectedDot).toHaveClass('bg-primary')
    expect(selectedDot).toHaveClass('rounded-full')
    expect(otherDot).toHaveClass('bg-muted-foreground')
    expect(otherDot).not.toHaveClass('bg-primary')
    expect(otherDot).toHaveClass('rounded-full')
  })

  it('orders sessions by most-recently-started first', async () => {
    // Given a folder whose sessions are returned oldest-first
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([
      LOAD_BALANCING_SESSION,
      CACHE_SESSION,
    ])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')

    // When expanding the folder
    await user.click(screen.getByText('System Design'))
    await screen.findByText('Cache invalidation')

    // Then the more recently started session is rendered first, even
    // though it was returned second
    const topics = screen
      .getAllByText(/^(Cache invalidation|Load balancing)$/)
      .map((el) => el.textContent)
    expect(topics).toEqual(['Cache invalidation', 'Load balancing'])
  })

  it('renders no session rows while a folder’s sessions are still loading', async () => {
    // Given a folder whose session fetch never resolves during this test
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockReturnValueOnce(new Promise(() => {}))
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')

    // When expanding it (open flips synchronously, before the fetch settles)
    await user.click(screen.getByText('System Design'))

    // Then no session row is rendered yet — only the "New session" control
    await screen.findByText('New session')
    expect(screen.queryAllByRole('button', { name: /options$/ })).toHaveLength(1)
  })

  it('toggling one folder does not affect another folder’s open state', async () => {
    // Given two collapsed folders
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL, SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('General')
    await screen.findByText('System Design')

    const generalHeader = screen.getByText('General').closest('div')
    const systemDesignHeader = screen.getByText('System Design').closest('div')
    const generalChevron = generalHeader?.querySelector('svg')
    const systemDesignChevron = systemDesignHeader?.querySelector('svg')
    expect(generalChevron).toHaveClass('transition-transform')
    expect(generalChevron).not.toHaveClass('rotate-90')
    expect(systemDesignChevron).not.toHaveClass('rotate-90')

    // When expanding only System Design
    await user.click(screen.getByText('System Design'))
    await screen.findByText('New session')

    // Then only its chevron rotates — General's is untouched
    expect(systemDesignChevron).toHaveClass('rotate-90')
    expect(generalChevron).not.toHaveClass('rotate-90')
  })

  it('creates a new folder via the dialog, trimming the name and resetting afterwards', async () => {
    // Given the default folder and a folder creation that succeeds
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL])
    const created: Folder = { id: 'folder-2', name: 'Java', isDefault: false }
    vi.mocked(createFolder).mockResolvedValueOnce(created)
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('General')

    // When opening the dialog, typing a padded name and creating it
    await user.click(screen.getByRole('button', { name: 'New folder' }))
    await user.type(await screen.findByPlaceholderText(/distributed systems/i), '  Java  ')
    await user.click(screen.getByRole('button', { name: 'Create' }))

    // Then it was created with the trimmed name, now appears in the tree,
    // collapsed, and the dialog closed
    expect(createFolder).toHaveBeenCalledWith('Java')
    await waitFor(() => expect(screen.getByText('Java')).toBeInTheDocument())
    expect(screen.queryByPlaceholderText(/distributed systems/i)).not.toBeInTheDocument()
    const javaChevron = screen.getByText('Java').closest('div')?.querySelector('svg')
    expect(javaChevron).not.toHaveClass('rotate-90')

    // When reopening the dialog
    await user.click(screen.getByRole('button', { name: 'New folder' }))

    // Then the name field was reset, not left over from the last creation
    expect(await screen.findByPlaceholderText(/distributed systems/i)).toHaveValue('')
  })

  it('does not create a folder from an empty or whitespace-only name', async () => {
    // Given the default folder and the new-folder dialog open with a
    // whitespace-only name typed in
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('General')
    await user.click(screen.getByRole('button', { name: 'New folder' }))
    const input = await screen.findByPlaceholderText(/distributed systems/i)
    await user.type(input, '   ')

    // Then the Create button is disabled...
    expect(screen.getByRole('button', { name: 'Create' })).toBeDisabled()

    // ...and forcing a submit (bypassing the disabled button) still does
    // not create a folder
    fireEvent.submit(input.closest('form')!)
    expect(createFolder).not.toHaveBeenCalled()
  })

  it('renames a folder inline, trimming the name, without touching other folders', async () => {
    // Given two folders and a rename that succeeds
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL, SYSTEM_DESIGN])
    vi.mocked(renameFolder).mockResolvedValueOnce()
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')

    // When opening its menu, choosing Rename, and submitting a padded name
    await user.click(screen.getByRole('button', { name: 'System Design options' }))
    await user.click(await screen.findByText('Rename'))
    const input = await screen.findByDisplayValue('System Design')
    await user.clear(input)
    await user.type(input, '  Distributed Systems  {Enter}')

    // Then it was renamed with the trimmed name, the new name is shown,
    // and the other folder is untouched
    expect(renameFolder).toHaveBeenCalledWith('folder-1', 'Distributed Systems')
    await waitFor(() => expect(screen.getByText('Distributed Systems')).toBeInTheDocument())
    expect(screen.getByText('General')).toBeInTheDocument()
  })

  it('does not submit a rename when the trimmed name is empty', async () => {
    // Given a folder with its rename input open
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByRole('button', { name: 'System Design options' }))
    await user.click(await screen.findByText('Rename'))
    const input = await screen.findByDisplayValue('System Design')

    // When clearing it and submitting a whitespace-only value
    await user.clear(input)
    await user.type(input, '   {Enter}')

    // Then it was never renamed, and the original name is still shown
    expect(renameFolder).not.toHaveBeenCalled()
    expect(await screen.findByText('System Design')).toBeInTheDocument()
  })

  it('cancels a rename with Escape without saving', async () => {
    // Given a folder with its rename input open and edited
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByRole('button', { name: 'System Design options' }))
    await user.click(await screen.findByText('Rename'))
    const input = await screen.findByDisplayValue('System Design')
    await user.clear(input)
    await user.type(input, 'Distributed Systems')

    // When pressing Escape
    await user.keyboard('{Escape}')

    // Then it was never renamed, and the original name is shown, not the
    // edited-but-uncommitted one
    expect(renameFolder).not.toHaveBeenCalled()
    expect(await screen.findByText('System Design')).toBeInTheDocument()
    expect(screen.queryByText('Distributed Systems')).not.toBeInTheDocument()
  })

  it('submits a rename on blur', async () => {
    // Given a folder with its rename input open and edited
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(renameFolder).mockResolvedValueOnce()
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByRole('button', { name: 'System Design options' }))
    await user.click(await screen.findByText('Rename'))
    const input = await screen.findByDisplayValue('System Design')
    await user.clear(input)
    await user.type(input, 'Distributed Systems')

    // When the input loses focus without pressing Enter
    fireEvent.blur(input)

    // Then it was renamed anyway
    await waitFor(() => expect(renameFolder).toHaveBeenCalledWith('folder-1', 'Distributed Systems'))
    expect(await screen.findByText('Distributed Systems')).toBeInTheDocument()
  })

  it('clicking inside the rename input does not toggle the folder', async () => {
    // Given a folder with its rename input open — note opening "Rename"
    // from the dropdown (a Radix portal) bubbles its own click through the
    // React tree up to the folder header, toggling it open as a side
    // effect; that's incidental to this test, which targets specifically
    // whether a *second* click, landing directly on the input itself, also
    // bubbles and toggles it back closed
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByRole('button', { name: 'System Design options' }))
    await user.click(await screen.findByText('Rename'))
    const input = await screen.findByDisplayValue('System Design')
    await screen.findByText('New session')

    // When clicking directly inside the input
    await user.click(input)

    // Then the folder was not toggled back closed by that click
    expect(screen.getByText('New session')).toBeInTheDocument()
  })

  it('disables deleting the default folder', async () => {
    // Given the default folder
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('General')

    // When opening its menu
    await user.click(screen.getByRole('button', { name: 'General options' }))

    // Then "Delete folder" is disabled
    const deleteItem = await screen.findByText('Delete folder')
    expect(deleteItem.closest('[role="menuitem"]')).toHaveAttribute('data-disabled')
  })

  it('deletes a non-default folder after confirming, leaving other folders in place', async () => {
    // Given two non-default... a default and a non-default folder, and a
    // delete that succeeds
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL, SYSTEM_DESIGN])
    vi.mocked(deleteFolder).mockResolvedValueOnce()
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')

    // When deleting it and confirming
    await user.click(screen.getByRole('button', { name: 'System Design options' }))
    await user.click(await screen.findByText('Delete folder'))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Delete folder' }))

    // Then it was deleted and no longer appears, but General remains
    expect(deleteFolder).toHaveBeenCalledWith('folder-1')
    await waitFor(() => expect(screen.queryByText('System Design')).not.toBeInTheDocument())
    expect(screen.getByText('General')).toBeInTheDocument()
  })

  it('does not delete a folder when the confirmation is cancelled', async () => {
    // Given a non-default folder and its delete confirmation open
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL, SYSTEM_DESIGN])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByRole('button', { name: 'System Design options' }))
    await user.click(await screen.findByText('Delete folder'))
    const dialog = await screen.findByRole('alertdialog')

    // When cancelling
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))

    // Then it was never deleted, it still appears, and the dialog closed
    expect(deleteFolder).not.toHaveBeenCalled()
    expect(screen.getByText('System Design')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
  })

  it('reloads the default folder’s sessions after deleting another folder, if they were already loaded', async () => {
    // Given the default folder already expanded (its sessions loaded) and
    // another folder that gets deleted
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL, SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder)
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([])
    vi.mocked(deleteFolder).mockResolvedValueOnce()
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('General')
    await user.click(screen.getByText('General'))
    await waitFor(() => expect(listStudySessionsByFolder).toHaveBeenCalledWith('default'))

    // When deleting System Design and confirming
    await user.click(screen.getByRole('button', { name: 'System Design options' }))
    await user.click(await screen.findByText('Delete folder'))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Delete folder' }))

    // Then the default folder's sessions are reloaded — fetched twice
    await waitFor(() =>
      expect(vi.mocked(listStudySessionsByFolder).mock.calls.filter(([id]) => id === 'default'))
        .toHaveLength(2),
    )
  })

  it('does not reload the default folder’s sessions after deleting another folder, if they were never loaded', async () => {
    // Given the default folder collapsed (never expanded) and another
    // folder that gets deleted. Note: opening "Delete folder" from the
    // dropdown (a Radix portal) bubbles its own click through the React
    // tree up to System Design's own header, incidentally expanding *it*
    // (not General) and fetching its own sessions — that's unrelated to
    // what this test targets, so it's tolerated via mockResolvedValueOnce.
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL, SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([])
    vi.mocked(deleteFolder).mockResolvedValueOnce()
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('General')

    // When deleting System Design and confirming
    await user.click(screen.getByRole('button', { name: 'System Design options' }))
    await user.click(await screen.findByText('Delete folder'))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Delete folder' }))
    await waitFor(() => expect(screen.queryByText('System Design')).not.toBeInTheDocument())

    // Then the default folder's sessions specifically were never fetched
    expect(listStudySessionsByFolder).not.toHaveBeenCalledWith('default')
  })

  it('starts a new session inline inside a folder, trimming the topic', async () => {
    // Given an expanded, empty folder and a session start that succeeds
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([])
    vi.mocked(startStudySession).mockResolvedValueOnce(CACHE_SESSION)
    const user = userEvent.setup()
    const { onSessionStarted } = renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    await screen.findByText('New session')

    // When starting a session with a padded topic
    await user.click(screen.getByText('New session'))
    await user.type(
      screen.getByPlaceholderText('What do you want to study?'),
      '  Cache invalidation  {Enter}',
    )

    // Then it was started with the trimmed topic, in this folder, and
    // reported to the parent, along with its folder's name
    expect(startStudySession).toHaveBeenCalledWith('Cache invalidation', 'folder-1')
    await waitFor(() =>
      expect(onSessionStarted).toHaveBeenCalledWith(CACHE_SESSION, 'System Design'),
    )
    expect(await screen.findByText('Cache invalidation')).toBeInTheDocument()
  })

  it('does not start a session from an empty or whitespace-only topic, keeping the form open', async () => {
    // Given an expanded, empty folder with the new-session form open
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    await screen.findByText('New session')
    await user.click(screen.getByText('New session'))
    const input = screen.getByPlaceholderText('What do you want to study?')

    // When submitting a whitespace-only topic
    await user.type(input, '   {Enter}')

    // Then no session was started, and the form is still open for retry
    expect(startStudySession).not.toHaveBeenCalled()
    expect(screen.getByPlaceholderText('What do you want to study?')).toBeInTheDocument()
  })

  it('cancels starting a new session with Escape', async () => {
    // Given an expanded, empty folder with the new-session form open and a
    // topic typed in
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    await screen.findByText('New session')
    await user.click(screen.getByText('New session'))
    await user.type(screen.getByPlaceholderText('What do you want to study?'), 'Cache invalidation')

    // When pressing Escape
    await user.keyboard('{Escape}')

    // Then no session was started, the form closed, and the "New session"
    // trigger is back
    expect(startStudySession).not.toHaveBeenCalled()
    expect(screen.queryByPlaceholderText('What do you want to study?')).not.toBeInTheDocument()
    expect(await screen.findByText('New session')).toBeInTheDocument()
  })

  it('starts a session in the correct folder among several, reporting that folder’s name', async () => {
    // Given two folders — General listed first, so a broken `.find` that
    // ignores the id would wrongly grab it instead of System Design
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL, SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockImplementation(() => Promise.resolve([]))
    vi.mocked(startStudySession).mockResolvedValueOnce(CACHE_SESSION)
    const user = userEvent.setup()
    const { onSessionStarted } = renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    await screen.findByText('New session')

    // When starting a session in System Design
    await user.click(screen.getByText('New session'))
    await user.type(
      screen.getByPlaceholderText('What do you want to study?'),
      'Cache invalidation{Enter}',
    )

    // Then the parent is notified with System Design's name, not General's
    await waitFor(() =>
      expect(onSessionStarted).toHaveBeenCalledWith(CACHE_SESSION, 'System Design'),
    )

    // And General — the other folder — never received the new session:
    // expanding it still triggers its own fetch and stays empty
    await user.click(screen.getByText('General'))
    await waitFor(() => expect(listStudySessionsByFolder).toHaveBeenCalledWith('default'))
    const generalSection = screen.getByText('General').closest('div')?.parentElement
    expect(generalSection ? within(generalSection).queryByText('Cache invalidation') : null).toBeNull()
  })

  it('shows a session started while its folder’s sessions were still loading', async () => {
    // Given a folder that is expanded but whose initial session fetch is
    // still pending (open flips synchronously, before the fetch settles)
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockReturnValueOnce(new Promise(() => {}))
    vi.mocked(startStudySession).mockResolvedValueOnce(CACHE_SESSION)
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    await screen.findByText('New session')

    // When starting a session before the pending fetch ever resolves
    await user.click(screen.getByText('New session'))
    await user.type(
      screen.getByPlaceholderText('What do you want to study?'),
      'Cache invalidation{Enter}',
    )

    // Then the new session still shows up, seeded onto a still-null
    // sessions list rather than being silently dropped
    expect(await screen.findByText('Cache invalidation')).toBeInTheDocument()
  })

  it('moves a session to another folder, excluding its current folder from the menu', async () => {
    // Given two folders, one expanded with a session
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL, SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockImplementation((folderId: string) =>
      Promise.resolve(folderId === 'folder-1' ? [CACHE_SESSION] : []),
    )
    vi.mocked(moveStudySession).mockResolvedValueOnce()
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    await screen.findByText('Cache invalidation')

    // When opening the session's "Move to" menu
    await user.click(screen.getByRole('button', { name: 'Cache invalidation options' }))
    const menu = (await screen.findByText('Move to')).closest<HTMLElement>('[role="menu"]')!

    // Then its own folder is not offered as a target
    expect(within(menu).queryByRole('menuitem', { name: 'System Design' })).not.toBeInTheDocument()

    // When moving the session to General
    await user.click(await screen.findByRole('menuitem', { name: 'General' }))

    // Then it was moved
    expect(moveStudySession).toHaveBeenCalledWith('session-1', 'default')
  })

  it('marks session rows as draggable and folder headers as drop targets', async () => {
    // Given two folders, one expanded with a session
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL, SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([CACHE_SESSION])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))

    // Then the session row is a draggable, and both folder headers can
    // receive it
    const sessionRow = (await screen.findByText('Cache invalidation')).closest('div')
    expect(sessionRow).toHaveAttribute('aria-roledescription', 'draggable')
    expect(screen.getByText('General').closest('div')).toBeInTheDocument()
    expect(screen.getByText('System Design').closest('div')).toBeInTheDocument()
  })

  it('drags a session onto another folder, dimming it and highlighting the drop target, and moves it', async () => {
    // Given two folders, one expanded with a session
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL, SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockImplementation((folderId: string) =>
      Promise.resolve(folderId === 'folder-1' ? [CACHE_SESSION] : []),
    )
    vi.mocked(moveStudySession).mockResolvedValueOnce()
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    const sessionRow = (await screen.findByText('Cache invalidation')).closest('div')!
    const generalHeader = screen.getByText('General').closest('div')!
    expect(sessionRow).not.toHaveClass('opacity-40')
    expect(generalHeader).not.toHaveClass('bg-accent')

    // When dragging the session onto General and dropping it there
    await dragSessionOnto(sessionRow, generalHeader)

    // Then it was moved, and the overlay/dimming state cleared afterwards
    await waitFor(() => expect(moveStudySession).toHaveBeenCalledWith('session-1', 'default'))
    await waitFor(() => expect(screen.getAllByText('Cache invalidation')).toHaveLength(1))
    await settleDrag()
  })

  it('does not move a session dropped back onto its own folder', async () => {
    // Given a single folder expanded with a session — the only folder in
    // the tree, so its header is unambiguously the drop target (with more
    // than one folder present, the mocked-rect trick used to simulate a
    // drop in jsdom gives every droppable an identical rect, and ties are
    // broken by registration order rather than by which one the test means
    // to target)
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([CACHE_SESSION])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    const sessionRow = (await screen.findByText('Cache invalidation')).closest('div')!
    const systemDesignHeader = screen.getByText('System Design').closest('div')!

    // When dragging the session and dropping it back onto its own folder
    await dragSessionOnto(sessionRow, systemDesignHeader)

    // Then no move was requested, and the drag state settled back down —
    // waiting for this (rather than ending the test right after pointerUp)
    // gives dnd-kit's own effects time to fully detach their document-level
    // listeners before the component unmounts, so a later test's plain
    // clicks aren't affected by a sensor mid-teardown
    await waitFor(() => expect(sessionRow).not.toHaveClass('opacity-40'))
    expect(moveStudySession).not.toHaveBeenCalled()
    await settleDrag()
  })

  it('ends a drag without moving the session when it is not dropped on any folder', async () => {
    // Given a folder expanded with a session, and no rect stubbing — jsdom's
    // default zero-size rects mean the drag never collides with anything
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([CACHE_SESSION])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    const sessionRow = (await screen.findByText('Cache invalidation')).closest('div')!

    // When starting and ending a drag with nothing under the pointer
    fireEvent.pointerDown(sessionRow, {
      pointerId: 1,
      clientX: 0,
      clientY: 0,
      isPrimary: true,
      button: 0,
    })
    fireEvent.pointerMove(document, {
      pointerId: 1,
      clientX: 10,
      clientY: 10,
      isPrimary: true,
      button: 0,
    })
    await waitFor(() => expect(sessionRow).toHaveClass('opacity-40'))
    fireEvent.pointerUp(document, {
      pointerId: 1,
      clientX: 10,
      clientY: 10,
      isPrimary: true,
      button: 0,
    })

    // Then nothing crashed and no move was requested
    await waitFor(() => expect(sessionRow).not.toHaveClass('opacity-40'))
    expect(moveStudySession).not.toHaveBeenCalled()
    await settleDrag()
  })

  it('clicking the session options button does not also select the session', async () => {
    // Given an expanded folder with one session
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([CACHE_SESSION])
    const user = userEvent.setup()
    const { onSelectSession } = renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    await screen.findByText('Cache invalidation')

    // When clicking the session's options button
    await user.click(screen.getByRole('button', { name: 'Cache invalidation options' }))

    // Then the click did not bubble up and select the session too
    expect(onSelectSession).not.toHaveBeenCalled()
  })

  it('clicking the folder options button does not also toggle the folder', async () => {
    // Given a collapsed folder
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')

    // When clicking the folder's options button
    await user.click(screen.getByRole('button', { name: 'System Design options' }))

    // Then the click did not bubble up and expand the folder too
    expect(listStudySessionsByFolder).not.toHaveBeenCalled()
  })

  it('deletes a session after confirming, notifies the parent, and leaves the folder and its other sessions intact', async () => {
    // Given an expanded folder with two sessions and a delete that succeeds
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([
      CACHE_SESSION,
      LOAD_BALANCING_SESSION,
    ])
    vi.mocked(deleteStudySession).mockResolvedValueOnce()
    const user = userEvent.setup()
    const { onSessionDeleted } = renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    await screen.findByText('Cache invalidation')

    // When deleting only "Cache invalidation" and confirming
    await user.click(screen.getByRole('button', { name: 'Cache invalidation options' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Delete' }))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Delete session' }))

    // Then it was deleted, no longer appears, and the parent was notified
    expect(deleteStudySession).toHaveBeenCalledWith('session-1')
    expect(onSessionDeleted).toHaveBeenCalledWith('session-1')
    await waitFor(() => expect(screen.queryByText('Cache invalidation')).not.toBeInTheDocument())

    // And its sibling session and folder are untouched
    expect(screen.getByText('Load balancing')).toBeInTheDocument()
    expect(screen.getByText('System Design')).toBeInTheDocument()
  })

  it('does not touch another folder’s not-yet-loaded sessions when deleting a session', async () => {
    // Given one expanded folder with a session, and another folder never
    // expanded (its sessions still unloaded)
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL, SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockImplementation((folderId: string) =>
      Promise.resolve(folderId === 'folder-1' ? [CACHE_SESSION] : []),
    )
    vi.mocked(deleteStudySession).mockResolvedValueOnce()
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    await screen.findByText('Cache invalidation')

    // When deleting the session and confirming
    await user.click(screen.getByRole('button', { name: 'Cache invalidation options' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Delete' }))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Delete session' }))
    await waitFor(() => expect(screen.queryByText('Cache invalidation')).not.toBeInTheDocument())

    // Then General's sessions are still unloaded — expanding it still
    // triggers its own fetch
    await user.click(screen.getByText('General'))
    await waitFor(() => expect(listStudySessionsByFolder).toHaveBeenCalledWith('default'))
  })

  it('does not delete a session when the confirmation is cancelled', async () => {
    // Given an expanded folder with one session
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([CACHE_SESSION])
    const user = userEvent.setup()
    const { onSessionDeleted } = renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    await screen.findByText('Cache invalidation')

    // When opening the delete confirmation and cancelling it
    await user.click(screen.getByRole('button', { name: 'Cache invalidation options' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Delete' }))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))

    // Then it was never deleted, and the dialog closed
    expect(deleteStudySession).not.toHaveBeenCalled()
    expect(onSessionDeleted).not.toHaveBeenCalled()
    expect(screen.getByText('Cache invalidation')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
  })
})
