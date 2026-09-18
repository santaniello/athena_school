// Package folder holds the Folder domain model and the Repository port
// infrastructure adapters implement. Folders group study.Session (and later
// other session modes) by theme, mirroring ChatGPT Projects — see
// specs/phases/phase-01-desktop-mvp/10-study-folders.md.
package folder

import "time"

// Folder groups related sessions together, like a ChatGPT project.
type Folder struct {
	ID        string
	Name      string
	CreatedAt time.Time
}
