// Package folder holds the Folder domain model and the Repository port
// infrastructure adapters implement. Folders group study.Session (and later
// other session modes) by theme, mirroring ChatGPT Projects — see
// specs/phases/phase-01-desktop-mvp/10-study-folders.md.
package folder

import "time"

// Folder groups related sessions together, like a ChatGPT project.
type Folder struct {
	ID   string
	Name string
	// ArchLabEnabled is true when the user marked this folder as holding
	// architecture study: only then can a challenge be generated from its
	// sessions. A new folder starts disabled. See
	// specs/phases/phase-07-improve-arch-lab/01-arch-lab-per-folder.md.
	ArchLabEnabled bool
	CreatedAt      time.Time
}
