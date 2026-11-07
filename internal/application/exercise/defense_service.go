package exercise

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// defenseTimeout bounds one model call of the defense, so a hung provider
// cannot hold the attempt's lock for the provider's own, much longer, timeout.
const defenseTimeout = 2 * time.Minute

// ErrEmptyQuestion is returned when the model writes a question with no text.
var ErrEmptyQuestion = errors.New("the model returned an empty question")

// DefenseView is the defense of an attempt as the screen needs it: how many
// questions it has, the turns so far in order, and whether the last answer
// has arrived.
type DefenseView struct {
	Total    int
	Turns    []domainexercise.DefenseTurn
	Complete bool
}

// DefenseOutcome is what the end of an attempt shows: both evaluations, the
// criteria they refer to, the minimum passing score frozen on the attempt and
// the outcome of the pass policy.
type DefenseOutcome struct {
	Diagram       domainexercise.Evaluation
	Understanding domainexercise.Evaluation
	Criteria      []domainexercise.RubricCriterion
	PassScore     int
	Outcome       domainexercise.AttemptOutcome
}

// DefenseService runs the tutor's questions after the diagram evaluation and
// the final evaluation of the answers. The criterion each question targets is
// chosen here by the domain, the model only writes the text. Calls for one
// attempt are serialized. See
// specs/phases/phase-03-architecture-lab/12-architecture-defense.md.
type DefenseService struct {
	challenges  domainexercise.ChallengeRepository
	rubrics     domainexercise.RubricRepository
	attempts    domainexercise.AttemptRepository
	submissions domainexercise.SubmissionRepository
	evaluations domainexercise.EvaluationRepository
	defense     domainexercise.DefenseRepository
	generator   domainexercise.QuestionGenerator
	evaluator   domainexercise.UnderstandingEvaluator
	tx          Transactor
	clock       Clock
	newID       func() string
	locks       attemptLocks
}

// NewDefenseService creates a DefenseService backed by the given ports.
func NewDefenseService(
	challenges domainexercise.ChallengeRepository,
	rubrics domainexercise.RubricRepository,
	attempts domainexercise.AttemptRepository,
	submissions domainexercise.SubmissionRepository,
	evaluations domainexercise.EvaluationRepository,
	defense domainexercise.DefenseRepository,
	generator domainexercise.QuestionGenerator,
	evaluator domainexercise.UnderstandingEvaluator,
	tx Transactor,
	clock Clock,
) *DefenseService {
	return &DefenseService{
		challenges:  challenges,
		rubrics:     rubrics,
		attempts:    attempts,
		submissions: submissions,
		evaluations: evaluations,
		defense:     defense,
		generator:   generator,
		evaluator:   evaluator,
		tx:          tx,
		clock:       clock,
		newID:       uuid.NewString,
		locks:       attemptLocks{held: map[string]*attemptLock{}},
	}
}

// defenseContext is what every step reads: the attempt, its challenge, frozen
// rubric, diagram evaluation and the turns so far, plus the criteria the
// defense asks about.
type defenseContext struct {
	attempt   domainexercise.ChallengeAttempt
	challenge domainexercise.Challenge
	rubric    domainexercise.RubricVersion
	diagram   domainexercise.Evaluation
	turns     []domainexercise.DefenseTurn
	selected  []string
}

func (c defenseContext) view() DefenseView {
	complete := len(c.turns) == len(c.selected)
	for _, turn := range c.turns {
		complete = complete && turn.Answered()
	}
	return DefenseView{Total: len(c.selected), Turns: c.turns, Complete: complete}
}

// pending returns the last turn when it still has no answer.
func (c defenseContext) pending() (domainexercise.DefenseTurn, bool) {
	if len(c.turns) == 0 || c.turns[len(c.turns)-1].Answered() {
		return domainexercise.DefenseTurn{}, false
	}
	return c.turns[len(c.turns)-1], true
}

func (s *DefenseService) load(ctx context.Context, attempt domainexercise.ChallengeAttempt) (defenseContext, error) {
	turns, err := s.defense.ListByAttempt(ctx, attempt.ID)
	if err != nil {
		return defenseContext{}, fmt.Errorf("exercise: listing defense turns: %w", err)
	}
	return s.loadWithTurns(ctx, attempt, turns)
}

