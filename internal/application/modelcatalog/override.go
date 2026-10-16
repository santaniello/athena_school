package modelcatalog

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// OverrideResolver is a development aid: it reports a fixed context window
// for every model, so a small limit can be reached quickly and the 80%
// warning / 95% block (spec 2.6) can be exercised without a million-token
// conversation. A blank model ID still goes to the wrapped resolver, so an
// unresolved model is never given an invented limit.
type OverrideResolver struct {
	inner  domainllm.ModelContextResolver
	length int
}

// NewOverrideResolver wraps inner and reports length (tokens, positive) for
// every non-blank model ID.
func NewOverrideResolver(inner domainllm.ModelContextResolver, length int) *OverrideResolver {
	return &OverrideResolver{inner: inner, length: length}
}

// CachedContextLength reports the override without any I/O.
func (r *OverrideResolver) CachedContextLength(modelID string) (int, bool) {
	if modelID == "" {
		return r.inner.CachedContextLength(modelID)
	}
	return r.length, true
}

// RefreshContextLength reports the override and never hits the catalog.
func (r *OverrideResolver) RefreshContextLength(ctx context.Context, modelID string) (int, error) {
	if modelID == "" {
		return r.inner.RefreshContextLength(ctx, modelID)
	}
	return r.length, nil
}

// ParseContextWindowOverride reads the raw setting: empty, "0" or "off" mean
// the override is off (0); a positive integer is the window in tokens.
// Anything else is an error so a typo never silently turns it on or off.
func ParseContextWindowOverride(raw string) (int, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" || value == "off" {
		return 0, nil
	}
	length, err := strconv.Atoi(value)
	if err != nil || length < 0 {
		return 0, fmt.Errorf("modelcatalog: invalid context window override %q: want a positive integer or \"off\"", raw)
	}
	return length, nil
}
