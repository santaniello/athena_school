import type { FormEvent } from 'react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { TagInput } from '@/components/tag-input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { AuthLayout } from '@/components/auth-layout'
import type { ProfileDraft } from '@/lib/profile'
import { ASSISTANT_LANGUAGES, EXPERIENCE_LEVELS, STUDY_STYLES } from '@/lib/profileOptions'

interface OnboardingFormScreenProps {
  draft: ProfileDraft
  onChange: (draft: ProfileDraft) => void
  onNext: () => void
}

function isDraftComplete(draft: ProfileDraft): boolean {
  return (
    draft.name.trim() !== '' &&
    draft.assistantName.trim() !== '' &&
    draft.area.trim() !== '' &&
    draft.experienceLevel !== '' &&
    draft.goals.length > 0 &&
    draft.studyStyle !== '' &&
    draft.assistantLanguage !== ''
  )
}

// Single-page static form (per spec clarification: onboarding collects
// answers through a plain form, not an LLM-conducted conversation).
function OnboardingFormScreen({ draft, onChange, onNext }: OnboardingFormScreenProps) {
  function updateField<K extends keyof ProfileDraft>(field: K, value: ProfileDraft[K]) {
    onChange({ ...draft, [field]: value })
  }

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    onNext()
  }

  return (
    <AuthLayout title="Tell us about yourself">
      <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="onboarding-name">Name</Label>
          <Input
            id="onboarding-name"
            value={draft.name}
            onChange={(event) => updateField('name', event.target.value)}
            required
          />
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="onboarding-assistant-name">
            What would you like to call the assistant?
          </Label>
          <Input
            id="onboarding-assistant-name"
            value={draft.assistantName}
            onChange={(event) => updateField('assistantName', event.target.value)}
            required
          />
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="onboarding-assistant-language">Assistant language</Label>
          <Select
            value={draft.assistantLanguage}
            onValueChange={(value) => updateField('assistantLanguage', value)}
          >
            <SelectTrigger id="onboarding-assistant-language" className="w-full">
              <SelectValue placeholder="Select a language" />
            </SelectTrigger>
            <SelectContent>
              {ASSISTANT_LANGUAGES.map((language) => (
                <SelectItem key={language.value} value={language.value}>
                  {language.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="onboarding-area">Area of study or work</Label>
          <Input
            id="onboarding-area"
            value={draft.area}
            onChange={(event) => updateField('area', event.target.value)}
            required
          />
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="onboarding-experience-level">Experience level</Label>
          <Select
            value={draft.experienceLevel}
            onValueChange={(value) => updateField('experienceLevel', value)}
          >
            <SelectTrigger id="onboarding-experience-level" className="w-full">
              <SelectValue placeholder="Select a level" />
            </SelectTrigger>
            <SelectContent>
              {EXPERIENCE_LEVELS.map((level) => (
                <SelectItem key={level.value} value={level.value}>
                  {level.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="onboarding-goals">Goals</Label>
          <TagInput
            id="onboarding-goals"
            value={draft.goals}
            onChange={(goals) => updateField('goals', goals)}
            placeholder="Type a goal and press Enter"
          />
          <p className="text-xs text-muted-foreground">
            Press Enter or comma after each goal to add it.
          </p>
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="onboarding-study-style">Preferred study style</Label>
          <Select
            value={draft.studyStyle}
            onValueChange={(value) => updateField('studyStyle', value)}
          >
            <SelectTrigger id="onboarding-study-style" className="w-full">
              <SelectValue placeholder="Select a style" />
            </SelectTrigger>
            <SelectContent>
              {STUDY_STYLES.map((style) => (
                <SelectItem key={style.value} value={style.value}>
                  {style.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <Button type="submit" disabled={!isDraftComplete(draft)}>
          Continue
        </Button>
      </form>
    </AuthLayout>
  )
}

export default OnboardingFormScreen
