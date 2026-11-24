package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	applicationprogress "github.com/santaniello/athena/internal/application/progress"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// ProgressReader is the SQLite-backed implementation of the Progress
// dashboard's read port (application/progress.Reader). It only reads: every
// query runs over the tables that already exist, and nothing is copied, so a
// session moved to another folder takes its whole history along and a deleted
// session takes its topic away. See
// specs/phases/phase-05-progress/01-progress-dashboard.md.
type ProgressReader struct {
	db *sql.DB
}

// NewProgressReader creates a ProgressReader backed by db. db must already
// have its migrations applied (see Open).
func NewProgressReader(db *sql.DB) *ProgressReader {
	return &ProgressReader{db: db}
}

// ReadFolders returns the folders in creation order, the order of the Folders
// screen, each with its sessions and what belongs to them. It runs a fixed
// number of queries however many sessions there are: folders, sessions,
// quizzes, quiz attempts, challenge links, concepts, plus the four of
// AttemptRecordRepository for the challenges' attempts.
func (r *ProgressReader) ReadFolders(ctx context.Context) ([]applicationprogress.FolderData, error) {
	q := execer(ctx, r.db)
	folders, err := readProgressFolders(ctx, q)
	if err != nil {
		return nil, err
	}
	sessions, err := readProgressSessions(ctx, q)
	if err != nil {
		return nil, err
	}
	index := make(map[string]int, len(sessions))
	for i, session := range sessions {
		index[session.data.ID] = i
	}
	if err := attachQuizzes(ctx, q, sessions, index); err != nil {
		return nil, err
	}
	if err := r.attachChallenges(ctx, q, sessions, index); err != nil {
		return nil, err
	}
	if err := attachConcepts(ctx, q, sessions, index); err != nil {
		return nil, err
	}
	attachSessionsToFolders(folders, sessions)
	return folders, nil
}

// attachSessionsToFolders puts each session in its folder. The folders and the
// sessions are read by separate queries without a shared snapshot, so a
// session can name a folder created in between; it is left out of this read
// and shows up in the next one.
func attachSessionsToFolders(folders []applicationprogress.FolderData, sessions []*progressSession) {
	folderIndex := make(map[string]int, len(folders))
	for i, folder := range folders {
		folderIndex[folder.ID] = i
	}
	for _, session := range sessions {
		i, ok := folderIndex[session.folderID]
		if !ok {
			continue
		}
		folders[i].Sessions = append(folders[i].Sessions, session.data)
	}
}

// progressSession is a session being assembled, with the folder it is in.
type progressSession struct {
	folderID string
	data     applicationprogress.SessionData
}

func readProgressFolders(ctx context.Context, q sqlExecer) ([]applicationprogress.FolderData, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, name FROM folders ORDER BY rowid`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing folders for progress: %w", err)
	}
	defer func() { _ = rows.Close() }()

	folders := []applicationprogress.FolderData{}
	for rows.Next() {
		var folder applicationprogress.FolderData
		if err := rows.Scan(&folder.ID, &folder.Name); err != nil {
			return nil, fmt.Errorf("sqlite: scanning folder for progress: %w", err)
		}
		folders = append(folders, folder)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating folders for progress: %w", err)
	}
	return folders, nil
}

// readProgressSessions returns the sessions that are in a folder, oldest first.
func readProgressSessions(ctx context.Context, q sqlExecer) ([]*progressSession, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, topic, folder_id, started_at FROM sessions WHERE folder_id IS NOT NULL ORDER BY started_at, rowid`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing sessions for progress: %w", err)
	}
	defer func() { _ = rows.Close() }()

	sessions := []*progressSession{}
	for rows.Next() {
		session := &progressSession{}
		if err := rows.Scan(&session.data.ID, &session.data.Topic, &session.folderID, &session.data.StartedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scanning session for progress: %w", err)
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating sessions for progress: %w", err)
	}
	return sessions, nil
}

