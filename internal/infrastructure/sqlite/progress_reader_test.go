package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationprogress "github.com/santaniello/athena/internal/application/progress"
	"github.com/santaniello/athena/internal/domain/exercise"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

var progressBase = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

func newProgressFixture(t *testing.T) (*ProgressReader, *sql.DB) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewProgressReader(db), db
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	_, err := db.Exec(query, args...)
	require.NoError(t, err)
}

func addFolder(t *testing.T, db *sql.DB, id, name string) {
	t.Helper()
	mustExec(t, db, `INSERT INTO folders (id, name, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, id, name)
}

func addSession(t *testing.T, db *sql.DB, id, topic, folderID string, startedAt time.Time) {
	t.Helper()
	mustExec(t, db, `INSERT INTO sessions (id, topic, mode, folder_id, started_at) VALUES (?, ?, 'socratic', ?, ?)`,
		id, topic, folderID, startedAt)
}

func addQuiz(t *testing.T, db *sql.DB, id, sessionID string, messageID any) {
	t.Helper()
	mustExec(t, db, `INSERT INTO quizzes (id, session_id, session_title, difficulty, created_at, message_id)
		VALUES (?, ?, 'title', 'INTERMEDIATE', ?, ?)`, id, sessionID, formatChallengeTime(progressBase), messageID)
}

func addQuizAttempt(t *testing.T, db *sql.DB, id, quizID string, correct, total, day int) {
	t.Helper()
	mustExec(t, db, `INSERT INTO quiz_attempts (id, quiz_id, correct_count, total, score, finished_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, quizID, correct, total, float64(correct)/float64(total), formatChallengeTime(progressBase.AddDate(0, 0, day)))
}

func addChallenge(t *testing.T, db *sql.DB, id, sourceSessionID string) {
	t.Helper()
	challenge := newTestChallenge(id)
	challenge.SourceSessionID = sourceSessionID
	require.NoError(t, NewChallengeRepository(db).Create(context.Background(), challenge))
}

func linkChallenge(t *testing.T, db *sql.DB, linkID, sessionID, challengeID string) {
	t.Helper()
	mustExec(t, db, `INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, ?, 'assistant', 'x', CURRENT_TIMESTAMP)`,
		"message-"+linkID, sessionID)
	mustExec(t, db, `INSERT INTO session_exercises (id, session_id, challenge_id, message_id, created_at) VALUES (?, ?, ?, ?, 'now')`,
		linkID, sessionID, challengeID, "message-"+linkID)
}

// addRubric stores a one-criterion rubric version for the challenge.
func addRubric(t *testing.T, db *sql.DB, versionID, challengeID string) {
	t.Helper()
	mustExec(t, db, `INSERT INTO rubric_versions (id, challenge_id, version, base_version, created_at) VALUES (?, ?, 1, 'v1', 'now')`,
		versionID, challengeID)
	mustExec(t, db, `INSERT INTO rubric_criteria (id, rubric_version_id, position, key, name, description, weight, source, mandatory, levels)
		VALUES (?, ?, 0, 'k', 'K', 'd', 100, 'BASE', 0, '[]')`, "crit-"+versionID, versionID)
}

// addLabAttempt stores an attempt with the given evaluations; a score below
// zero leaves that evaluation out.
func addLabAttempt(t *testing.T, db *sql.DB, id, challengeID, versionID string, day, diagramScore, understandingScore int) {
	t.Helper()
	mustExec(t, db, `INSERT INTO challenge_attempts (id, challenge_id, rubric_version_id, state, started_at, deadline, submitted_at)
		VALUES (?, ?, ?, 'FINISHED', ?, ?, ?)`, id, challengeID, versionID,
		formatChallengeTime(progressBase.AddDate(0, 0, day)),
		formatChallengeTime(progressBase.AddDate(0, 0, day).Add(time.Hour)),
		formatChallengeTime(progressBase.AddDate(0, 0, day).Add(30*time.Minute)))
	for kind, score := range map[exercise.EvaluationKind]int{
		exercise.EvaluationKindDiagram:       diagramScore,
		exercise.EvaluationKindUnderstanding: understandingScore,
	} {
		if score < 0 {
			continue
		}
		evaluationID := id + "-" + string(kind)
		mustExec(t, db, `INSERT INTO evaluations (id, attempt_id, kind, model, score, summary, strengths, gaps, recommendations, created_at)
			VALUES (?, ?, ?, 'm', ?, 's', '[]', '[]', '[]', 'now')`, evaluationID, id, string(kind), score)
		mustExec(t, db, `INSERT INTO criterion_results (evaluation_id, criterion_id, level, justification, evidence, insufficient_evidence)
			VALUES (?, ?, 4, 'j', 'e', 0)`, evaluationID, "crit-"+versionID)
	}
}

func sessionsOf(t *testing.T, reader *ProgressReader, folderID string) []applicationprogress.SessionData {
	t.Helper()
	folders, err := reader.ReadFolders(context.Background())
	require.NoError(t, err)
	for _, folder := range folders {
		if folder.ID == folderID {
			return folder.Sessions
		}
	}
	require.Failf(t, "folder not found", "folder %s", folderID)
	return nil
}

func sessionByID(t *testing.T, sessions []applicationprogress.SessionData, id string) applicationprogress.SessionData {
	t.Helper()
	for _, session := range sessions {
		if session.ID == id {
			return session
		}
	}
	require.Failf(t, "session not found", "session %s", id)
	return applicationprogress.SessionData{}
}

func progressChallengeIDs(challenges []applicationprogress.ChallengeData) []string {
	ids := make([]string, len(challenges))
	for i, challenge := range challenges {
		ids[i] = challenge.ID
	}
	return ids
}

func TestProgressReader_ReadFolders_returnsNothingWithoutFolders(t *testing.T) {
	// Given an empty database
	reader, _ := newProgressFixture(t)

	// When reading
	folders, err := reader.ReadFolders(context.Background())

	// Then there are no folders
	require.NoError(t, err)
	assert.Empty(t, folders)
}

func TestProgressReader_ReadFolders_keepsTheCreationOrderAndReturnsEmptyFoldersEmpty(t *testing.T) {
	// Given three folders created in a non-alphabetical order, the last one without sessions
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f-b", "Beta")
	addFolder(t, db, "f-a", "Alpha")
	addFolder(t, db, "f-empty", "Empty")
	addSession(t, db, "s1", "Cache", "f-b", progressBase)

	// When reading
	folders, err := reader.ReadFolders(context.Background())

	// Then they come in creation order, each with its own sessions
	require.NoError(t, err)
	require.Len(t, folders, 3)
	assert.Equal(t, []string{"f-b", "f-a", "f-empty"}, []string{folders[0].ID, folders[1].ID, folders[2].ID})
	assert.Equal(t, "Beta", folders[0].Name)
	assert.Len(t, folders[0].Sessions, 1)
	assert.Empty(t, folders[1].Sessions)
	assert.Empty(t, folders[2].Sessions)
}

func TestProgressReader_ReadFolders_returnsTheSessionIdentity(t *testing.T) {
	// Given a session
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "System Design")
	addSession(t, db, "s1", "Cache", "f1", progressBase)

	// When reading
	sessions := sessionsOf(t, reader, "f1")

	// Then it carries its topic and start
	require.Len(t, sessions, 1)
	assert.Equal(t, "s1", sessions[0].ID)
	assert.Equal(t, "Cache", sessions[0].Topic)
	assert.True(t, progressBase.Equal(sessions[0].StartedAt), "got %s", sessions[0].StartedAt)
}

