package desktop

import (
	"time"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Wails events emitted while a study session streams. Every payload is
// session-scoped so the frontend can ignore events for a session other
// than the one currently displayed — see
// specs/phases/phase-02-knowledge-engine/05-rag-integration.md.
const (
	eventStudyChunk   = "study:chunk"
	eventStudyDone    = "study:done"
	eventStudyError   = "study:error"
	eventStudySources = "study:sources"
)

// StudyChunkEvent is study:chunk's payload.
type StudyChunkEvent struct {
	SessionID string `json:"sessionId"`
	Content   string `json:"content"`
}

// StudyDoneEvent is study:done's payload.
type StudyDoneEvent struct {
	SessionID string `json:"sessionId"`
}

// StudyErrorEvent is study:error's payload.
type StudyErrorEvent struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
}

// StudySourceResult is the desktop-facing DTO for one local source the
// model received. It deliberately omits internal IDs and the full
// excerpt — see the domain Source it is built from.
type StudySourceResult struct {
	SourceType string  `json:"sourceType"`
	FilePath   string  `json:"filePath"`
	Heading    string  `json:"heading"`
	Concept    string  `json:"concept"`
	Score      float32 `json:"score"`
}

// StudySourcesEvent is study:sources' payload.
type StudySourcesEvent struct {
	SessionID string              `json:"sessionId"`
	Sources   []StudySourceResult `json:"sources"`
}

// toStudySourceResults always returns a non-nil slice, so an empty source
// list marshals to "[]", never "null".
func toStudySourceResults(sources []domainknowledge.Source) []StudySourceResult {
	results := make([]StudySourceResult, len(sources))
	for i, s := range sources {
		results[i] = StudySourceResult{
			SourceType: s.SourceType,
			FilePath:   s.FilePath,
			Heading:    s.Heading,
			Concept:    s.Concept,
			Score:      s.Score,
		}
	}
	return results
}

// StudySessionResult is the desktop-facing DTO for a study session.
type StudySessionResult struct {
	ID        string `json:"id"`
	Topic     string `json:"topic"`
	FolderID  string `json:"folderId"`
	StartedAt string `json:"startedAt"`
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
	return StudySessionResult{
		ID:        s.ID,
		Topic:     s.Topic,
		FolderID:  s.FolderID,
		StartedAt: s.StartedAt.Format(time.RFC3339),
	}
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

// ResumeStudySession returns sessionID's full message history, so the user
// can keep chatting in it.
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
// failure). The opening turn performs no retrieval and never emits
// "study:sources".
func (a *App) RequestOpeningTurn(sessionID, topic string) error {
	err := a.study.RequestOpeningTurn(a.ctx, sessionID, topic, func(chunk string) error {
		a.emit(a.ctx, eventStudyChunk, StudyChunkEvent{SessionID: sessionID, Content: chunk})
		return nil
	})
	if err != nil {
		a.emit(a.ctx, eventStudyError, StudyErrorEvent{SessionID: sessionID, Message: err.Error()})
		return err
	}
	a.emit(a.ctx, eventStudyDone, StudyDoneEvent{SessionID: sessionID})
	return nil
}

// SendStudyMessage appends content to the session identified by sessionID
// (about topic), following sourceMode's local-knowledge retrieval policy,
// and streams the LLM's reply via "study:chunk", ending with "study:done"
// (or "study:error" on failure). "study:sources" is emitted exactly once
// per call, before any "study:chunk".
func (a *App) SendStudyMessage(sessionID, topic, content, sourceMode string) error {
	err := a.study.SendMessage(a.ctx, sessionID, topic, content, sourceMode,
		func(sources []domainknowledge.Source) error {
			a.emit(a.ctx, eventStudySources, StudySourcesEvent{SessionID: sessionID, Sources: toStudySourceResults(sources)})
			return nil
		},
		func(chunk string) error {
			a.emit(a.ctx, eventStudyChunk, StudyChunkEvent{SessionID: sessionID, Content: chunk})
			return nil
		},
	)
	if err != nil {
		a.emit(a.ctx, eventStudyError, StudyErrorEvent{SessionID: sessionID, Message: err.Error()})
		return err
	}
	a.emit(a.ctx, eventStudyDone, StudyDoneEvent{SessionID: sessionID})
	return nil
}

// DeleteStudySession permanently deletes sessionID and every message in it.
// This cannot be undone.
func (a *App) DeleteStudySession(sessionID string) error {
	return a.study.DeleteSession(a.ctx, sessionID)
}
