import { SaveOpenRouterKey } from '../../wailsjs/go/desktop/App'

// Thin wrapper around the SaveOpenRouterKey binding, shared by the
// onboarding key gate and (later) the settings screen — see
// specs/phases/phase-01-desktop-mvp/08-settings.md. Error mapping happens
// at the call site (see lib/onboardingErrors.ts), same as authErrorMessage.
export async function saveOpenRouterKey(key: string): Promise<void> {
  await SaveOpenRouterKey(key)
}
