package quiz

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/google/uuid"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainquiz "github.com/santaniello/athena/internal/domain/quiz"
)

// GenerationService writes a quiz from a studied session with one model call.
// See specs/phases/phase-03-architecture-lab/21-interactive-quiz-core.md.
type GenerationService struct {
	sessions SessionReader
	contexts generationContextBuilder
	llm      domainllm.Provider
	repo     domainquiz.Repository
	now      func() time.Time
	newID    func() string
	shuffle  func(n int) []int // a permutation of 0..n-1
}

// NewGenerationService creates a GenerationService backed by the given ports.
func NewGenerationService(
	sessions SessionReader,
	summaries SummaryReader,
	profiles ProfileReader,
	notes NotesSearcher,
	llm domainllm.Provider,
	repo domainquiz.Repository,
) *GenerationService {
	return &GenerationService{
		sessions: sessions,
		contexts: generationContextBuilder{summaries: summaries, profiles: profiles, notes: notes},
		llm:      llm,
		repo:     repo,
		now:      func() time.Time { return time.Now().UTC() },
		newID:    uuid.NewString,
		shuffle:  rand.Perm,
	}
}

// Generate asks the model for a quiz about the session's topic, of the size and
// difficulty in spec, shuffles the options of its multiple-choice questions and
// saves it. It makes exactly one model call and never retries: on any failure
// nothing is stored and the caller may call again. It returns
// ErrSourceSessionNotFound for an unknown session and ErrInvalidGeneration when
// the model's answer breaks a quiz rule. An invalid spec is refused before
// anything else.
func (s *GenerationService) Generate(ctx context.Context, sessionID string, spec domainquiz.Spec) (domainquiz.Quiz, error) {
	return s.generate(ctx, sessionID, "", spec)
}

// GenerateForMessage is Generate for a quiz an offer in the study chat asked
// for: the stored quiz carries the id of the message the offer hangs under.
func (s *GenerationService) GenerateForMessage(ctx context.Context, sessionID, messageID string, spec domainquiz.Spec) (domainquiz.Quiz, error) {
	return s.generate(ctx, sessionID, messageID, spec)
}

func (s *GenerationService) generate(ctx context.Context, sessionID, messageID string, spec domainquiz.Spec) (domainquiz.Quiz, error) {
	spec = spec.Normalized()
	if err := spec.Validate(); err != nil {
		return domainquiz.Quiz{}, err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return domainquiz.Quiz{}, ErrSourceSessionNotFound
	}
	session, err := s.sessions.ReadSession(ctx, sessionID)
	if err != nil {
		return domainquiz.Quiz{}, fmt.Errorf("quiz: reading source session: %w", err)
	}
	gc, err := s.contexts.build(ctx, sessionID, session)
	if err != nil {
		return domainquiz.Quiz{}, err
	}

	response, err := s.llm.Chat(ctx, domainllm.ChatRequest{
		SessionID: sessionID,
		Task:      generationTask,
		Messages:  buildPrompt(gc, spec),
	})
	if err != nil {
		return domainquiz.Quiz{}, fmt.Errorf("quiz: generating quiz: %w", err)
	}
	generated, err := parseQuestions(response.Content, spec.Count)
	if err != nil {
		return domainquiz.Quiz{}, err
	}
	quiz, err := s.assemble(sessionID, session.Topic, spec, generated)
	if err != nil {
		return domainquiz.Quiz{}, err
	}
	quiz.MessageID = messageID
	if err := s.repo.CreateQuiz(ctx, quiz); err != nil {
		return domainquiz.Quiz{}, fmt.Errorf("quiz: saving generated quiz: %w", err)
	}
	return quiz, nil
}

// assemble gives the quiz and its questions their IDs and positions, shuffles
// the multiple-choice options and validates the whole with the domain rules.
func (s *GenerationService) assemble(sessionID, topic string, spec domainquiz.Spec, generated []domainquiz.Question) (domainquiz.Quiz, error) {
	quiz := domainquiz.Quiz{
		ID:           s.newID(),
		SessionID:    sessionID,
		SessionTitle: strings.TrimSpace(topic),
		Difficulty:   spec.Difficulty,
		CreatedAt:    s.now(),
		Questions:    make([]domainquiz.Question, len(generated)),
	}
	for i, question := range generated {
		shuffled, err := question.Shuffled(s.shuffle(len(question.Options)))
		if err != nil {
			return domainquiz.Quiz{}, fmt.Errorf("quiz: shuffling question %d: %w", i+1, err)
		}
		shuffled.ID = s.newID()
		shuffled.Position = i + 1
		quiz.Questions[i] = shuffled
	}
	if err := quiz.Validate(); err != nil {
		return domainquiz.Quiz{}, fmt.Errorf("%w: %w", ErrInvalidGeneration, err)
	}
	return quiz, nil
}
