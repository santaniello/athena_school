import { createRef } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Folder } from '@/lib/folder'
import { createFolder, deleteFolder, listFolders, renameFolder } from '@/lib/folder'
import type { StudySession } from '@/lib/study'
import { listStudySessionsByFolder, moveStudySession, startStudySession } from '@/lib/study'
import { StudyFolderTree, type StudyFolderTreeHandle } from './study-folder-tree'

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
}))

const GENERAL: Folder = { id: 'default', name: 'General', isDefault: true }
const SYSTEM_DESIGN: Folder = { id: 'folder-1', name: 'System Design', isDefault: false }

const CACHE_SESSION: StudySession = {
  id: 'session-1',
  topic: 'Cache invalidation',
  folderId: 'folder-1',
  startedAt: '2026-08-16T10:00:00Z',
  endedAt: '',
}
const CONCURRENCY_SESSION: StudySession = {
  id: 'session-2',
  topic: 'Concurrency patterns',
  folderId: 'folder-1',
  startedAt: '2026-08-15T10:00:00Z',
  endedAt: '2026-08-15T11:00:00Z',
}

function renderTree(props?: {
  selectedSessionId?: string | null
  onSelectSession?: (session: StudySession) => void
  onSessionStarted?: (session: StudySession) => void
}) {
  const onSelectSession = props?.onSelectSession ?? vi.fn()
  const onSessionStarted = props?.onSessionStarted ?? vi.fn()
  const ref = createRef<StudyFolderTreeHandle>()
  render(
    <StudyFolderTree
      ref={ref}
      selectedSessionId={props?.selectedSessionId ?? null}
      onSelectSession={onSelectSession}
      onSessionStarted={onSessionStarted}
    />,
  )
  return { onSelectSession, onSessionStarted, ref }
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

  it('lazily loads and shows a folder’s sessions when expanded', async () => {
    // Given a folder with two sessions
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([CACHE_SESSION, CONCURRENCY_SESSION])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')

    // When expanding the folder
    await user.click(screen.getByText('System Design'))

    // Then its sessions are fetched and shown
    expect(await screen.findByText('Cache invalidation')).toBeInTheDocument()
    expect(screen.getByText('Concurrency patterns')).toBeInTheDocument()
    expect(listStudySessionsByFolder).toHaveBeenCalledWith('folder-1')
  })

  it('does not refetch a folder’s sessions on subsequent expands', async () => {
    // Given an already-expanded folder
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([CACHE_SESSION])
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    await screen.findByText('Cache invalidation')

    // When collapsing and expanding it again
    await user.click(screen.getByText('System Design'))
    await user.click(screen.getByText('System Design'))

    // Then the session list is only fetched once
    expect(listStudySessionsByFolder).toHaveBeenCalledTimes(1)
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

    // Then it is reported to the parent
    expect(onSelectSession).toHaveBeenCalledWith(CACHE_SESSION)
  })

  it('creates a new folder via the dialog', async () => {
    // Given the default folder and a folder creation that succeeds
    vi.mocked(listFolders).mockResolvedValueOnce([GENERAL])
    const created: Folder = { id: 'folder-2', name: 'Java', isDefault: false }
    vi.mocked(createFolder).mockResolvedValueOnce(created)
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('General')

    // When opening the dialog, typing a name and creating it
    await user.click(screen.getByRole('button', { name: 'New folder' }))
    await user.type(await screen.findByPlaceholderText(/distributed systems/i), 'Java')
    await user.click(screen.getByRole('button', { name: 'Create' }))

    // Then it was created and now appears in the tree
    expect(createFolder).toHaveBeenCalledWith('Java')
    await waitFor(() => expect(screen.getByText('Java')).toBeInTheDocument())
  })

  it('renames a folder inline', async () => {
    // Given a folder and a rename that succeeds
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(renameFolder).mockResolvedValueOnce()
    const user = userEvent.setup()
    renderTree()
    await screen.findByText('System Design')

    // When opening its menu, choosing Rename, and submitting a new name
    await user.click(screen.getByRole('button', { name: 'System Design options' }))
    await user.click(await screen.findByText('Rename'))
    const input = await screen.findByDisplayValue('System Design')
    await user.clear(input)
    await user.type(input, 'Distributed Systems{Enter}')

    // Then it was renamed and the new name is shown
    expect(renameFolder).toHaveBeenCalledWith('folder-1', 'Distributed Systems')
    await waitFor(() => expect(screen.getByText('Distributed Systems')).toBeInTheDocument())
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

  it('deletes a non-default folder after confirming', async () => {
    // Given a non-default folder and a delete that succeeds
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

    // Then it was deleted and no longer appears
    expect(deleteFolder).toHaveBeenCalledWith('folder-1')
    await waitFor(() => expect(screen.queryByText('System Design')).not.toBeInTheDocument())
  })

  it('starts a new session inline inside a folder', async () => {
    // Given an expanded, empty folder and a session start that succeeds
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder).mockResolvedValueOnce([])
    vi.mocked(startStudySession).mockResolvedValueOnce(CACHE_SESSION)
    const user = userEvent.setup()
    const { onSessionStarted } = renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    await screen.findByText('New session')

    // When starting a session with a topic
    await user.click(screen.getByText('New session'))
    await user.type(
      screen.getByPlaceholderText('What do you want to study?'),
      'Cache invalidation{Enter}',
    )

    // Then it was started in this folder and reported to the parent
    expect(startStudySession).toHaveBeenCalledWith('Cache invalidation', 'folder-1')
    await waitFor(() => expect(onSessionStarted).toHaveBeenCalledWith(CACHE_SESSION))
    expect(await screen.findByText('Cache invalidation')).toBeInTheDocument()
  })

  it('moves a session to another folder', async () => {
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

    // When moving the session to General
    await user.click(screen.getByRole('button', { name: 'Cache invalidation options' }))
    await screen.findByText('Move to')
    await user.click(await screen.findByRole('menuitem', { name: 'General' }))

    // Then it was moved
    expect(moveStudySession).toHaveBeenCalledWith('session-1', 'default')
  })

  it('exposes refreshFolder to re-fetch an already-loaded folder', async () => {
    // Given an expanded folder whose session has since ended elsewhere (in
    // the chat view, outside this tree)
    vi.mocked(listFolders).mockResolvedValueOnce([SYSTEM_DESIGN])
    vi.mocked(listStudySessionsByFolder)
      .mockResolvedValueOnce([CACHE_SESSION])
      .mockResolvedValueOnce([{ ...CACHE_SESSION, endedAt: '2026-08-16T11:00:00Z' }])
    const user = userEvent.setup()
    const { ref } = renderTree()
    await screen.findByText('System Design')
    await user.click(screen.getByText('System Design'))
    await screen.findByText('Cache invalidation')

    // When refreshFolder is called imperatively
    await act(async () => {
      ref.current?.refreshFolder('folder-1')
    })

    // Then the session list was re-fetched
    await waitFor(() => expect(listStudySessionsByFolder).toHaveBeenCalledTimes(2))
  })
})
