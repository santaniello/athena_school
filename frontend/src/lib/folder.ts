import {
  CreateFolder,
  DeleteFolder,
  ListFolders,
  RenameFolder,
  SetFolderArchLabEnabled,
} from '../../wailsjs/go/desktop/App'

export interface Folder {
  id: string
  name: string
  // True when the user marked the folder as holding architecture study: only
  // then can an Architecture Lab challenge be generated from its sessions.
  archLabEnabled: boolean
}

export async function createFolder(name: string): Promise<Folder> {
  const result = await CreateFolder(name)
  return { id: result.id, name: result.name, archLabEnabled: result.archLabEnabled }
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
    archLabEnabled: result.archLabEnabled,
  }))
}

export async function setFolderArchLabEnabled(id: string, enabled: boolean): Promise<void> {
  await SetFolderArchLabEnabled(id, enabled)
}
