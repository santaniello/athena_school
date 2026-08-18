package desktop

// FolderResult is the desktop-facing DTO for a Folder.
type FolderResult struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

// CreateFolder creates a new folder named name.
func (a *App) CreateFolder(name string) (FolderResult, error) {
	f, err := a.folder.CreateFolder(a.ctx, name)
	if err != nil {
		return FolderResult{}, err
	}
	return FolderResult{ID: f.ID, Name: f.Name, IsDefault: f.IsDefault}, nil
}

// RenameFolder renames the folder with the given id, including the default
// folder.
func (a *App) RenameFolder(id, name string) error {
	return a.folder.RenameFolder(a.ctx, id, name)
}

// DeleteFolder deletes the folder with the given id, moving its sessions to
// the default folder first. The default folder itself cannot be deleted.
func (a *App) DeleteFolder(id string) error {
	return a.folder.DeleteFolder(a.ctx, id)
}

// ListFolders returns every folder.
func (a *App) ListFolders() ([]FolderResult, error) {
	folders, err := a.folder.ListFolders(a.ctx)
	if err != nil {
		return nil, err
	}
	results := make([]FolderResult, len(folders))
	for i, f := range folders {
		results[i] = FolderResult{ID: f.ID, Name: f.Name, IsDefault: f.IsDefault}
	}
	return results, nil
}
