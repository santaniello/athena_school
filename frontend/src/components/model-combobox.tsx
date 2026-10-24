import { useId, useMemo, useState } from 'react'
import type { FocusEvent, KeyboardEvent } from 'react'
import { ChevronDown } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

// The catalog has hundreds of models; rendering them all would make the list
// slow and unreadable, so only the first matches show and the search narrows.
const MAX_VISIBLE_OPTIONS = 50

interface ModelComboboxProps {
  // Accessible name of the control, unique per setting (e.g. "Study chat model").
  label: string
  value: string
  defaultValue: string
  options: string[]
  disabled?: boolean
  onSelect: (modelId: string) => void
}

// A searchable model picker. A native select cannot filter a catalog this
// large, and the Radix Select used elsewhere has no search box.
function ModelCombobox({
  label,
  value,
  defaultValue,
  options,
  disabled,
  onSelect,
}: ModelComboboxProps) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const listId = useId()

  const matches = useMemo(() => {
    const needle = query.trim().toLowerCase()
    return options.filter((option) => option.toLowerCase().includes(needle))
  }, [options, query])
  const visible = matches.slice(0, MAX_VISIBLE_OPTIONS)
  const hidden = matches.length - visible.length

  function close() {
    setOpen(false)
    setQuery('')
  }

  function choose(modelId: string) {
    close()
    if (modelId !== value) onSelect(modelId)
  }

  function handleKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key === 'Escape' && open) {
      event.stopPropagation()
      close()
    }
  }

  function handleBlur(event: FocusEvent<HTMLDivElement>) {
    if (!event.currentTarget.contains(event.relatedTarget)) close()
  }

  return (
    <div className="relative" onKeyDown={handleKeyDown} onBlur={handleBlur}>
      <Button
        type="button"
        variant="outline"
        role="combobox"
        aria-label={label}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listId : undefined}
        disabled={disabled}
        className="w-full justify-between font-normal"
        onClick={() => (open ? close() : setOpen(true))}
      >
        <span className="truncate">{value}</span>
        <span className="flex shrink-0 items-center gap-2">
          {value === defaultValue && <span className="text-xs text-muted-foreground">default</span>}
          <ChevronDown className="size-4" aria-hidden="true" />
        </span>
      </Button>

      {open && (
        <div className="absolute z-20 mt-1 w-full rounded-md border border-border bg-popover p-2 shadow-md">
          <Input
            autoFocus
            aria-label={`Search ${label}`}
            placeholder="Search models"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
          <ul
            id={listId}
            role="listbox"
            aria-label={`${label} options`}
            className="thin-scroll mt-2 max-h-60 overflow-y-auto pr-2"
          >
            {visible.map((option) => (
              <li key={option} role="presentation">
                <button
                  type="button"
                  role="option"
                  aria-selected={option === value}
                  className="w-full rounded-sm px-2 py-1.5 text-left text-sm hover:bg-accent hover:text-accent-foreground aria-selected:bg-accent"
                  onClick={() => choose(option)}
                >
                  {option === defaultValue ? `${option} (default)` : option}
                </button>
              </li>
            ))}
          </ul>
          {visible.length === 0 && (
            <p className="px-2 py-1.5 text-sm text-muted-foreground">
              No models match "{query.trim()}".
            </p>
          )}
          {hidden > 0 && (
            <p className="px-2 py-1.5 text-xs text-muted-foreground">
              {hidden} more — refine your search.
            </p>
          )}
        </div>
      )}
    </div>
  )
}

export { ModelCombobox }