func TestProgressReader_ReadFolders_movesTheWholeHistoryWithTheSession(t *testing.T) {
	// Given a session with a quiz attempt, a challenge and a concept, in folder 1
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "One")
	addFolder(t, db, "f2", "Two")
	addSession(t, db, "s1", "Cache", "f1", progressBase)
	addQuiz(t, db, "q1", "s1", nil)
	addQuizAttempt(t, db, "qa1", "q1", 8, 10, 1)
	addChallenge(t, db, "c1", "s1")
	mustExec(t, db, `INSERT INTO session_concepts (session_id, name, status, evidence_count, updated_at) VALUES ('s1', 'ttl', 'NEEDS_REVIEW', 2, ?)`,
		progressBase.UTC().Format(conceptTimeLayout))

	// When it moves to folder 2
	mustExec(t, db, `UPDATE sessions SET folder_id = 'f2' WHERE id = 's1'`)

	// Then folder 1 is empty and folder 2 holds the session with everything it had
	assert.Empty(t, sessionsOf(t, reader, "f1"))
	moved := sessionByID(t, sessionsOf(t, reader, "f2"), "s1")
	require.Len(t, moved.Quizzes, 1)
	assert.Len(t, moved.Quizzes[0].Attempts, 1)
	assert.Equal(t, []string{"c1"}, progressChallengeIDs(moved.Challenges))
	require.Len(t, moved.Concepts, 1)
}

