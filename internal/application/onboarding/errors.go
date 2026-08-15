package onboarding

import "errors"

// ErrOpenRouterKeyRequired is returned by SaveOpenRouterKey when the given
// key is blank.
var ErrOpenRouterKeyRequired = errors.New("openrouter key is required")
