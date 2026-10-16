//go:build !production

package main

import "os"

// contextWindowOverrideEnv is the environment variable that turns the
// development context-window override on. It is only read in non-production
// builds (`wails dev`, `go run`); `wails build` sets the production tag and
// uses context_override_prod.go, which ignores it.
const contextWindowOverrideEnv = "ATHENA_CONTEXT_WINDOW"

func contextWindowOverrideSetting() string {
	return os.Getenv(contextWindowOverrideEnv)
}