func (s *DefenseService) getAttempt(ctx context.Context, attemptID string) (domainexercise.ChallengeAttempt, error) {
	attempt, err := s.attempts.Get(ctx, attemptID)
	if err != nil {
		return domainexercise.ChallengeAttempt{}, fmt.Errorf("exercise: getting attempt: %w", err)
	}
	return attempt, nil
}

// NextQuestion returns the question the user must answer now. The first call
// moves an EVALUATED attempt to DEFENDING; a question still waiting for its
// answer is returned again without a model call; once every question is
// answered it is domainexercise.ErrDefenseNotActive. A failed generation
// stores nothing and can be repeated.
func (s *DefenseService) NextQuestion(ctx context.Context, attemptID string) (domainexercise.DefenseTurn, error) {
	defer s.locks.lock(attemptID)()

	attempt, err := s.getAttempt(ctx, attemptID)
	if err != nil {
		return domainexercise.DefenseTurn{}, err
	}
	if attempt.State != domainexercise.AttemptEvaluated && attempt.State != domainexercise.AttemptDefending {
		return domainexercise.DefenseTurn{}, domainexercise.ErrDefenseNotActive
	}
	defense, err := s.load(ctx, attempt)
	if err != nil {
		return domainexercise.DefenseTurn{}, err
	}
	if pending, ok := defense.pending(); ok {
		return pending, nil
	}
	if len(defense.turns) >= len(defense.selected) {
		return domainexercise.DefenseTurn{}, domainexercise.ErrDefenseNotActive
	}
	turn, err := s.askNext(ctx, defense)
	if err != nil {
		return domainexercise.DefenseTurn{}, err
	}
	if err := s.storeQuestion(ctx, attempt, turn); err != nil {
		return domainexercise.DefenseTurn{}, err
	}
	return turn, nil
}

// askNext has the model write the question for the next criterion.
func (s *DefenseService) askNext(ctx context.Context, defense defenseContext) (domainexercise.DefenseTurn, error) {
	submission, err := s.submissions.GetByAttempt(ctx, defense.attempt.ID)
	if err != nil {
		return domainexercise.DefenseTurn{}, fmt.Errorf("exercise: getting submission: %w", err)
	}
	criterionID := defense.selected[len(defense.turns)]
	position := len(defense.turns) + 1
	callCtx, cancel := context.WithTimeout(ctx, defenseTimeout)
	defer cancel()
	text, err := s.generator.GenerateQuestion(callCtx, domainexercise.QuestionInput{
		Challenge: defense.challenge, Rubric: defense.rubric, Submission: submission, Diagram: defense.diagram,
		Criterion: criterionByID(defense.rubric, criterionID), Position: position, Total: len(defense.selected),
		Previous: defense.turns,
	})
	if err != nil {
		return domainexercise.DefenseTurn{}, fmt.Errorf("exercise: generating question: %w", err)
	}
	question := strings.TrimSpace(text)
	if question == "" {
		return domainexercise.DefenseTurn{}, ErrEmptyQuestion
	}
	return domainexercise.DefenseTurn{
		ID: s.newID(), AttemptID: defense.attempt.ID, Position: position, CriterionID: criterionID,
		Question: question, AskedAt: s.clock.Now(),
	}, nil
}

func criterionByID(rubric domainexercise.RubricVersion, id string) domainexercise.RubricCriterion {
	for _, criterion := range rubric.Criteria {
		if criterion.ID == id {
			return criterion
		}
	}
	return domainexercise.RubricCriterion{}
}

// storeQuestion stores the turn and, on the first question, the move to
// DEFENDING in one transaction.
func (s *DefenseService) storeQuestion(ctx context.Context, attempt domainexercise.ChallengeAttempt, turn domainexercise.DefenseTurn) error {
	if attempt.State == domainexercise.AttemptDefending {
		if err := s.defense.AddQuestion(ctx, turn); err != nil {
			return fmt.Errorf("exercise: storing question: %w", err)
		}
		return nil
	}
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.attempts.StartDefense(ctx, attempt.ID); err != nil {
			return err
		}
		return s.defense.AddQuestion(ctx, turn)
	})
	if err != nil {
		return fmt.Errorf("exercise: starting defense: %w", err)
	}
	return nil
}

