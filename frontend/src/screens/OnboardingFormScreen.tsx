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

interface OnboardingFormScreenProps {
  draft: ProfileDraft
  onChange: (draft: ProfileDraft) => void
  onNext: () => void
}

const EXPERIENCE_LEVELS: Array<{ value: string; label: string }> = [
  { value: 'beginner', label: 'Iniciante' },
  { value: 'intermediate', label: 'Intermediário' },
  { value: 'advanced', label: 'Avançado' },
]

function isDraftComplete(draft: ProfileDraft): boolean {
  return (
    draft.name.trim() !== '' &&
    draft.assistantName.trim() !== '' &&
    draft.area.trim() !== '' &&
    draft.specialty.trim() !== '' &&
    draft.experienceLevel !== '' &&
    draft.goals.length > 0 &&
    draft.studyStyle.trim() !== ''
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
    <AuthLayout title="Conte sobre você">
      <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="onboarding-name">Nome</Label>
          <Input
            id="onboarding-name"
            value={draft.name}
            onChange={(event) => updateField('name', event.target.value)}
            required
          />
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="onboarding-assistant-name">Como quer chamar o assistente?</Label>
          <Input
            id="onboarding-assistant-name"
            value={draft.assistantName}
            onChange={(event) => updateField('assistantName', event.target.value)}
            required
          />
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="onboarding-area">Área de atuação ou estudo</Label>
          <Input
            id="onboarding-area"
            value={draft.area}
            onChange={(event) => updateField('area', event.target.value)}
            required
          />
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="onboarding-specialty">Foco específico</Label>
          <Input
            id="onboarding-specialty"
            value={draft.specialty}
            onChange={(event) => updateField('specialty', event.target.value)}
            required
          />
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="onboarding-experience-level">Nível de experiência</Label>
          <Select
            value={draft.experienceLevel}
            onValueChange={(value) => updateField('experienceLevel', value)}
          >
            <SelectTrigger id="onboarding-experience-level" className="w-full">
              <SelectValue placeholder="Selecione um nível" />
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
          <Label htmlFor="onboarding-goals">Objetivos</Label>
          <TagInput
            id="onboarding-goals"
            value={draft.goals}
            onChange={(goals) => updateField('goals', goals)}
            placeholder="Digite um objetivo e pressione Enter"
          />
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="onboarding-study-style">Estilo de estudo preferido</Label>
          <Input
            id="onboarding-study-style"
            value={draft.studyStyle}
            onChange={(event) => updateField('studyStyle', event.target.value)}
            required
          />
        </div>

        <Button type="submit" disabled={!isDraftComplete(draft)}>
          Continuar
        </Button>
      </form>
    </AuthLayout>
  )
}

export default OnboardingFormScreen
