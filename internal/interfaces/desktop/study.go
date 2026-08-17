package desktop

import "time"

// Wails events emitted while a study session streams. The UI only ever has
// one study session active at a time, so chunk/error payloads are plain
// strings — no sessionID needed to disambiguate.
const (
	eventStudyChunk = "study:chunk"
	eventStudyDone  = "study:done"
	eventStudyError = "study:error"
)

// StudySessionResult is the desktop-facing DTO returned by
// StartStudySession.
type StudySessionResult struct {
	ID        string `json:"id"`
	Topic     string `json:"topic"`
	StartedAt string `json:"startedAt"`
}

// StartStudySession starts a new study session for topic and returns
// immediately — it does not call the LLM. Call RequestOpeningTurn
// afterwards to stream the assistant's opening turn once the chat view is
// already showing.
func (a *App) StartStudySession(topic string) (StudySessionResult, error) {
	session, err := a.study.Start(a.ctx, topic, "")
	if err != nil {
		return StudySessionResult{}, err
	}
	return StudySessionResult{
		ID:        session.ID,
		Topic:     session.Topic,
		StartedAt: session.StartedAt.Format(time.RFC3339),
	}, nil
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
