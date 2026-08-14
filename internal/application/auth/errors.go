package auth

import "errors"

// ErrInvalidCredentials is returned by Login when the email/password
// combination does not match a local account. It intentionally does not
// distinguish an unknown email from a wrong password, to avoid leaking
// account existence.
var ErrInvalidCredentials = errors.New("invalid credentials")
