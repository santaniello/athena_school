package openrouter

import (
	"context"
	"encoding/json"
	"fmt"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

type openRouterModel struct {
	ID            string `json:"id"`
	ContextLength int    `json:"context_length"`
}

type openRouterModelsResponse struct {
	Data []openRouterModel `json:"data"`
}

// ListModels implements domainllm.ModelCatalog against OpenRouter's public
// model listing. It performs no entry validation itself — that is the
// application-level cache's responsibility (see
// internal/application/modelcatalog), since it owns what "valid" means for
// its cache (entry-isolated skip/collapse/exclude rules).
func (c *Client) ListModels(ctx context.Context) ([]domainllm.ModelInfo, error) {
	resp, err := c.get(ctx, "/api/v1/models")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var decoded openRouterModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("openrouter: decoding models response: %w", err)
	}

	models := make([]domainllm.ModelInfo, len(decoded.Data))
	for i, m := range decoded.Data {
		models[i] = domainllm.ModelInfo{ID: m.ID, ContextLength: m.ContextLength}
	}
	return models, nil
}