// Answer stores the answer to the question that is waiting and reports the
// defense, complete once the last answer is in. An invalid answer is
// domainexercise.ErrInvalidAnswer; with no question waiting it is
// domainexercise.ErrNoQuestionPending.
func (s *DefenseService) Answer(ctx context.Context, attemptID, answer string) (DefenseView, error) {
	if err := domainexercise.ValidateAnswer(answer); err != nil {
		return DefenseView{}, err
	}
	defer s.locks.lock(attemptID)()

	attempt, err := s.getAttempt(ctx, attemptID)
	if err != nil {
		return DefenseView{}, err
	}
	if attempt.State != domainexercise.AttemptDefending {
		return DefenseView{}, domainexercise.ErrDefenseNotActive
	}
	turns, err := s.defense.ListByAttempt(ctx, attemptID)
	if err != nil {
		return DefenseView{}, fmt.Errorf("exercise: listing defense turns: %w", err)
	}
	if len(turns) == 0 || turns[len(turns)-1].Answered() {
		return DefenseView{}, domainexercise.ErrNoQuestionPending
	}
	last := &turns[len(turns)-1]
	last.Answer, last.AnsweredAt = strings.TrimSpace(answer), s.clock.Now()
	if err := s.defense.SaveAnswer(ctx, last.ID, last.Answer, last.AnsweredAt); err != nil {
		return DefenseView{}, fmt.Errorf("exercise: saving answer: %w", err)
	}
	defense, err := s.loadWithTurns(ctx, attempt, turns)
	if err != nil {
		return DefenseView{}, err
	}
	return defense.view(), nil
}

// loadWithTurns is load for a caller that already holds the turns, and reads
// the rest of what every step needs.
func (s *DefenseService) loadWithTurns(ctx context.Context, attempt domainexercise.ChallengeAttempt, turns []domainexercise.DefenseTurn) (defenseContext, error) {
	challenge, err := s.challenges.Get(ctx, attempt.ChallengeID)
	if err != nil {
		return defenseContext{}, fmt.Errorf("exercise: getting challenge: %w", err)
	}
	rubric, err := s.rubrics.Get(ctx, attempt.RubricVersionID)
	if err != nil {
		return defenseContext{}, fmt.Errorf("exercise: getting rubric: %w", err)
	}
	diagram, err := s.evaluations.GetByAttempt(ctx, attempt.ID, domainexercise.EvaluationKindDiagram)
	if err != nil {
		return defenseContext{}, fmt.Errorf("exercise: getting diagram evaluation: %w", err)
	}
	return defenseContext{
		attempt: attempt, challenge: challenge, rubric: rubric, diagram: diagram, turns: turns,
		selected: domainexercise.SelectDefenseCriteria(rubric, diagram, challenge.DefenseQuestions),
	}, nil
}

// Evaluate judges the answers once every question is answered, stores the
// UNDERSTANDING evaluation and finishes the attempt in one transaction, opened
// only after the model has answered. Any failure leaves the attempt DEFENDING
// so the call can be repeated. A FINISHED attempt returns its stored
// evaluation without calling the model.
func (s *DefenseService) Evaluate(ctx context.Context, attemptID string) (domainexercise.Evaluation, error) {
	defer s.locks.lock(attemptID)()

	attempt, err := s.getAttempt(ctx, attemptID)
	if err != nil {
		return domainexercise.Evaluation{}, err
	}
	if attempt.State == domainexercise.AttemptFinished {
		return s.getUnderstanding(ctx, attemptID)
	}
	if attempt.State != domainexercise.AttemptDefending {
		return domainexercise.Evaluation{}, domainexercise.ErrDefenseNotActive
	}
	defense, err := s.load(ctx, attempt)
	if err != nil {
		return domainexercise.Evaluation{}, err
	}
	if !defense.view().Complete {
		return domainexercise.Evaluation{}, domainexercise.ErrQuestionPending
	}
	submission, err := s.submissions.GetByAttempt(ctx, attemptID)
	if err != nil {
		return domainexercise.Evaluation{}, fmt.Errorf("exercise: getting submission: %w", err)
	}
	callCtx, cancel := context.WithTimeout(ctx, defenseTimeout)
	defer cancel()
	output, err := s.evaluator.EvaluateUnderstanding(callCtx, domainexercise.UnderstandingInput{
		Challenge: defense.challenge, Rubric: defense.rubric, Submission: submission, Diagram: defense.diagram, Turns: defense.turns,
	})
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return domainexercise.Evaluation{}, fmt.Errorf("exercise: evaluating understanding: %w", err)
	}
	evaluation, err := domainexercise.NewUnderstandingEvaluation(attempt, defense.rubric, defense.turns, output, output.Model, s.clock.Now())
	if err != nil {
		return domainexercise.Evaluation{}, err
	}
	evaluation.ID = s.newID()
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.evaluations.Create(ctx, evaluation); err != nil {
			return err
		}
		return s.attempts.FinishDefense(ctx, attemptID)
	})
	if err != nil {
		return domainexercise.Evaluation{}, fmt.Errorf("exercise: finishing defense: %w", err)
	}
	return evaluation, nil
}

