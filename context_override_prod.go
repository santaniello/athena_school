//go:build production

package main

// contextWindowOverrideEnv mirrors the development constant so shared code
// compiles in both builds; production never reads it.
const contextWindowOverrideEnv = "ATHENA_CONTEXT_WINDOW"

// contextWindowOverrideSetting is always empty in production builds, so the
// real model window from the catalog is the only limit that applies.
func contextWindowOverrideSetting() string {
	return ""
}
