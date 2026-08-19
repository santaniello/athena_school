import { useState } from 'react'
import type { KeyboardEvent } from 'react'
import { XIcon } from 'lucide-react'
import { cn } from '@/lib/utils'

interface TagInputProps {
  id?: string
  value: string[]
  onChange: (value: string[]) => void
  placeholder?: string
  className?: string
  'aria-label'?: string
}

// Free-text tag list backed by a plain string[] (used for UserProfile.Goals):
// Enter or comma commits the current draft as a tag, Backspace on an empty
// draft removes the last tag.
function TagInput({
  id,
  value,
  onChange,
  placeholder,
  className,
  'aria-label': ariaLabel,
}: TagInputProps) {
  const [draft, setDraft] = useState('')

  function addTag(rawTag: string) {
    const tag = rawTag.trim()
    if (tag === '' || value.includes(tag)) return
    onChange([...value, tag])
  }

  function removeTag(tag: string) {
    onChange(value.filter((existing) => existing !== tag))
  }

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'Enter' || event.key === ',') {
      event.preventDefault()
      addTag(draft)
      setDraft('')
      return
    }
    if (event.key === 'Backspace' && draft === '' && value.length > 0) {
      removeTag(value[value.length - 1])
    }
  }

  return (
    <div
      className={cn(
        'flex flex-wrap items-center gap-1.5 rounded-lg border border-input bg-transparent p-1.5',
        className,
      )}
    >
      {value.map((tag) => (
        <span
          key={tag}
          className="flex items-center gap-1 rounded-md bg-secondary px-2 py-0.5 text-sm text-secondary-foreground"
        >
          {tag}
          <button
            type="button"
            aria-label={`Remove ${tag}`}
            onClick={() => removeTag(tag)}
            className="cursor-pointer text-secondary-foreground/70 hover:text-secondary-foreground"
          >
            <XIcon className="size-3" />
          </button>
        </span>
      ))}
      <input
        id={id}
        aria-label={ariaLabel}
        type="text"
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={value.length === 0 ? placeholder : undefined}
        className="min-w-24 flex-1 bg-transparent px-1 py-0.5 text-sm text-foreground outline-none placeholder:text-muted-foreground"
      />
    </div>
  )
}

export { TagInput }
