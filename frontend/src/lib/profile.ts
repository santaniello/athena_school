import { SaveProfile } from '../../wailsjs/go/desktop/App'
import { desktop } from '../../wailsjs/go/models'

// Plain-string mirror of UserProfileInput, used to hold the onboarding draft
// while it is being filled in and edited (no CreatedAt: that is stamped by
// the backend on save).
export interface ProfileDraft {
  name: string
  assistantName: string
  area: string
  specialty: string
  experienceLevel: string
  goals: string[]
  studyStyle: string
}

// Error mapping happens at the call site (see lib/onboardingErrors.ts), same
// as authErrorMessage.
export async function saveUserProfile(draft: ProfileDraft): Promise<void> {
  await SaveProfile(desktop.UserProfileInput.createFrom(draft))
}