func (s *DefenseService) getUnderstanding(ctx context.Context, attemptID string) (domainexercise.Evaluation, error) {
	evaluation, err := s.evaluations.GetByAttempt(ctx, attemptID, domainexercise.EvaluationKindUnderstanding)
	if err != nil {
		return domainexercise.Evaluation{}, fmt.Errorf("exercise: getting understanding evaluation: %w", err)
	}
	return evaluation, nil
}

// GetDefense returns the defense of an attempt whose diagram was evaluated:
// the turns so far and how many questions there are. A FINISHED attempt keeps
// its transcript, read-only.
func (s *DefenseService) GetDefense(ctx context.Context, attemptID string) (DefenseView, error) {
	attempt, err := s.getAttempt(ctx, attemptID)
	if err != nil {
		return DefenseView{}, err
	}
	switch attempt.State {
	case domainexercise.AttemptEvaluated, domainexercise.AttemptDefending, domainexercise.AttemptFinished:
	default:
		return DefenseView{}, domainexercise.ErrDefenseNotActive
	}
	defense, err := s.load(ctx, attempt)
	if err != nil {
		return DefenseView{}, err
	}
	return defense.view(), nil
}

// Outcome returns the end of a FINISHED attempt: both evaluations and the
// outcome of the pass policy at the minimum score frozen on the attempt.
func (s *DefenseService) Outcome(ctx context.Context, attemptID string) (DefenseOutcome, error) {
	attempt, err := s.getAttempt(ctx, attemptID)
	if err != nil {
		return DefenseOutcome{}, err
	}
	if attempt.State != domainexercise.AttemptFinished {
		return DefenseOutcome{}, domainexercise.ErrDefenseNotActive
	}
	rubric, err := s.rubrics.Get(ctx, attempt.RubricVersionID)
	if err != nil {
		return DefenseOutcome{}, fmt.Errorf("exercise: getting rubric: %w", err)
	}
	diagram, err := s.evaluations.GetByAttempt(ctx, attemptID, domainexercise.EvaluationKindDiagram)
	if err != nil {
		return DefenseOutcome{}, fmt.Errorf("exercise: getting diagram evaluation: %w", err)
	}
	understanding, err := s.getUnderstanding(ctx, attemptID)
	if err != nil {
		return DefenseOutcome{}, err
	}
	return DefenseOutcome{
		Diagram: diagram, Understanding: understanding, Criteria: rubric.Criteria, PassScore: attempt.PassScore,
		Outcome: domainexercise.EvaluateOutcome(attempt.PassScore, rubric, &diagram, &understanding),
	}, nil
}

// attemptLocks serializes the calls for one attempt. The desktop app is one
// process, so an in-process lock is enough. An entry exists only while some
// call holds or awaits it.
type attemptLocks struct {
	mu   sync.Mutex
	held map[string]*attemptLock
}

type attemptLock struct {
	mu   sync.Mutex
	refs int
}

// lock blocks until the attempt's lock is free and returns the function that
// releases it.
func (l *attemptLocks) lock(attemptID string) func() {
	l.mu.Lock()
	entry, ok := l.held[attemptID]
	if !ok {
		entry = &attemptLock{}
		l.held[attemptID] = entry
	}
	entry.refs++
	l.mu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		l.mu.Lock()
		defer l.mu.Unlock()
		entry.refs--
		if entry.refs == 0 {
			delete(l.held, attemptID)
		}
	}
}
