// Package folder holds the Folder use cases: creating, renaming, deleting
// and listing folders. See specs/phases/phase-01-desktop-mvp/10-study-folders.md.
package folder

import (
	domainfolder "github.com/santaniello/athena/internal/domain/folder"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Service implements the Folder use cases against a domainfolder.Repository
// and a domainstudy.SessionRepository — deleting a folder needs the latter
// to reassign its sessions to the default folder first.
type Service struct {
	folders  domainfolder.Repository
	sessions domainstudy.SessionRepository
}

// NewService creates a Service backed by the given ports.
func NewService(folders domainfolder.Repository, sessions domainstudy.SessionRepository) *Service {
	return &Service{folders: folders, sessions: sessions}
}
