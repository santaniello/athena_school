package study

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Start opens a new study session for topic and goal inside folderID and
// persists it. folderID is required — there is no fallback folder any
// session can land in. goal is required — it is what buildSystemPrompt renders into the
// system prompt's "Goal:" line for this session, replacing what used to be
// a profile-wide UserProfile.Goals. It does not call the LLM: the caller
// requests the opening turn separately via RequestOpeningTurn, once the
// session already exists, so the UI can switch to the chat view immediately
// instead of waiting for the entire opening response before showing
// anything.
func (s *Service) Start(ctx context.Context, topic, folderID, goal string) (domainstudy.Session, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return domainstudy.Session{}, ErrTopicRequired
	}

	goal = strings.TrimSpace(goal)
	if goal == "" {
		return domainstudy.Session{}, ErrGoalRequired
	}

	folderID = strings.TrimSpace(folderID)
	if folderID == "" {
		return domainstudy.Session{}, ErrFolderRequired
	}
	if _, err := s.folders.GetByID(ctx, folderID); err != nil {
		return domainstudy.Session{}, fmt.Errorf("study: finding folder: %w", err)
	}

	session := domainstudy.Session{
		ID:        uuid.NewString(),
		Topic:     topic,
		Mode:      domainstudy.ModeStudy,
		FolderID:  folderID,
		Goal:      goal,
		StartedAt: time.Now().UTC(),
		Context:   domainstudy.ContextUsage{State: domainstudy.ContextStateNormal},
	}
	if err := s.sessions.Create(ctx, session); err != nil {
		return domainstudy.Session{}, fmt.Errorf("study: creating session: %w", err)
	}

	return session, nil
}
