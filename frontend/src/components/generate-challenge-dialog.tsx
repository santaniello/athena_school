import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  DIFFICULTY_LABELS,
  cancelChallengeGeneration,
  exerciseErrorCode,
  exerciseErrorMessage,
  formatSessionDate,
  generateChallenge,
  listSessionsForGeneration,
  type ChallengeDifficulty,
  type GeneratedChallenge,
  type SessionOption,
} from '@/lib/exercise'
import { cn } from '@/lib/utils'

interface GenerateChallengeDialogProps {
  open: boolean
  // Preselected from the profile's experience level.
  defaultDifficulty: ChallengeDifficulty
  onClose: () => void
  // Called with the new challenge; the caller closes the dialog and reloads.
  onGenerated: (challenge: GeneratedChallenge) => void
}

const DIFFICULTIES = Object.keys(DIFFICULTY_LABELS) as ChallengeDifficulty[]

// Opened by "Generate challenge with AI". The content is its own component, so
// the chosen session, the search text and any error are discarded each time
// the dialog closes. See
// specs/phases/phase-03-architecture-lab/07-challenge-catalog-ui.md.
export function GenerateChallengeDialog({ open, ...rest }: GenerateChallengeDialogProps) {
  return open ? <GenerateChallengeContent {...rest} /> : null
}

type Sessions =
  { state: 'loading' } | { state: 'failed' } | { state: 'ready'; list: SessionOption[] }

function GenerateChallengeContent({
  defaultDifficulty,
  onClose,
  onGenerated,
}: Omit<GenerateChallengeDialogProps, 'open'>) {
  const [sessions, setSessions] = useState<Sessions>({ state: 'loading' })
  const [reloadToken, setReloadToken] = useState(0)
  const [search, setSearch] = useState('')
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [difficulty, setDifficulty] = useState(defaultDifficulty)
  const [generating, setGenerating] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let stale = false
    listSessionsForGeneration()
      .then((list) => {
        if (!stale) setSessions({ state: 'ready', list })
      })
      .catch(() => {
        if (!stale) setSessions({ state: 'failed' })
      })
    return () => {
      stale = true
    }
  }, [reloadToken])

  function retryLoading() {
    setSessions({ state: 'loading' })
    setReloadToken((token) => token + 1)
  }

  async function handleGenerate() {
    if (!selectedId) return
    setError(null)
    setGenerating(true)
    try {
      onGenerated(await generateChallenge(selectedId, difficulty))
    } catch (err) {
      // A cancelled generation is the user's own doing: back to the choice, quietly.
      if (exerciseErrorCode(err) !== 'cancelled') setError(exerciseErrorMessage(err))
      setGenerating(false)
    }
  }

  // Dismissing the dialog (Escape, the close button) must not leave a
  // generation running that nobody is waiting for.
  function handleDismiss() {
    if (generating) void cancelChallengeGeneration()
    onClose()
  }

  const list = sessions.state === 'ready' ? sessions.list : []
  const needle = search.trim().toLowerCase()
  const visible = list.filter(
    (session) =>
      session.topic.toLowerCase().includes(needle) ||
      session.folderName.toLowerCase().includes(needle),
  )

  return (
    <Dialog open onOpenChange={(nextOpen) => !nextOpen && handleDismiss()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Generate challenge with AI</DialogTitle>
          <DialogDescription>
            Pick a study session. The challenge is generated from what you studied in it.
          </DialogDescription>
        </DialogHeader>

        {sessions.state === 'loading' && (
          <p className="text-sm text-muted-foreground">Loading your study sessions…</p>
        )}
        {sessions.state === 'failed' && (
          <div role="alert" className="flex items-center justify-between gap-3 text-sm">
            <span className="text-destructive">Could not load your study sessions.</span>
            <Button type="button" variant="outline" size="sm" onClick={retryLoading}>
              Retry
            </Button>
          </div>
        )}
        {sessions.state === 'ready' && list.length === 0 && (
          <p className="text-sm text-muted-foreground">
            Study a topic first. A challenge is generated from a session you have already studied.
          </p>
        )}
        {sessions.state === 'ready' && list.length > 0 && (
          <div className="space-y-3">
            <Input
              type="search"
              aria-label="Search study sessions"
              placeholder="Search by topic or folder"
              value={search}
              disabled={generating}
              onChange={(event) => setSearch(event.target.value)}
            />
            <div
              role="radiogroup"
              aria-label="Study sessions"
              className="thin-scroll max-h-64 space-y-1 overflow-y-auto"
            >
              {visible.map((session) => (
                <button
                  key={session.id}
                  type="button"
                  role="radio"
                  aria-checked={session.id === selectedId}
                  disabled={generating}
                  onClick={() => setSelectedId(session.id)}
                  className={cn(
                    'flex w-full cursor-pointer flex-col rounded-lg border border-border px-3 py-2 text-left text-sm transition-colors hover:bg-accent disabled:cursor-not-allowed disabled:opacity-60',
                    session.id === selectedId && 'border-primary bg-primary/10',
                  )}
                >
                  <span className="font-medium text-foreground">{session.topic}</span>
                  <span className="text-muted-foreground">
                    {session.folderName} · {formatSessionDate(session.startedAt)}
                  </span>
                </button>
              ))}
              {visible.length === 0 && (
                <p className="py-2 text-sm text-muted-foreground">
                  No study session matches your search.
                </p>
              )}
            </div>
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground">Difficulty</span>
              <Select
                value={difficulty}
                disabled={generating}
                onValueChange={(value) => setDifficulty(value as ChallengeDifficulty)}
              >
                <SelectTrigger aria-label="Difficulty">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {DIFFICULTIES.map((value) => (
                    <SelectItem key={value} value={value}>
                      {DIFFICULTY_LABELS[value]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        )}

        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        {generating && (
          <p role="status" className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            Generating the challenge. This can take a minute…
          </p>
        )}

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={generating ? () => void cancelChallengeGeneration() : onClose}
          >
            Cancel
          </Button>
          {sessions.state === 'ready' && list.length > 0 && !generating && (
            <Button type="button" disabled={!selectedId} onClick={() => void handleGenerate()}>
              {error ? 'Try again' : 'Generate'}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