// attachQuizzes adds every quiz of the sessions, whatever its origin (chat
// offer or menu), with its attempts oldest first. A quiz of a deleted session
// matches no session and is left out.
func attachQuizzes(ctx context.Context, q sqlExecer, sessions []*progressSession, index map[string]int) error {
	rows, err := q.QueryContext(ctx, `SELECT id, session_id FROM quizzes ORDER BY created_at, id`)
	if err != nil {
		return fmt.Errorf("sqlite: listing quizzes for progress: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type quizPosition struct{ session, quiz int }
	positions := map[string]quizPosition{}
	for rows.Next() {
		var quizID, sessionID string
		if err := rows.Scan(&quizID, &sessionID); err != nil {
			return fmt.Errorf("sqlite: scanning quiz for progress: %w", err)
		}
		i, ok := index[sessionID]
		if !ok {
			continue
		}
		data := &sessions[i].data
		data.Quizzes = append(data.Quizzes, applicationprogress.QuizData{ID: quizID})
		positions[quizID] = quizPosition{session: i, quiz: len(data.Quizzes) - 1}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("sqlite: iterating quizzes for progress: %w", err)
	}
	return attachQuizAttempts(ctx, q, func(quizID string, attempt applicationprogress.QuizAttemptData) {
		if position, ok := positions[quizID]; ok {
			quiz := &sessions[position.session].data.Quizzes[position.quiz]
			quiz.Attempts = append(quiz.Attempts, attempt)
		}
	})
}

func attachQuizAttempts(ctx context.Context, q sqlExecer, add func(quizID string, attempt applicationprogress.QuizAttemptData)) error {
	rows, err := q.QueryContext(ctx,
		`SELECT quiz_id, correct_count, total, finished_at FROM quiz_attempts ORDER BY finished_at, id`)
	if err != nil {
		return fmt.Errorf("sqlite: listing quiz attempts for progress: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var quizID, finishedAt string
		var attempt applicationprogress.QuizAttemptData
		if err := rows.Scan(&quizID, &attempt.CorrectCount, &attempt.Total, &finishedAt); err != nil {
			return fmt.Errorf("sqlite: scanning quiz attempt for progress: %w", err)
		}
		var parseErr error
		if attempt.FinishedAt, parseErr = parseChallengeTime(finishedAt); parseErr != nil {
			return parseErr
		}
		add(quizID, attempt)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("sqlite: iterating quiz attempts for progress: %w", err)
	}
	return nil
}

// attachChallenges adds the challenges generated from each session plus the
// ones linked to it through session_exercises, each once, with all its
// attempts. The UNION drops a pair found on both sides.
func (r *ProgressReader) attachChallenges(ctx context.Context, q sqlExecer, sessions []*progressSession, index map[string]int) error {
	rows, err := q.QueryContext(ctx, `
		SELECT session_id, challenge_id FROM (
			SELECT source_session_id AS session_id, id AS challenge_id FROM challenges WHERE source_session_id IS NOT NULL
			UNION
			SELECT session_id, challenge_id FROM session_exercises
		) ORDER BY session_id, challenge_id`)
	if err != nil {
		return fmt.Errorf("sqlite: listing challenges for progress: %w", err)
	}
	defer func() { _ = rows.Close() }()

	challengeIDs := []string{}
	seen := map[string]bool{}
	for rows.Next() {
		var sessionID, challengeID string
		if err := rows.Scan(&sessionID, &challengeID); err != nil {
			return fmt.Errorf("sqlite: scanning challenge for progress: %w", err)
		}
		i, ok := index[sessionID]
		if !ok {
			continue
		}
		sessions[i].data.Challenges = append(sessions[i].data.Challenges, applicationprogress.ChallengeData{ID: challengeID})
		if !seen[challengeID] {
			seen[challengeID] = true
			challengeIDs = append(challengeIDs, challengeID)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("sqlite: iterating challenges for progress: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("sqlite: closing challenges for progress: %w", err)
	}
	records, err := NewAttemptRecordRepository(r.db).ListByChallenges(ctx, challengeIDs)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		for i := range session.data.Challenges {
			session.data.Challenges[i].Attempts = records[session.data.Challenges[i].ID]
		}
	}
	return nil
}

func attachConcepts(ctx context.Context, q sqlExecer, sessions []*progressSession, index map[string]int) error {
	rows, err := q.QueryContext(ctx,
		`SELECT session_id, status, evidence_count, updated_at FROM session_concepts ORDER BY session_id, name`)
	if err != nil {
		return fmt.Errorf("sqlite: listing concepts for progress: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var sessionID, status, updatedAt string
		var concept applicationprogress.ConceptData
		if err := rows.Scan(&sessionID, &status, &concept.EvidenceCount, &updatedAt); err != nil {
			return fmt.Errorf("sqlite: scanning concept for progress: %w", err)
		}
		concept.Status = domainstudy.ConceptStatus(status)
		var parseErr error
		if concept.UpdatedAt, parseErr = parseConceptTime(updatedAt); parseErr != nil {
			return parseErr
		}
		if i, ok := index[sessionID]; ok {
			sessions[i].data.Concepts = append(sessions[i].data.Concepts, concept)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("sqlite: iterating concepts for progress: %w", err)
	}
	return nil
}

func parseConceptTime(raw string) (time.Time, error) {
	t, err := time.Parse(conceptTimeLayout, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("sqlite: decoding concept time: %w", err)
	}
	return t, nil
}
