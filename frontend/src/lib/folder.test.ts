import { describe, expect, it, vi } from 'vitest'
import { CreateFolder, DeleteFolder, ListFolders, RenameFolder } from '../../wailsjs/go/desktop/App'
import { createFolder, deleteFolder, listFolders, renameFolder } from './folder'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  CreateFolder: vi.fn(),
  RenameFolder: vi.fn(),
  DeleteFolder: vi.fn(),
  ListFolders: vi.fn(),
}))

describe('createFolder', () => {
  it('returns the created folder', async () => {
    // Given a CreateFolder call that succeeds
    vi.mocked(CreateFolder).mockResolvedValueOnce({
      id: 'folder-1',
      name: 'System Design',
      isDefault: false,
    } as never)

    // When creating a folder
    const folder = await createFolder('System Design')

    // Then it forwarded the name and returned the created folder
    expect(CreateFolder).toHaveBeenCalledWith('System Design')
    expect(folder).toEqual({ id: 'folder-1', name: 'System Design', isDefault: false })
  })
})

describe('renameFolder', () => {
  it('forwards the id and new name', async () => {
    // Given a RenameFolder call that succeeds
    vi.mocked(RenameFolder).mockResolvedValueOnce()

    // When renaming a folder
    await renameFolder('folder-1', 'New name')

    // Then it forwarded both arguments
    expect(RenameFolder).toHaveBeenCalledWith('folder-1', 'New name')
  })
})

describe('deleteFolder', () => {
  it('forwards the id', async () => {
    // Given a DeleteFolder call that succeeds
    vi.mocked(DeleteFolder).mockResolvedValueOnce()

    // When deleting a folder
    await deleteFolder('folder-1')

    // Then it forwarded the id
    expect(DeleteFolder).toHaveBeenCalledWith('folder-1')
  })
})

describe('listFolders', () => {
  it('returns every folder', async () => {
    // Given a ListFolders call that returns two folders
    vi.mocked(ListFolders).mockResolvedValueOnce([
      { id: 'default', name: 'General', isDefault: true },
      { id: 'folder-1', name: 'System Design', isDefault: false },
    ] as never)

    // When listing folders
    const folders = await listFolders()

    // Then every folder is returned
    expect(folders).toEqual([
      { id: 'default', name: 'General', isDefault: true },
      { id: 'folder-1', name: 'System Design', isDefault: false },
    ])
  })
})