func TestProgressReader_ReadFolders_countsQuizzesFromBothOrigins(t *testing.T) {
	// Given a quiz accepted in the chat (with a message id) and one generated from the menu (without)
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "One")
	addSession(t, db, "s1", "Cache", "f1", progressBase)
	addQuiz(t, db, "q-chat", "s1", "message-1")
	addQuiz(t, db, "q-menu", "s1", nil)

	// When reading
	session := sessionByID(t, sessionsOf(t, reader, "f1"), "s1")

	// Then both belong to the topic
	require.Len(t, session.Quizzes, 2)
	ids := []string{session.Quizzes[0].ID, session.Quizzes[1].ID}
	assert.ElementsMatch(t, []string{"q-chat", "q-menu"}, ids)
}

func TestProgressReader_ReadFolders_returnsQuizAttemptsOldestFirst(t *testing.T) {
	// Given a quiz retaken twice, the second attempt inserted first
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "One")
	addSession(t, db, "s1", "Cache", "f1", progressBase)
	addQuiz(t, db, "q1", "s1", nil)
	addQuizAttempt(t, db, "qa-late", "q1", 9, 10, 5)
	addQuizAttempt(t, db, "qa-early", "q1", 6, 10, 1)

	// When reading
	session := sessionByID(t, sessionsOf(t, reader, "f1"), "s1")

	// Then the attempts come oldest first with their counts
	require.Len(t, session.Quizzes, 1)
	attempts := session.Quizzes[0].Attempts
	require.Len(t, attempts, 2)
	assert.Equal(t, applicationprogress.QuizAttemptData{CorrectCount: 6, Total: 10, FinishedAt: progressBase.AddDate(0, 0, 1)}, attempts[0])
	assert.Equal(t, 9, attempts[1].CorrectCount)
	assert.True(t, progressBase.AddDate(0, 0, 5).Equal(attempts[1].FinishedAt))
}

func TestProgressReader_ReadFolders_returnsAQuizWithoutAttemptsWithNone(t *testing.T) {
	// Given a quiz nobody took
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "One")
	addSession(t, db, "s1", "Cache", "f1", progressBase)
	addQuiz(t, db, "q1", "s1", nil)

	// When reading
	session := sessionByID(t, sessionsOf(t, reader, "f1"), "s1")

	// Then the quiz is listed with no attempt
	require.Len(t, session.Quizzes, 1)
	assert.Empty(t, session.Quizzes[0].Attempts)
}

func TestProgressReader_ReadFolders_leavesOutAQuizOfADeletedSession(t *testing.T) {
	// Given a quiz whose session was deleted (quizzes keep a dangling session id)
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "One")
	addSession(t, db, "s1", "Cache", "f1", progressBase)
	addQuiz(t, db, "q-orphan", "gone", nil)
	addQuizAttempt(t, db, "qa1", "q-orphan", 1, 2, 1)

	// When reading
	session := sessionByID(t, sessionsOf(t, reader, "f1"), "s1")

	// Then it belongs to nobody
	assert.Empty(t, session.Quizzes)
}

