import { HasOpenRouterKey, SaveOpenRouterKey } from '../../wailsjs/go/desktop/App'

// Thin wrapper around the SaveOpenRouterKey binding, shared by the
// onboarding key gate and (later) the settings screen — see
// specs/phases/phase-01-desktop-mvp/08-settings.md. Error mapping happens
// at the call site (see lib/onboardingErrors.ts), same as authErrorMessage.
export async function saveOpenRouterKey(key: string): Promise<void> {
  await SaveOpenRouterKey(key)
}

// Reports whether a key is already configured, without ever exposing the
// secret itself to the frontend — used by the settings screen so the user
// isn't left wondering why the (intentionally always-blank) masked field
// looks empty. See specs/phases/phase-01-desktop-mvp/08-settings.md.
export async function hasOpenRouterKey(): Promise<boolean> {
  return HasOpenRouterKey()
}
