import { cn } from '@/lib/utils'
import { useTranslation } from 'react-i18next'

type LabKind = 'INTERVIEW' | 'PRACTICE'

interface LabSubmenuProps {
  kind: LabKind
  onSelect: (kind: LabKind) => void
}

// The two lists of the Architecture Lab, under its sidebar row while the lab is
// open, styled like the study sessions of a folder (a dot that glows when
// selected): Interview (the canonical problems) and Practice (the user's
// practice challenges). See
// specs/phases/phase-07-improve-arch-lab/03-canonical-interviews.md.
function LabSubmenu({ kind, onSelect }: LabSubmenuProps) {
  const { t } = useTranslation()
  return (
    <div
      role="group"
      aria-label={t('canonical.tabsLabel')}
      className="flex flex-col gap-0.5 py-0.5"
    >
      {(['INTERVIEW', 'PRACTICE'] as const).map((value) => {
        const selected = kind === value
        return (
          <button
            key={value}
            type="button"
            aria-current={selected ? 'true' : undefined}
            onClick={() => onSelect(value)}
            className={cn(
              'flex w-full cursor-pointer items-center gap-1.5 rounded-md py-1 pr-3 pl-10 text-left text-sm hover:bg-accent',
              selected && 'bg-secondary',
            )}
          >
            <span
              className={cn(
                'size-1.5 shrink-0 rounded-full',
                selected ? 'bg-primary shadow-[0_0_6px_1px_var(--primary)]' : 'bg-muted-foreground',
              )}
              aria-hidden="true"
            />
            <span className="truncate text-foreground">
              {value === 'INTERVIEW' ? t('canonical.interviewTab') : t('canonical.practiceTab')}
            </span>
          </button>
        )
      })}
    </div>
  )
}

export { LabSubmenu }