func TestProgressReader_ReadFolders_countsAChallengeOnceWhetherGeneratedFromOrLinkedToTheSession(t *testing.T) {
	// Given c-source generated from s1, c-link only linked to it, c-both generated from and linked to it
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "One")
	addSession(t, db, "s1", "Cache", "f1", progressBase)
	addSession(t, db, "s2", "Other", "f1", progressBase)
	addChallenge(t, db, "c-source", "s1")
	addChallenge(t, db, "c-link", "s2")
	addChallenge(t, db, "c-both", "s1")
	linkChallenge(t, db, "l1", "s1", "c-link")
	linkChallenge(t, db, "l2", "s1", "c-both")

	// When reading
	sessions := sessionsOf(t, reader, "f1")

	// Then s1 has the three, each once, and s2 only the one it generated
	assert.ElementsMatch(t, []string{"c-source", "c-link", "c-both"}, progressChallengeIDs(sessionByID(t, sessions, "s1").Challenges))
	assert.Equal(t, []string{"c-link"}, progressChallengeIDs(sessionByID(t, sessions, "s2").Challenges))
}

func TestProgressReader_ReadFolders_leavesOutAChallengeWhoseSourceSessionWasDeleted(t *testing.T) {
	// Given a challenge generated from a session that is gone (the source is set to NULL)
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "One")
	addSession(t, db, "s1", "Cache", "f1", progressBase)
	addSession(t, db, "s-gone", "Gone", "f1", progressBase)
	addChallenge(t, db, "c1", "s-gone")
	mustExec(t, db, `DELETE FROM sessions WHERE id = 's-gone'`)

	// When reading
	sessions := sessionsOf(t, reader, "f1")

	// Then no remaining topic holds it
	require.Len(t, sessions, 1)
	assert.Empty(t, sessions[0].Challenges)
}

func TestProgressReader_ReadFolders_returnsEveryAttemptOfAChallengeWithItsEvaluations(t *testing.T) {
	// Given a challenge with a complete attempt, a standalone later one stopped before the defense
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "One")
	addSession(t, db, "s1", "Cache", "f1", progressBase)
	addChallenge(t, db, "c1", "s1")
	addRubric(t, db, "rv1", "c1")
	addLabAttempt(t, db, "a-late", "c1", "rv1", 3, 70, -1)
	addLabAttempt(t, db, "a-early", "c1", "rv1", 1, 85, 40)

	// When reading
	session := sessionByID(t, sessionsOf(t, reader, "f1"), "s1")

	// Then both attempts come oldest first, the complete one with both evaluations and the rubric
	require.Len(t, session.Challenges, 1)
	attempts := session.Challenges[0].Attempts
	require.Len(t, attempts, 2)
	assert.Equal(t, "a-early", attempts[0].Attempt.ID)
	require.NotNil(t, attempts[0].Diagram)
	require.NotNil(t, attempts[0].Understanding)
	require.NotNil(t, attempts[0].Rubric)
	assert.Equal(t, 85, attempts[0].Diagram.Score)
	assert.Equal(t, 40, attempts[0].Understanding.Score)
	assert.Equal(t, "a-late", attempts[1].Attempt.ID)
	assert.NotNil(t, attempts[1].Diagram)
	assert.Nil(t, attempts[1].Understanding)
}

func TestProgressReader_ReadFolders_returnsAChallengeWithoutAttemptsWithNone(t *testing.T) {
	// Given a challenge never attempted
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "One")
	addSession(t, db, "s1", "Cache", "f1", progressBase)
	addChallenge(t, db, "c1", "s1")

	// When reading
	session := sessionByID(t, sessionsOf(t, reader, "f1"), "s1")

	// Then it is listed with no attempt
	require.Len(t, session.Challenges, 1)
	assert.Empty(t, session.Challenges[0].Attempts)
}

