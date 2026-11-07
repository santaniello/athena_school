// Package architecture holds the Architecture Lab's implementations of the
// generic exercise ports, so the exercise engine knows nothing about
// diagrams. See specs/phases/phase-03-architecture-lab/10-01-tutor-hints.md.
package architecture

import (
	"context"
	"errors"
	"fmt"
	"strings"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// ErrEmptyHint is returned when the model answers with no text.
var ErrEmptyHint = errors.New("the model returned an empty hint")

// ProfileReader reads the user's profile, for the assistant language. Defined
// here (consumer side); the composition root adapts the profile store to it.
type ProfileReader interface {
	ReadProfile(ctx context.Context) (domainexercise.GenerationProfile, error)
}

// HintGenerator implements domainexercise.HintGenerator with one model call
// per hint, through the provider that records its usage.
type HintGenerator struct {
	llm      domainllm.Provider
	profiles ProfileReader
}

// NewHintGenerator creates a HintGenerator backed by the given ports.
func NewHintGenerator(llm domainllm.Provider, profiles ProfileReader) *HintGenerator {
	return &HintGenerator{llm: llm, profiles: profiles}
}

// GenerateHint asks the model for the next hint, in the user's assistant
// language. It makes exactly one model call and never retries.
func (g *HintGenerator) GenerateHint(ctx context.Context, input domainexercise.HintInput) (string, error) {
	profile, err := g.profiles.ReadProfile(ctx)
	if err != nil {
		return "", fmt.Errorf("architecture: reading profile: %w", err)
	}
	response, err := g.llm.Chat(ctx, domainllm.ChatRequest{
		Task:     hintTask,
		Messages: buildHintPrompt(input, profile.Language),
	})
	if err != nil {
		return "", fmt.Errorf("architecture: generating hint: %w", err)
	}
	hint := strings.TrimSpace(response.Content)
	if hint == "" {
		return "", ErrEmptyHint
	}
	return hint, nil
}
