package desktop

// LoginResult is the Wails-serializable outcome of a successful Login call.
// It never carries the password hash.
type LoginResult struct {
	AccountID string `json:"accountId"`
	Email     string `json:"email"`
}

// Register creates a local account. See specs/phases/phase-01-desktop-mvp/02-auth-ui.md.
func (a *App) Register(email, password string) error {
	return a.auth.Register(a.ctx, email, password)
}

// Login validates local credentials and, on success, saves a local session.
func (a *App) Login(email, password string) (LoginResult, error) {
	account, err := a.auth.Login(a.ctx, email, password)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{AccountID: account.ID, Email: account.Email}, nil
}

// ResetLocalAccount deletes the local account so it can be recreated.
func (a *App) ResetLocalAccount(email string) error {
	return a.auth.ResetLocalAccount(a.ctx, email)
}

// HasLocalSession reports whether a local session marker already exists,
// so the frontend can skip the login screen on subsequent launches.
func (a *App) HasLocalSession() bool {
	_, err := a.sessions.Load()
	return err == nil
}