func TestProgressReader_ReadFolders_returnsTheConceptsOfTheSession(t *testing.T) {
	// Given two sessions with a concept each
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "One")
	addSession(t, db, "s1", "Cache", "f1", progressBase)
	addSession(t, db, "s2", "Other", "f1", progressBase)
	repo := NewConceptStateRepository(db)
	require.NoError(t, repo.Upsert(context.Background(), domainstudy.ConceptState{
		SessionID: "s1", Name: "ttl", Status: domainstudy.StatusNeedsReview, LastStrategy: domainstudy.StrategyGuidedPractice,
		EvidenceCount: 3, UpdatedAt: progressBase.AddDate(0, 0, 2),
	}))
	require.NoError(t, repo.Upsert(context.Background(), domainstudy.ConceptState{
		SessionID: "s2", Name: "lru", Status: domainstudy.StatusDemonstrated, LastStrategy: domainstudy.StrategyGuidedPractice,
		UpdatedAt: progressBase,
	}))

	// When reading
	sessions := sessionsOf(t, reader, "f1")

	// Then each session holds its own concept with status, evidence and update time
	first := sessionByID(t, sessions, "s1").Concepts
	require.Len(t, first, 1)
	assert.Equal(t, domainstudy.StatusNeedsReview, first[0].Status)
	assert.Equal(t, 3, first[0].EvidenceCount)
	assert.True(t, progressBase.AddDate(0, 0, 2).Equal(first[0].UpdatedAt), "got %s", first[0].UpdatedAt)
	second := sessionByID(t, sessions, "s2").Concepts
	require.Len(t, second, 1)
	assert.Equal(t, domainstudy.StatusDemonstrated, second[0].Status)
}

func TestProgressReader_ReadFolders_failsOnAConceptWithAnUnreadableDate(t *testing.T) {
	// Given a concept whose updated_at is not in the stored layout
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "One")
	addSession(t, db, "s1", "Cache", "f1", progressBase)
	mustExec(t, db, `INSERT INTO session_concepts (session_id, name, status, evidence_count, updated_at) VALUES ('s1', 'ttl', 'INTRODUCED', 0, 'garbage')`)

	// When reading
	_, err := reader.ReadFolders(context.Background())

	// Then it fails instead of inventing a date
	require.Error(t, err)
}

func TestProgressReader_ReadFolders_failsOnAQuizAttemptWithAnUnreadableDate(t *testing.T) {
	// Given a quiz attempt whose finished_at is not in the stored layout
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "One")
	addSession(t, db, "s1", "Cache", "f1", progressBase)
	addQuiz(t, db, "q1", "s1", nil)
	mustExec(t, db, `INSERT INTO quiz_attempts (id, quiz_id, correct_count, total, score, finished_at) VALUES ('qa1', 'q1', 1, 2, 0.5, 'garbage')`)

	// When reading
	_, err := reader.ReadFolders(context.Background())

	// Then it fails instead of inventing a date
	require.Error(t, err)
}

func TestAttachSessionsToFolders_leavesOutASessionWhoseFolderWasNotRead(t *testing.T) {
	folders := []applicationprogress.FolderData{{ID: "f1", Name: "One"}}
	sessions := []*progressSession{
		{folderID: "f1", data: applicationprogress.SessionData{ID: "s1"}},
		{folderID: "created-in-between", data: applicationprogress.SessionData{ID: "s2"}},
	}

	attachSessionsToFolders(folders, sessions)

	require.Len(t, folders[0].Sessions, 1)
	assert.Equal(t, "s1", folders[0].Sessions[0].ID)
}

func TestAttachSessionsToFolders_doesNotPanicWithoutFolders(t *testing.T) {
	sessions := []*progressSession{{folderID: "f1", data: applicationprogress.SessionData{ID: "s1"}}}

	assert.NotPanics(t, func() { attachSessionsToFolders(nil, sessions) })
}

func TestProgressReader_ReadFolders_failsWhenTheDatabaseIsClosed(t *testing.T) {
	// Given a closed database
	reader, db := newProgressFixture(t)
	require.NoError(t, db.Close())

	// When reading
	_, err := reader.ReadFolders(context.Background())

	// Then the error is returned
	require.Error(t, err)
}

