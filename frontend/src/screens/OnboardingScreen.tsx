import { useState } from 'react'
import OnboardingFormScreen from './OnboardingFormScreen'
import OnboardingConfirmScreen from './OnboardingConfirmScreen'
import type { ProfileDraft } from '@/lib/profile'

interface OnboardingScreenProps {
  onComplete: () => void
}

type Step = 'form' | 'confirm'

function emptyDraft(): ProfileDraft {
  return {
    name: '',
    assistantName: '',
    area: '',
    experienceLevel: '',
    goals: [],
    studyStyle: '',
  }
}

// Orchestrates the two onboarding steps (static form, then confirmation
// with inline editing) and owns the draft being filled in.
function OnboardingScreen({ onComplete }: OnboardingScreenProps) {
  const [step, setStep] = useState<Step>('form')
  const [draft, setDraft] = useState<ProfileDraft>(emptyDraft())

  if (step === 'confirm') {
    return <OnboardingConfirmScreen draft={draft} onChange={setDraft} onConfirmed={onComplete} />
  }

  return (
    <OnboardingFormScreen draft={draft} onChange={setDraft} onNext={() => setStep('confirm')} />
  )
}

export default OnboardingScreen
