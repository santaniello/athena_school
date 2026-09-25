// Package folder holds the Folder use cases: creating, renaming, deleting
// and listing folders. See specs/phases/phase-01-desktop-mvp/10-study-folders.md.
package folder

import (
	"context"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// KnowledgeCascade deletes study sessions together with the knowledge they
// own; see the identically named interface in application/study. Defined
// here (consumer side) per Go convention; implemented by
// *applicationknowledge.Service.
type KnowledgeCascade interface {
	DeleteSessionsWithKnowledge(ctx context.Context, sessionIDs []string, deleteSessions func(ctx context.Context) error) error
}

// Service implements the Folder use cases against a domainfolder.Repository
// and a domainstudy.SessionRepository — deleting a folder needs the latter
// to delete its sessions first.
type Service struct {
	folders   domainfolder.Repository
	sessions  domainstudy.SessionRepository
	knowledge KnowledgeCascade
}

// NewService creates a Service backed by the given ports.
func NewService(folders domainfolder.Repository, sessions domainstudy.SessionRepository, knowledge KnowledgeCascade) *Service {
	return &Service{folders: folders, sessions: sessions, knowledge: knowledge}
}