func TestProgressReader_ReadTopic_failsForAMissingSessionOrOneInNoFolder(t *testing.T) {
	// Given a session in a folder and a session whose folder was removed
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "One")
	addSession(t, db, "s1", "Cache", "f1", progressBase)
	mustExec(t, db, `PRAGMA foreign_keys = OFF`)
	mustExec(t, db, `INSERT INTO sessions (id, topic, mode, folder_id, started_at) VALUES ('orphan', 'X', 'socratic', 'missing', ?)`, progressBase)

	for _, id := range []string{"nope", "orphan"} {
		// When reading a session that is not a topic
		_, err := reader.ReadTopic(context.Background(), id)

		// Then it is not found
		require.ErrorIs(t, err, applicationprogress.ErrTopicNotFound, id)
	}
}

func TestProgressReader_ReadTopic_returnsTheNamesAndOnlyTheQuizzesOfTheSession(t *testing.T) {
	// Given two sessions with quizzes, one quiz of each origin in the first
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "System Design")
	addSession(t, db, "s1", "Cache", "f1", progressBase)
	addSession(t, db, "s2", "Other", "f1", progressBase)
	mustExec(t, db, `INSERT INTO messages (id, session_id, role, content, created_at) VALUES ('m1', 's1', 'assistant', 'x', CURRENT_TIMESTAMP)`)
	addQuiz(t, db, "q-menu", "s1", nil)
	addQuiz(t, db, "q-chat", "s1", "m1")
	addQuiz(t, db, "q-other", "s2", nil)
	addQuizAttempt(t, db, "a2", "q-menu", 3, 4, 2)
	addQuizAttempt(t, db, "a1", "q-menu", 1, 4, 1)
	addQuizAttempt(t, db, "a3", "q-other", 4, 4, 1)

	// When reading the first topic
	topic, err := reader.ReadTopic(context.Background(), "s1")

	// Then it names the session and folder and has its two quizzes only, attempts oldest first
	require.NoError(t, err)
	assert.Equal(t, "s1", topic.SessionID)
	assert.Equal(t, "Cache", topic.Topic)
	assert.Equal(t, "System Design", topic.FolderName)
	require.Len(t, topic.Quizzes, 2)
	menu := topic.Quizzes[0]
	if menu.ID != "q-menu" {
		menu = topic.Quizzes[1]
	}
	assert.Equal(t, "INTERMEDIATE", menu.Difficulty)
	require.Len(t, menu.Attempts, 2)
	assert.Equal(t, 1, menu.Attempts[0].CorrectCount)
	assert.Equal(t, 3, menu.Attempts[1].CorrectCount)
}

func TestProgressReader_ReadTopic_countsAChallengeOnceWithItsTitleAndAttempts(t *testing.T) {
	// Given a challenge generated from the session and also linked to it, with an evaluated attempt
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "One")
	addSession(t, db, "s1", "Cache", "f1", progressBase)
	addChallenge(t, db, "c1", "s1")
	linkChallenge(t, db, "l1", "s1", "c1")
	mustExec(t, db, `UPDATE challenges SET title = 'Design a cache', difficulty = 'ADVANCED', time_limit_minutes = 60 WHERE id = 'c1'`)
	addRubric(t, db, "rv1", "c1")
	addLabAttempt(t, db, "at1", "c1", "rv1", 1, 70, 60)

	// When reading the topic
	topic, err := reader.ReadTopic(context.Background(), "s1")

	// Then the challenge appears once with its title and both evaluations
	require.NoError(t, err)
	require.Len(t, topic.Challenges, 1)
	assert.Equal(t, "Design a cache", topic.Challenges[0].Title)
	assert.Equal(t, "ADVANCED", topic.Challenges[0].Difficulty)
	assert.Equal(t, 60, topic.Challenges[0].TimeLimitMinutes)
	require.Len(t, topic.Challenges[0].Attempts, 1)
	assert.Equal(t, 70, topic.Challenges[0].Attempts[0].Diagram.Score)
	assert.Equal(t, 60, topic.Challenges[0].Attempts[0].Understanding.Score)
}

