import { cn } from '@/lib/utils'
import { useTranslation } from 'react-i18next'

type LabKind = 'INTERVIEW' | 'PRACTICE'

interface LabSubmenuProps {
  kind: LabKind
  onSelect: (kind: LabKind) => void
}

// The two lists of the Architecture Lab, under its sidebar row while the lab is
// open, drawn as a tree (a trunk with a branch to each item): Interview (the canonical problems) and Practice (the user's practice
// challenges). See specs/phases/phase-07-improve-arch-lab/03-canonical-interviews.md.
function LabSubmenu({ kind, onSelect }: LabSubmenuProps) {
  const { t } = useTranslation()
  return (
    <div
      role="group"
      aria-label={t('canonical.tabsLabel')}
      className="mt-0.5 ml-5 flex flex-col gap-0.5 border-l border-border"
    >
      {(['INTERVIEW', 'PRACTICE'] as const).map((value) => (
        <button
          key={value}
          type="button"
          aria-current={kind === value ? 'true' : undefined}
          onClick={() => onSelect(value)}
          className={cn(
            'relative ml-3 w-[calc(100%-0.75rem)] cursor-pointer rounded-lg px-2.5 py-1.5 text-left before:absolute before:top-1/2 before:-left-3 before:h-px before:w-3 before:bg-border text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground',
            kind === value && 'bg-primary/10 font-medium text-foreground',
          )}
        >
          {value === 'INTERVIEW' ? t('canonical.interviewTab') : t('canonical.practiceTab')}
        </button>
      ))}
    </div>
  )
}

export { LabSubmenu }
