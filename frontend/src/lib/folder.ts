import { CreateFolder, DeleteFolder, ListFolders, RenameFolder } from '../../wailsjs/go/desktop/App'

export interface Folder {
  id: string
  name: string
  isDefault: boolean
}

export async function createFolder(name: string): Promise<Folder> {
  const result = await CreateFolder(name)
  return { id: result.id, name: result.name, isDefault: result.isDefault }
}

export async function renameFolder(id: string, name: string): Promise<void> {
  await RenameFolder(id, name)
}

export async function deleteFolder(id: string): Promise<void> {
  await DeleteFolder(id)
}

export async function listFolders(): Promise<Folder[]> {
  const results = await ListFolders()
  return results.map((result) => ({
    id: result.id,
    name: result.name,
    isDefault: result.isDefault,
  }))
}
