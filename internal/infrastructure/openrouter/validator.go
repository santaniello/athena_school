// Package openrouter provides a lightweight adapter that confirms an
// OpenRouter API key is valid. It intentionally does not implement the full
// LLMProvider from specs/phases/phase-01-desktop-mvp/05-llm-service.md
// (Chat/ChatStream/Embeddings) — onboarding and settings only need to know
// whether a key is accepted, not to run completions.
package openrouter

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/santaniello/athena/internal/domain/config"
)

const defaultBaseURL = "https://openrouter.ai"

// Validator implements config.KeyValidator against the OpenRouter API.
type Validator struct {
	baseURL    string
	httpClient *http.Client
}

// NewValidator creates a Validator against baseURL. An empty baseURL
// defaults to the real OpenRouter API.
func NewValidator(baseURL string) *Validator {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Validator{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// ValidateKey confirms key is accepted by OpenRouter by calling its
// lightweight key-info endpoint. Only the response status matters: this
// adapter does not decode or depend on the response body's shape.
func (v *Validator) ValidateKey(ctx context.Context, key string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.baseURL+"/api/v1/key", nil)
	if err != nil {
		return fmt.Errorf("openrouter: building key validation request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("openrouter: calling key validation endpoint: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return config.ErrKeyInvalid
	default:
		return fmt.Errorf("openrouter: key validation endpoint returned status %d", resp.StatusCode)
	}
}
