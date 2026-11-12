import { useEffect, useRef, useState } from 'react'
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
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  DIFFICULTY_LABELS,
  KIND_LABELS,
  cancelChallengeGeneration,
  exerciseErrorCode,
  exerciseErrorMessage,
  formatSessionDate,
  generateChallenge,
  listSessionsForGeneration,
  practiceSuggestions,
  type ChallengeDifficulty,
  type ChallengeKind,
  type GeneratedChallenge,
  type SessionOption,
} from '@/lib/exercise'

interface GenerateChallengeDialogProps {
  open: boolean
  // Preselected from the profile's experience level.
  defaultDifficulty: ChallengeDifficulty
  onClose: () => void
  // Called with the new challenge; the caller closes the dialog and reloads.
  onGenerated: (challenge: GeneratedChallenge) => void
  // Called instead when the dialog was already closed by the time the
  // challenge arrived: it exists in the catalog, but nothing on screen is
  // waiting for it, so the caller must not touch a dialog opened since.
  onGeneratedAfterClose: (challenge: GeneratedChallenge) => void
}

const DIFFICULTIES = Object.keys(DIFFICULTY_LABELS) as ChallengeDifficulty[]
const KINDS = Object.keys(KIND_LABELS) as ChallengeKind[]

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
  onGeneratedAfterClose,
}: Omit<GenerateChallengeDialogProps, 'open'>) {
  const [sessions, setSessions] = useState<Sessions>({ state: 'loading' })
  const [reloadToken, setReloadToken] = useState(0)
  const [search, setSearch] = useState('')
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [difficulty, setDifficulty] = useState(defaultDifficulty)
  const [kind, setKind] = useState<ChallengeKind>('INTERVIEW')
  const [goal, setGoal] = useState('')
  const [generating, setGenerating] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // False once the dialog has closed, so a late answer is told apart from one
  // this instance is still waiting for.
  // Stryker disable next-line BooleanLiteral: the mount effect below sets it before any answer can arrive.
  const mountedRef = useRef(false)

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
    }
  }, [])

  // Stryker disable ConditionalExpression,BlockStatement,BooleanLiteral: the
  // stale flag only matters when this dialog unmounts before the list
  // arrives, and a state update on an unmounted component has no visible
  // effect to assert on.
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
  // Stryker restore ConditionalExpression,BlockStatement,BooleanLiteral

  function retryLoading() {
    setSessions({ state: 'loading' })
    // Stryker disable next-line ArithmeticOperator: any change of the token re-runs the effect.
    setReloadToken((token) => token + 1)
  }

  async function handleGenerate() {
    // Stryker disable next-line ConditionalExpression: Generate is disabled
    // until a session is chosen, so this guard is unreachable from the UI.
    if (!selectedId) return
    setError(null)
    setGenerating(true)
    try {
      const challenge = await generateChallenge(
        selectedId,
        difficulty,
        kind,
        practice ? trimmedGoal : '',
      )
      if (mountedRef.current) onGenerated(challenge)
      else onGeneratedAfterClose(challenge)
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

  const practice = kind === 'PRACTICE'
  const trimmedGoal = goal.trim()
  const list = sessions.state === 'ready' ? sessions.list : []
  const selectedTopic = list.find((session) => session.id === selectedId)?.topic
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
                // A native radio gives one tab stop and the arrow keys for free.
                <label
                  key={session.id}
                  className="flex w-full cursor-pointer flex-col rounded-lg border border-border px-3 py-2 text-left text-sm transition-colors hover:bg-accent has-[:checked]:border-primary has-[:checked]:bg-primary/10 has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-60 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-ring"
                >
                  <input
                    type="radio"
                    name="study-session"
                    className="sr-only"
                    checked={session.id === selectedId}
                    disabled={generating}
                    onChange={() => setSelectedId(session.id)}
                  />
                  <span className="font-medium text-foreground">{session.topic}</span>
                  <span className="text-muted-foreground">
                    {session.folderName} · {formatSessionDate(session.startedAt)}
                  </span>
                </label>
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
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground">Type</span>
              <Select
                value={kind}
                disabled={generating}
                onValueChange={(value) => setKind(value as ChallengeKind)}
              >
                <SelectTrigger aria-label="Type">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {KINDS.map((value) => (
                    <SelectItem key={value} value={value}>
                      {KIND_LABELS[value]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {practice && (
              <div className="space-y-2">
                <label htmlFor="practice-goal" className="text-sm text-muted-foreground">
                  What do you want to practice?
                </label>
                <Textarea
                  id="practice-goal"
                  value={goal}
                  disabled={generating}
                  placeholder="E.g. cache aside, a slow API that another one depends on…"
                  onChange={(event) => setGoal(event.target.value)}
                />
                <div className="flex flex-wrap gap-2">
                  {practiceSuggestions(selectedTopic).map((suggestion) => (
                    <Button
                      key={suggestion}
                      type="button"
                      size="xs"
                      variant="outline"
                      disabled={generating}
                      onClick={() => setGoal(suggestion)}
                    >
                      {suggestion}
                    </Button>
                  ))}
                </div>
              </div>
            )}
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
            <Button
              type="button"
              disabled={!selectedId || (practice && trimmedGoal === '')}
              onClick={() => void handleGenerate()}
            >
              {error ? 'Try again' : 'Generate'}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
