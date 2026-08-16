import { GetProfile, SaveProfile } from '../../wailsjs/go/desktop/App'
import { desktop } from '../../wailsjs/go/models'

// Plain-string mirror of UserProfileInput, used to hold the onboarding draft
// while it is being filled in and edited (no CreatedAt: that is stamped by
// the backend on save).
export interface ProfileDraft {
  name: string
  assistantName: string
  area: string
  experienceLevel: string
  goals: string[]
  studyStyle: string
  assistantLanguage: string
}

// Error mapping happens at the call site (see lib/onboardingErrors.ts), same
// as authErrorMessage.
export async function saveUserProfile(draft: ProfileDraft): Promise<void> {
  await SaveProfile(desktop.UserProfileInput.createFrom(draft))
}

// Reads the saved profile back, for the home screen greeting (see
// specs/phases/phase-01-desktop-mvp/03-home-screen.md).
export async function getUserProfile(): Promise<ProfileDraft> {
  const profile = await GetProfile()
  return {
    name: profile.name,
    assistantName: profile.assistantName,
    area: profile.area,
    experienceLevel: profile.experienceLevel,
    goals: profile.goals,
    studyStyle: profile.studyStyle,
    assistantLanguage: profile.assistantLanguage,
  }
}
