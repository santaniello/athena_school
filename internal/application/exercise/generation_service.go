package exercise

import (
	"context"
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
}

// NewGenerationService creates a GenerationService backed by the given ports.
func NewGenerationService(
	sessions SessionTopicReader,
	summaries SummaryReader,
	profiles ProfileReader,
	notes NotesSearcher,
	llm domainllm.Provider,
	challenges *ChallengeService,
	rubrics *RubricService,
	tx Transactor,
) *GenerationService {
	return &GenerationService{
		sessions:   sessions,
		contexts:   generationContextBuilder{summaries: summaries, profiles: profiles, notes: notes},
		llm:        llm,
		challenges: challenges,
		rubrics:    rubrics,
		tx:         tx,
	}
}

// GenerateChallenge asks the model for a challenge about the session's topic
// at the given difficulty and saves it, with its first rubric version, in one
// transaction. It makes exactly one model call and never retries: on any
// failure nothing is stored and the caller may call again. It returns
// ErrSourceSessionNotFound for an unknown session and ErrInvalidGeneration
// when the model's answer breaks a challenge or rubric rule.
func (s *GenerationService) GenerateChallenge(ctx context.Context, sessionID string, difficulty domainexercise.Difficulty) (domainexercise.Challenge, error) {
	if domainexercise.DefaultTimeLimit(difficulty) == 0 {
		return domainexercise.Challenge{}, domainexercise.ErrDifficultyInvalid
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return domainexercise.Challenge{}, ErrSourceSessionNotFound
	}
	session, err := s.sessions.ReadSession(ctx, sessionID)
	if err != nil {
		return domainexercise.Challenge{}, fmt.Errorf("exercise: reading source session: %w", err)
	}
	gc, err := s.contexts.build(ctx, sessionID, session)
	if err != nil {
		return domainexercise.Challenge{}, err
	}

	response, err := s.llm.Chat(ctx, domainllm.ChatRequest{
		SessionID: sessionID,
		Task:      generationTask,
		Messages:  buildGenerationPrompt(gc, difficulty),
	})
	if err != nil {
		return domainexercise.Challenge{}, fmt.Errorf("exercise: generating challenge: %w", err)
	}
	generated, err := parseGeneration(response.Content, session.Topic, difficulty)
	if err != nil {
		return domainexercise.Challenge{}, err
	}
	generated.Challenge.SourceSessionID = sessionID
	return s.save(ctx, generated)
}

// save stores the challenge and its rubric together or not at all: both
// repositories join the transaction carried by the context.
func (s *GenerationService) save(ctx context.Context, generated generatedChallenge) (domainexercise.Challenge, error) {
	var challenge domainexercise.Challenge
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if challenge, err = s.challenges.Create(ctx, generated.Challenge); err != nil {
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
