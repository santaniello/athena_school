package progress

import (
	"context"
	"fmt"
	"log"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainprogress "github.com/santaniello/athena/internal/domain/progress"
)

// GetTopicDetail derives the detail of one topic on read: its quizzes and
// challenges with their latest results, and what the learning evidence shows.
// The pass score is read first, as in GetDashboard. No LLM is called. See
// specs/phases/phase-05-progress/02-topic-detail.md.
func (s *Service) GetTopicDetail(ctx context.Context, sessionID string) (domainprogress.TopicDetail, error) {
	passScore, err := s.passScores.Current(ctx)
	if err != nil {
		return domainprogress.TopicDetail{}, fmt.Errorf("progress: reading the pass score: %w", err)
	}
	topic, err := s.reader.ReadTopic(ctx, sessionID)
	if err != nil {
		return domainprogress.TopicDetail{}, fmt.Errorf("progress: reading topic: %w", err)
	}
	input, err := topicDetailInput(topic, passScore)
	if err != nil {
		return domainprogress.TopicDetail{}, err
	}
	detail := domainprogress.BuildTopicDetail(input)
	evolution, err := evolutionInput(topic, passScore)
	if err != nil {
		return domainprogress.TopicDetail{}, err
	}
	detail.Evolution = domainprogress.BuildEvolution(evolution, passScore)
	// The insight is an extra: when it cannot be read the rest of the detail
	// still shows, with the insight as if none were stored.
	detail.Insight, err = s.insightView(ctx, topic, passScore)
	if err != nil {
		log.Printf("progress: insight of topic %s unavailable: %v", topic.SessionID, err)
	}
	return detail, nil
}

// topicDetailInput maps the topic's evidence for BuildTopicDetail.
func topicDetailInput(topic TopicData, passScore int) (domainprogress.TopicDetailInput, error) {
	input := domainprogress.TopicDetailInput{SessionID: topic.SessionID, Name: topic.Topic, FolderName: topic.FolderName}
	for _, quiz := range topic.Quizzes {
		input.Quizzes = append(input.Quizzes, domainprogress.DetailQuiz{
			QuizID: quiz.ID, Difficulty: quiz.Difficulty, Attempts: quizInput(QuizData{ID: quiz.ID, Attempts: quiz.Attempts}).Attempts,
		})
	}
	for _, challenge := range topic.Challenges {
		detail, err := detailChallenge(challenge, passScore)
		if err != nil {
			return domainprogress.TopicDetailInput{}, err
		}
		input.Challenges = append(input.Challenges, detail)
	}
	for _, concept := range topic.Concepts {
		input.Concepts = append(input.Concepts, detailConcept(concept))
	}
	return input, nil
}

// evolutionInput maps every attempt of the topic's quizzes and challenges, with
// the challenge titles the chart tooltip names.
func evolutionInput(topic TopicData, passScore int) (domainprogress.TopicInput, error) {
	input := domainprogress.TopicInput{SessionID: topic.SessionID, Name: topic.Topic}
	for _, quiz := range topic.Quizzes {
		input.Quizzes = append(input.Quizzes, quizInput(QuizData{ID: quiz.ID, Attempts: quiz.Attempts}))
	}
	for _, challenge := range topic.Challenges {
		item, err := challengeInput(ChallengeData{ID: challenge.ID, Attempts: challenge.Attempts}, passScore)
		if err != nil {
			return domainprogress.TopicInput{}, err
		}
		item.Title = challenge.Title
		input.Challenges = append(input.Challenges, item)
	}
	return input, nil
}

func detailConcept(concept TopicConceptData) domainprogress.DetailConcept {
	detail := domainprogress.DetailConcept{
		Name: concept.Name, DisplayName: concept.DisplayName, Status: domainprogress.ConceptStatus(concept.Status),
		EvidenceCount: concept.EvidenceCount, UpdatedAt: concept.UpdatedAt,
	}
	for _, proof := range concept.Proofs {
		detail.Proofs = append(detail.Proofs, domainprogress.DetailProof{
			Assessment: string(proof.Assessment), Description: proof.Description, Assisted: proof.Assisted, At: proof.CreatedAt,
		})
	}
	return detail
}

func detailChallenge(challenge TopicChallengeData, passScore int) (domainprogress.DetailChallenge, error) {
	detail := domainprogress.DetailChallenge{
		ChallengeID: challenge.ID, Title: challenge.Title, Difficulty: challenge.Difficulty, TimeLimitMinutes: challenge.TimeLimitMinutes,
	}
	for _, record := range challenge.Attempts {
		attempt, err := detailAttempt(record, passScore)
		if err != nil {
			return domainprogress.DetailChallenge{}, err
		}
		detail.Attempts = append(detail.Attempts, attempt)
	}
	return detail, nil
}

// detailAttempt maps an attempt. An attempt without evaluations has no scores
// and a PENDING result; one with evaluations needs its frozen rubric, for the
// outcome and for the titles of the weak criteria.
func detailAttempt(record domainexercise.AttemptRecord, passScore int) (domainprogress.DetailLabAttempt, error) {
	attempt := domainprogress.DetailLabAttempt{Result: domainprogress.LabResultPending, At: attemptTime(record.Attempt)}
	if record.Diagram == nil && record.Understanding == nil {
		return attempt, nil
	}
	if record.Rubric == nil {
		return attempt, fmt.Errorf("progress: attempt %s has evaluations but no rubric", record.Attempt.ID)
	}
	if record.Diagram != nil {
		attempt.DiagramScore = &record.Diagram.Score
	}
	if record.Understanding != nil {
		attempt.DefenseScore = &record.Understanding.Score
	}
	outcome := domainexercise.EvaluateOutcome(passScore, *record.Rubric, record.Diagram, record.Understanding)
	attempt.Result = domainprogress.LabResult(outcome.Result)
	attempt.WeakCriteria = domainexercise.WeakCriteria(*record.Rubric, record.Diagram, record.Understanding)
	return attempt, nil
}
