import { cn } from '@/lib/utils'
import { useTranslation } from 'react-i18next'

type LabKind = 'INTERVIEW' | 'PRACTICE'

interface LabSubmenuProps {
  kind: LabKind
  onSelect: (kind: LabKind) => void
}

// The two lists of the Architecture Lab, under its sidebar row while the lab is
// open: Interview (the canonical problems) and Practice (the user's practice
// challenges). See specs/phases/phase-07-improve-arch-lab/03-canonical-interviews.md.
function LabSubmenu({ kind, onSelect }: LabSubmenuProps) {
  const { t } = useTranslation()
  return (
    <div
      role="group"
      aria-label={t('canonical.tabsLabel')}
      className="mt-0.5 flex flex-col gap-0.5 pl-6"
    >
      {(['INTERVIEW', 'PRACTICE'] as const).map((value) => (
        <button
          key={value}
          type="button"
          aria-current={kind === value ? 'true' : undefined}
          onClick={() => onSelect(value)}
          className={cn(
            'w-full cursor-pointer rounded-lg px-2.5 py-1.5 text-left text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground',
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
