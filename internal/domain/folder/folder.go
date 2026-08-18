// Package folder holds the Folder domain model and the Repository port
// infrastructure adapters implement. Folders group study.Session (and later
// other session modes) by theme, mirroring ChatGPT Projects — see
// specs/phases/phase-01-desktop-mvp/10-study-folders.md.
package folder

import "time"

// DefaultFolderID is the fixed ID of the default folder every session falls
// back to when no folder is explicitly chosen. It is seeded by the sqlite
// migrations and can never be deleted.
const DefaultFolderID = "default"

// Folder groups related sessions together, like a ChatGPT project.
type Folder struct {
	ID        string
	Name      string
	IsDefault bool
	CreatedAt time.Time
}
