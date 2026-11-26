package progress

import (
	"context"
	"errors"
	"fmt"
	"time"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainprogress "github.com/santaniello/athena/internal/domain/progress"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// ErrInsightsUnavailable is returned when the Service was built without the
// ports the insights need.
var ErrInsightsUnavailable = errors.New("topic insights are not available")

type insightPorts struct {
	repo      domainprogress.InsightRepository
	generator domainprogress.InsightGenerator
	language  LanguageProvider
	clock     Clock
}

// WithInsights wires the Athena Insights of a topic (spec 5.4).
func WithInsights(repo domainprogress.InsightRepository, generator domainprogress.InsightGenerator, language LanguageProvider, clock Clock) Option {
	return func(s *Service) {
		s.insights = &insightPorts{repo: repo, generator: generator, language: language, clock: clock}
	}
}

// GenerateTopicInsight asks the model for the insight of a topic from its
// persisted evidence and stores it, replacing the previous one. It is the only
// place an LLM is called for Progress, and only on the user's request. A topic
// without evidence is domainprogress.ErrInsufficientEvidence; a model answer
// that does not validate is domainprogress.ErrInvalidInsight; either way, and
// on any other failure, the stored insight is left as it was. See
// specs/phases/phase-05-progress/04-topic-insights.md.
func (s *Service) GenerateTopicInsight(ctx context.Context, sessionID string) (domainprogress.Insight, error) {
	if s.insights == nil {
		return domainprogress.Insight{}, ErrInsightsUnavailable
	}
	passScore, err := s.passScores.Current(ctx)
	if err != nil {
		return domainprogress.Insight{}, fmt.Errorf("progress: reading the pass score: %w", err)
	}
	topic, err := s.reader.ReadTopic(ctx, sessionID)
	if err != nil {
		return domainprogress.Insight{}, fmt.Errorf("progress: reading topic: %w", err)
	}
	status, err := topicStatus(topic, passScore)
	if err != nil {
		return domainprogress.Insight{}, err
	}
	if status == domainprogress.StatusInsufficientEvidence {
		return domainprogress.Insight{}, domainprogress.ErrInsufficientEvidence
	}
	input, err := insightInput(topic, passScore)
	if err != nil {
		return domainprogress.Insight{}, err
	}
	language, err := s.insights.language.AssistantLanguage(ctx)
	if err != nil {
		return domainprogress.Insight{}, fmt.Errorf("progress: reading the assistant language: %w", err)
	}
	raw, err := s.insights.generator.GenerateInsight(ctx, input, language)
	if err != nil {
		return domainprogress.Insight{}, err
	}
	content, err := domainprogress.ParseInsight(raw.Content, input)
	if err != nil {
		return domainprogress.Insight{}, err
	}
	insight := domainprogress.Insight{
		Analysis: content.Analysis, Recommendations: content.Recommendations,
		Model: raw.Model, Language: language,
		Fingerprint: domainprogress.Fingerprint(input, language), CreatedAt: s.insights.clock.Now(),
	}
	if err := s.insights.repo.Save(ctx, sessionID, insight); err != nil {
		return domainprogress.Insight{}, fmt.Errorf("progress: storing insight: %w", err)
	}
	return insight, nil
}

// insightView reads the stored insight of the topic and compares its
// fingerprint with the one of the evidence now, with no LLM call. On an error
// it still returns the view without the stored insight, so the caller can
// degrade instead of failing. Without the
// insight ports it is the zero view: nothing to show and nothing to generate.
func (s *Service) insightView(ctx context.Context, topic TopicData, passScore int) (domainprogress.InsightView, error) {
	if s.insights == nil {
		return domainprogress.InsightView{}, nil
	}
	status, err := topicStatus(topic, passScore)
	if err != nil {
		return domainprogress.InsightView{}, err
	}
	view := domainprogress.InsightView{CanGenerate: status != domainprogress.StatusInsufficientEvidence}
	stored, err := s.insights.repo.Get(ctx, topic.SessionID)
	if errors.Is(err, domainprogress.ErrInsightNotFound) {
		return view, nil
	}
	if err != nil {
		return view, fmt.Errorf("progress: reading insight: %w", err)
	}
	language, err := s.insights.language.AssistantLanguage(ctx)
	if err != nil {
		return view, fmt.Errorf("progress: reading the assistant language: %w", err)
	}
	input, err := insightInput(topic, passScore)
	if err != nil {
		return view, err
	}
	view.Current = &stored
	view.Stale = stored.Fingerprint != domainprogress.Fingerprint(input, language)
	return view, nil
}

// topicStatus is the status the dashboard gives the topic, which decides
// whether there is evidence to write an insight from.
func topicStatus(topic TopicData, passScore int) (domainprogress.Status, error) {
	session := SessionData{ID: topic.SessionID, Topic: topic.Topic}
	for _, quiz := range topic.Quizzes {
		session.Quizzes = append(session.Quizzes, QuizData{ID: quiz.ID, Attempts: quiz.Attempts})
	}
	for _, challenge := range topic.Challenges {
		session.Challenges = append(session.Challenges, ChallengeData{ID: challenge.ID, Attempts: challenge.Attempts})
	}
	for _, concept := range topic.Concepts {
		session.Concepts = append(session.Concepts, ConceptData{Status: concept.Status, EvidenceCount: concept.EvidenceCount, UpdatedAt: concept.UpdatedAt})
	}
	input, err := topicInput(session, passScore)
	if err != nil {
		return "", err
	}
	return domainprogress.SummarizeTopic(input, passScore).Status, nil
}

// insightInput maps the persisted evidence of the topic: every quiz with its
// attempts, the challenges with the attempts that have an evaluation and a rubric, and the
// concepts the tutor has assessed.
func insightInput(topic TopicData, passScore int) (domainprogress.InsightInput, error) {
	input := domainprogress.InsightInput{SessionID: topic.SessionID, Topic: topic.Topic}
	for i, quiz := range topic.Quizzes {
		input.Quizzes = append(input.Quizzes, domainprogress.InsightQuiz{
			ID: quiz.ID, Number: i + 1, Difficulty: quiz.Difficulty,
			Attempts: insightQuizAttempts(quiz.Attempts),
		})
	}
	for _, challenge := range topic.Challenges {
		item, err := insightChallenge(challenge, passScore)
		if err != nil {
			return domainprogress.InsightInput{}, err
		}
		input.Challenges = append(input.Challenges, item)
	}
	for _, concept := range topic.Concepts {
		if concept.Status == domainstudy.StatusNotAssessed {
			continue
		}
		input.Concepts = append(input.Concepts, insightConcept(concept))
	}
	return input, nil
}

func insightQuizAttempts(attempts []QuizAttemptData) []domainprogress.InsightQuizAttempt {
	scored := quizInput(QuizData{Attempts: attempts}).Attempts
	result := make([]domainprogress.InsightQuizAttempt, len(scored))
	for i, attempt := range scored {
		result[i] = domainprogress.InsightQuizAttempt(attempt)
	}
	return result
}

func insightChallenge(challenge TopicChallengeData, passScore int) (domainprogress.InsightChallenge, error) {
	item := domainprogress.InsightChallenge{ID: challenge.ID, Title: challenge.Title}
	for _, record := range challenge.Attempts {
		if record.Diagram == nil && record.Understanding == nil {
			continue
		}
		if record.Rubric == nil {
			// Without its frozen rubric the criteria have no names and the
			// outcome cannot be decided: the attempt is not evidence.
			continue
		}
		outcome := domainexercise.EvaluateOutcome(passScore, *record.Rubric, record.Diagram, record.Understanding)
		item.Attempts = append(item.Attempts, domainprogress.InsightLabAttempt{
			Result:  domainprogress.LabResult(outcome.Result),
			Diagram: insightEvaluation(record.Diagram, *record.Rubric),
			Defense: insightEvaluation(record.Understanding, *record.Rubric),
			At:      latestEvaluationTime(record),
		})
	}
	return item, nil
}

// latestEvaluationTime is the latest of the attempt's time and the creation of
// its evaluations, so an evaluation that lands later is new evidence.
func latestEvaluationTime(record domainexercise.AttemptRecord) time.Time {
	latest := attemptTime(record.Attempt)
	for _, evaluation := range []*domainexercise.Evaluation{record.Diagram, record.Understanding} {
		if evaluation != nil && evaluation.CreatedAt.After(latest) {
			latest = evaluation.CreatedAt
		}
	}
	return latest
}

// insightEvaluation maps an evaluation with the names of its criteria from the
// attempt's frozen rubric. A result whose criterion is not in the rubric has
// no name to show and is left out.
func insightEvaluation(evaluation *domainexercise.Evaluation, rubric domainexercise.RubricVersion) *domainprogress.InsightEvaluation {
	if evaluation == nil {
		return nil
	}
	names := make(map[string]string, len(rubric.Criteria))
	for _, criterion := range rubric.Criteria {
		names[criterion.ID] = criterion.Name
	}
	mapped := &domainprogress.InsightEvaluation{
		Score: evaluation.Score, Strengths: evaluation.Strengths, Gaps: evaluation.Gaps, Recommendations: evaluation.Recommendations,
	}
	for _, result := range evaluation.Results {
		name, known := names[result.CriterionID]
		if !known {
			continue
		}
		mapped.Criteria = append(mapped.Criteria, domainprogress.InsightCriterion{
			Name: name, Level: result.Level, Justification: result.Justification, InsufficientEvidence: result.InsufficientEvidence,
		})
	}
	return mapped
}

func insightConcept(concept TopicConceptData) domainprogress.InsightConcept {
	name := concept.DisplayName
	if name == "" {
		name = concept.Name
	}
	item := domainprogress.InsightConcept{Name: name, Status: domainprogress.ConceptStatus(concept.Status), UpdatedAt: concept.UpdatedAt}
	for _, proof := range concept.Proofs {
		item.Proofs = append(item.Proofs, domainprogress.InsightProof{
			Assessment: string(proof.Assessment), Description: proof.Description, Assisted: proof.Assisted, At: proof.CreatedAt,
		})
	}
	return item
}