func TestProgressReader_ReadTopic_returnsTheConceptsOfTheSessionWithDisplayNameAndProofs(t *testing.T) {
	// Given a concept with a display name and a proof, and a concept of another session
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "One")
	addSession(t, db, "s1", "Cache", "f1", progressBase)
	addSession(t, db, "s2", "Other", "f1", progressBase)
	ctx := context.Background()
	concepts := NewConceptStateRepository(db)
	require.NoError(t, concepts.Upsert(ctx, domainstudy.ConceptState{
		SessionID: "s1", Name: "cache-aside", DisplayName: "Cache-aside", Status: domainstudy.StatusDemonstrated,
		EvidenceCount: 1, UpdatedAt: progressBase,
	}))
	require.NoError(t, concepts.Upsert(ctx, domainstudy.ConceptState{
		SessionID: "s2", Name: "lru", Status: domainstudy.StatusIntroduced, UpdatedAt: progressBase,
	}))
	mustExec(t, db, `INSERT INTO messages (id, session_id, role, content, created_at) VALUES
		('u1', 's1', 'user', 'x', CURRENT_TIMESTAMP), ('r1', 's1', 'assistant', 'y', CURRENT_TIMESTAMP)`)
	require.NoError(t, NewEvidenceRepository(db).Add(ctx, domainstudy.Evidence{
		SessionID: "s1", Concept: "cache-aside", Assessment: domainstudy.AssessmentCorrect, Description: "explained it",
		UserMessageID: "u1", ReplyMessageID: "r1", CreatedAt: progressBase,
	}))

	// When reading the first topic
	topic, err := reader.ReadTopic(ctx, "s1")

	// Then it has its own concept, spelled as reported, with the proof
	require.NoError(t, err)
	require.Len(t, topic.Concepts, 1)
	assert.Equal(t, "cache-aside", topic.Concepts[0].Name)
	assert.Equal(t, "Cache-aside", topic.Concepts[0].DisplayName)
	require.Len(t, topic.Concepts[0].Proofs, 1)
	assert.Equal(t, "explained it", topic.Concepts[0].Proofs[0].Description)
}

func TestProgressReader_ReadTopic_aTopicWithNothingAttachedHasEmptyLists(t *testing.T) {
	// Given a session with no quiz, challenge or concept
	reader, db := newProgressFixture(t)
	addFolder(t, db, "f1", "One")
	addSession(t, db, "s1", "Cache", "f1", progressBase)

	// When reading it
	topic, err := reader.ReadTopic(context.Background(), "s1")

	// Then every list is empty
	require.NoError(t, err)
	assert.Empty(t, topic.Quizzes)
	assert.Empty(t, topic.Challenges)
	assert.Empty(t, topic.Concepts)
}

func TestProgressReader_ReadTopic_failsWhenTheDatabaseIsClosed(t *testing.T) {
	// Given a closed database
	reader, db := newProgressFixture(t)
	require.NoError(t, db.Close())

	// When reading a topic
	_, err := reader.ReadTopic(context.Background(), "s1")

	// Then the error is returned
	require.Error(t, err)
	assert.NotErrorIs(t, err, applicationprogress.ErrTopicNotFound)
}

func TestAppendQuizAttempt_ignoresAnAttemptOfAQuizReadAfterTheList(t *testing.T) {
	quizzes := []applicationprogress.TopicQuizData{{ID: "q1"}}
	position := map[string]int{"q1": 0}

	appendQuizAttempt(quizzes, position, "q-new", applicationprogress.QuizAttemptData{Total: 5})
	appendQuizAttempt(nil, map[string]int{}, "q-new", applicationprogress.QuizAttemptData{Total: 5})
	appendQuizAttempt(quizzes, position, "q1", applicationprogress.QuizAttemptData{Total: 3})

	assert.Equal(t, []applicationprogress.QuizAttemptData{{Total: 3}}, quizzes[0].Attempts)
}
