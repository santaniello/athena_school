package exercise

import (
	"context"
	"errors"
	"fmt"
	"strings"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// GenerationService writes a system-design challenge and its rubric from a
// studied session with one model call. See
// specs/phases/phase-03-architecture-lab/06-01-challenge-generation.md.
type GenerationService struct {
	sessions   SessionTopicReader
	contexts   generationContextBuilder
	llm        domainllm.Provider
	challenges *ChallengeService
	rubrics    *RubricService
	tx         Transactor
	starters   StarterDiagramRenderer
}

// NewGenerationService creates a GenerationService backed by the given ports.
func NewGenerationService(
	sessions SessionTopicReader,
	summaries SummaryReader,
	profiles ProfileReader,
	notes NotesSearcher,
	titles ExistingTitleLister,
	llm domainllm.Provider,
	starters StarterDiagramRenderer,
	challenges *ChallengeService,
	rubrics *RubricService,
	tx Transactor,
) *GenerationService {
	return &GenerationService{
		sessions:   sessions,
		contexts:   generationContextBuilder{summaries: summaries, profiles: profiles, notes: notes, titles: titles},
		llm:        llm,
		challenges: challenges,
		rubrics:    rubrics,
		tx:         tx,
		starters:   starters,
	}
}

// GenerateChallenge asks the model for a practice challenge about the
// session's topic, of the difficulty and goal in spec, and saves it, with its first rubric version, in one
// transaction. It makes exactly one model call and never retries: on any
// failure nothing is stored and the caller may call again. It returns
// ErrSourceSessionNotFound for an unknown session, ErrArchLabNotEnabled (before
// any other read, model call or write) when the session's folder has the
// Architecture Lab off at this moment, and ErrInvalidGeneration
// when the model's answer breaks a challenge or rubric rule. An invalid spec
// is refused before anything else, and so is an interview
// (ErrInterviewNotGeneratedFromSession): those come from GenerateCanonical.
func (s *GenerationService) GenerateChallenge(ctx context.Context, sessionID string, spec domainexercise.ChallengeSpec) (domainexercise.Challenge, error) {
	spec = spec.Normalized()
	if err := spec.Validate(); err != nil {
		return domainexercise.Challenge{}, err
	}
	if spec.Kind == domainexercise.ChallengeKindInterview {
		return domainexercise.Challenge{}, ErrInterviewNotGeneratedFromSession
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return domainexercise.Challenge{}, ErrSourceSessionNotFound
	}
	session, err := s.sessions.ReadSession(ctx, sessionID)
	if err != nil {
		return domainexercise.Challenge{}, fmt.Errorf("exercise: reading source session: %w", err)
	}
	if !session.ArchLabEnabled {
		return domainexercise.Challenge{}, domainexercise.ErrArchLabNotEnabled
	}
	gc, err := s.contexts.build(ctx, sessionID, session, spec.Kind)
	if err != nil {
		return domainexercise.Challenge{}, err
	}

	response, err := s.llm.Chat(ctx, domainllm.ChatRequest{
		SessionID: sessionID,
		Task:      generationTask,
		Messages:  buildGenerationPrompt(gc, spec),
	})
	if err != nil {
		return domainexercise.Challenge{}, fmt.Errorf("exercise: generating challenge: %w", err)
	}
	generated, err := parseGeneration(response.Content, session.Topic, spec, s.starters)
	if err != nil {
		return domainexercise.Challenge{}, err
	}
	generated.Challenge.SourceSessionID = sessionID
	return s.save(ctx, generated, s.challenges.Create)
}

// save stores the challenge (made by create: from a session or a canonical problem) and its rubric together or not at all: both
// repositories join the transaction carried by the context.
func (s *GenerationService) save(ctx context.Context, generated generatedChallenge, create func(context.Context, CreateChallengeInput) (domainexercise.Challenge, error)) (domainexercise.Challenge, error) {
	var challenge domainexercise.Challenge
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if challenge, err = create(ctx, generated.Challenge); err != nil {
			return err
		}
		_, err = s.rubrics.SetChallengeRubric(ctx, challenge.ID, generated.Criteria)
		return err
	})
	if err != nil {
		return domainexercise.Challenge{}, fmt.Errorf("exercise: saving generated challenge: %w", err)
	}
	return challenge, nil
}

// GenerateCanonical returns the interview challenge of a canonical problem at a
// difficulty. When one is already stored it is returned with no model call;
// otherwise the model writes its statement and rubric from the problem's core
// and the difficulty's scale (one call, no transaction open), and the
// challenge is saved with its first rubric version in one transaction. The
// title, topic and canonical id come from code, never from the model, and the
// challenge has no source session. It returns
// domainexercise.ErrCanonicalProblemNotFound for an unknown problem and
// domainexercise.ErrDifficultyInvalid for an unknown difficulty, both before
// any read. On failure nothing is stored. When two generations race, the
// unique index refuses the second and it returns the stored challenge. See
// specs/phases/phase-07-improve-arch-lab/03-canonical-interviews.md.
func (s *GenerationService) GenerateCanonical(ctx context.Context, problemID string, difficulty domainexercise.Difficulty) (domainexercise.Challenge, error) {
	problem, err := domainexercise.FindCanonicalProblem(problemID)
	if err != nil {
		return domainexercise.Challenge{}, err
	}
	if domainexercise.DefaultTimeLimit(difficulty) == 0 {
		return domainexercise.Challenge{}, domainexercise.ErrDifficultyInvalid
	}
	existing, err := s.challenges.FindCanonical(ctx, problem.ID, difficulty)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, domainexercise.ErrChallengeNotFound) {
		return domainexercise.Challenge{}, err
	}
	profile, err := s.contexts.profiles.ReadProfile(ctx)
	if err != nil {
		return domainexercise.Challenge{}, fmt.Errorf("exercise: reading profile: %w", err)
	}
	response, err := s.llm.Chat(ctx, domainllm.ChatRequest{
		Task:     generationTask,
		Messages: buildCanonicalPrompt(problem, difficulty, profile),
	})
	if err != nil {
		return domainexercise.Challenge{}, fmt.Errorf("exercise: generating canonical challenge: %w", err)
	}
	spec := domainexercise.ChallengeSpec{Difficulty: difficulty, Kind: domainexercise.ChallengeKindInterview}
	title := problem.Name.For(profile.Language)
	generated, err := parseGenerationWithTitle(response.Content, problem.Name.EN, title, spec, nil)
	if err != nil {
		return domainexercise.Challenge{}, err
	}
	generated.Challenge.Topic = problem.Name.EN
	generated.Challenge.CanonicalProblemID = problem.ID
	challenge, err := s.save(ctx, generated, s.challenges.CreateCanonical)
	if errors.Is(err, domainexercise.ErrCanonicalChallengeExists) {
		return s.challenges.FindCanonical(ctx, problem.ID, difficulty)
	}
	return challenge, err
}
