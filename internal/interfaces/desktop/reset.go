package desktop

// ResetLocalData permanently deletes every study session, user-created
// folder, and knowledge item, then reloads the app so every piece of UI
// state (folder tree, topic tree, review badges) reflects the empty state
// — see specs/phases/phase-01-desktop-mvp/13-reset-local-data.md. The
// OpenRouter key and onboarding profile are untouched, and the app window
// never closes: only a failed reset returns without reloading.
func (a *App) ResetLocalData() error {
	if err := a.reset.ResetLocalData(a.ctx); err != nil {
		return err
	}
	a.reloadApp(a.ctx)
	return nil
}
