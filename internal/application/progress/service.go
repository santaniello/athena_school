package progress

import (
	"context"
	"fmt"
	"time"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainprogress "github.com/santaniello/athena/internal/domain/progress"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Service builds the Progress dashboard.
type Service struct {
	reader     Reader
	passScores PassScoreProvider
}

// NewService creates a Service backed by the given ports.
func NewService(reader Reader, passScores PassScoreProvider) *Service {
	return &Service{reader: reader, passScores: passScores}
}

// GetDashboard derives the dashboard on read from the stored data. The pass
// score is read first: one that cannot be verified is an error, never a guess.
func (s *Service) GetDashboard(ctx context.Context) (domainprogress.Dashboard, error) {
	passScore, err := s.passScores.Current(ctx)
	if err != nil {
		return domainprogress.Dashboard{}, fmt.Errorf("progress: reading the pass score: %w", err)
	}
	folders, err := s.reader.ReadFolders(ctx)
	if err != nil {
		return domainprogress.Dashboard{}, fmt.Errorf("progress: reading folders: %w", err)
	}
	inputs := make([]domainprogress.FolderInput, len(folders))
	for i, folder := range folders {
		input, err := folderInput(folder, passScore)
		if err != nil {
			return domainprogress.Dashboard{}, err
		}
		inputs[i] = input
	}
	return domainprogress.BuildDashboard(inputs, passScore), nil
}

func folderInput(folder FolderData, passScore int) (domainprogress.FolderInput, error) {
	input := domainprogress.FolderInput{ID: folder.ID, Name: folder.Name, Topics: make([]domainprogress.TopicInput, len(folder.Sessions))}
	for i, session := range folder.Sessions {
		topic, err := topicInput(session, passScore)
		if err != nil {
			return domainprogress.FolderInput{}, err
		}
		input.Topics[i] = topic
	}
	return input, nil
}

func topicInput(session SessionData, passScore int) (domainprogress.TopicInput, error) {
	topic := domainprogress.TopicInput{SessionID: session.ID, Name: session.Topic, StartedAt: session.StartedAt}
	for _, quiz := range session.Quizzes {
		topic.Quizzes = append(topic.Quizzes, quizInput(quiz))
	}
	for _, challenge := range session.Challenges {
		input, err := challengeInput(challenge, passScore)
		if err != nil {
			return domainprogress.TopicInput{}, err
		}
		topic.Challenges = append(topic.Challenges, input)
	}
	for _, concept := range session.Concepts {
		topic.Concepts = append(topic.Concepts, domainprogress.ConceptInput{
			NeedsReview:   concept.Status == domainstudy.StatusNeedsReview,
			EvidenceCount: concept.EvidenceCount,
			UpdatedAt:     concept.UpdatedAt,
		})
	}
	return topic, nil
}

func quizInput(quiz QuizData) domainprogress.QuizInput {
	input := domainprogress.QuizInput{QuizID: quiz.ID}
	for _, attempt := range quiz.Attempts {
		if attempt.Total <= 0 {
			continue
		}
		input.Attempts = append(input.Attempts, domainprogress.ScoredAttempt{
			Percent: percentOf(attempt.CorrectCount, attempt.Total),
			At:      attempt.FinishedAt,
		})
	}
	return input
}

// percentOf is correct/total in percent, rounded half up.
func percentOf(correct, total int) int {
	return (200*correct + total) / (2 * total)
}

// challengeInput keeps the attempts that have both evaluations; the outcome of
// each is computed with the pass score in force, by the Arch Lab's own rule.
func challengeInput(challenge ChallengeData, passScore int) (domainprogress.ChallengeInput, error) {
	input := domainprogress.ChallengeInput{ChallengeID: challenge.ID}
	for _, record := range challenge.Attempts {
		if record.Diagram == nil || record.Understanding == nil {
			continue
		}
		if record.Rubric == nil {
			return domainprogress.ChallengeInput{}, fmt.Errorf("progress: attempt %s has both evaluations but no rubric", record.Attempt.ID)
		}
		outcome := domainexercise.EvaluateOutcome(passScore, *record.Rubric, record.Diagram, record.Understanding)
		input.Attempts = append(input.Attempts, domainprogress.LabAttempt{
			DiagramScore:       record.Diagram.Score,
			UnderstandingScore: record.Understanding.Score,
			Failed:             outcome.Result == domainexercise.ResultFailed,
			At:                 attemptTime(record.Attempt),
		})
	}
	return input, nil
}

// attemptTime is when the attempt was submitted, or when it started if the
// submission time is not set.
func attemptTime(attempt domainexercise.ChallengeAttempt) time.Time {
	if attempt.SubmittedAt.IsZero() {
		return attempt.StartedAt
	}
	return attempt.SubmittedAt
}
