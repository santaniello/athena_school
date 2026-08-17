package desktop

import (
	"time"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Wails events emitted while a study session streams. The UI only ever has
// one study session active at a time, so chunk/error payloads are plain
// strings — no sessionID needed to disambiguate.
const (
	eventStudyChunk = "study:chunk"
	eventStudyDone  = "study:done"
	eventStudyError = "study:error"
)

// StudySessionResult is the desktop-facing DTO for a study session.
type StudySessionResult struct {
	ID        string `json:"id"`
	Topic     string `json:"topic"`
	FolderID  string `json:"folderId"`
	StartedAt string `json:"startedAt"`
	EndedAt   string `json:"endedAt"` // empty when the session is still open
}

// StudyMessageResult is the desktop-facing DTO for a single message in a
// study session's history.
type StudyMessageResult struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

// StudySessionHistoryResult is returned by ResumeStudySession: the session
// itself plus every message exchanged in it so far.
type StudySessionHistoryResult struct {
	Session  StudySessionResult   `json:"session"`
	Messages []StudyMessageResult `json:"messages"`
}

func toStudySessionResult(s domainstudy.Session) StudySessionResult {
	result := StudySessionResult{
		ID:        s.ID,
		Topic:     s.Topic,
		FolderID:  s.FolderID,
		StartedAt: s.StartedAt.Format(time.RFC3339),
	}
	if !s.EndedAt.IsZero() {
		result.EndedAt = s.EndedAt.Format(time.RFC3339)
	}
	return result
}

// StartStudySession starts a new study session for topic inside folderID
// and returns immediately — it does not call the LLM. If folderID is
// blank, the session falls back to the default folder. Call
// RequestOpeningTurn afterwards to stream the assistant's opening turn
// once the chat view is already showing.
func (a *App) StartStudySession(topic, folderID string) (StudySessionResult, error) {
	session, err := a.study.Start(a.ctx, topic, folderID)
	if err != nil {
		return StudySessionResult{}, err
	}
	return toStudySessionResult(session), nil
}

// ResumeStudySession returns sessionID's full message history, reopening
// it first if it had been ended, so the user can keep chatting in it.
func (a *App) ResumeStudySession(sessionID string) (StudySessionHistoryResult, error) {
	session, history, err := a.study.Resume(a.ctx, sessionID)
	if err != nil {
		return StudySessionHistoryResult{}, err
	}
	messages := make([]StudyMessageResult, len(history))
	for i, m := range history {
		messages[i] = StudyMessageResult{
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		}
	}
	return StudySessionHistoryResult{Session: toStudySessionResult(session), Messages: messages}, nil
}

// MoveStudySession reassigns sessionID to folderID.
func (a *App) MoveStudySession(sessionID, folderID string) error {
	return a.study.MoveToFolder(a.ctx, sessionID, folderID)
}

// ListStudySessionsByFolder returns every session in the given folder.
func (a *App) ListStudySessionsByFolder(folderID string) ([]StudySessionResult, error) {
	sessions, err := a.study.ListSessionsByFolder(a.ctx, folderID)
	if err != nil {
		return nil, err
	}
	results := make([]StudySessionResult, len(sessions))
	for i, s := range sessions {
		results[i] = toStudySessionResult(s)
	}
	return results, nil
}

// RequestOpeningTurn streams the assistant's opening turn for sessionID
// (about topic) via the "study:chunk" event as it arrives. It blocks until
// the stream completes, then emits "study:done" (or "study:error" on
// failure).
func (a *App) RequestOpeningTurn(sessionID, topic string) error {
	err := a.study.RequestOpeningTurn(a.ctx, sessionID, topic, func(chunk string) error {
		a.emit(a.ctx, eventStudyChunk, chunk)
		return nil
	})
	if err != nil {
		a.emit(a.ctx, eventStudyError, err.Error())
		return err
	}
	a.emit(a.ctx, eventStudyDone)
	return nil
}

// SendStudyMessage appends content to the session identified by sessionID
// (about topic) and streams the LLM's reply via "study:chunk", ending with
// "study:done" (or "study:error" on failure).
func (a *App) SendStudyMessage(sessionID, topic, content string) error {
	err := a.study.SendMessage(a.ctx, sessionID, topic, content, func(chunk string) error {
		a.emit(a.ctx, eventStudyChunk, chunk)
		return nil
	})
	if err != nil {
		a.emit(a.ctx, eventStudyError, err.Error())
		return err
	}
	a.emit(a.ctx, eventStudyDone)
	return nil
}

// EndStudySession closes sessionID gracefully.
func (a *App) EndStudySession(sessionID string) error {
	return a.study.End(a.ctx, sessionID)
}

// DeleteStudySession permanently deletes sessionID and every message in it.
// Unlike EndStudySession, this cannot be undone.
func (a *App) DeleteStudySession(sessionID string) error {
	return a.study.DeleteSession(a.ctx, sessionID)
}
