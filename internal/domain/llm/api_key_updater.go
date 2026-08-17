package llm

// APIKeyUpdater is implemented by Provider adapters that can rotate their
// API key at runtime. Settings calls this immediately after persisting a
// new OpenRouter key to config.Store, so already-running study sessions
// pick it up without an app restart. See
// specs/phases/phase-01-desktop-mvp/08-settings.md.
type APIKeyUpdater interface {
	SetAPIKey(key string)
}
