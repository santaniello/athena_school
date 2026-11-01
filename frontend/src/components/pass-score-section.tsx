import { useCallback, useEffect, useState } from 'react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  getPassScore,
  PASS_SCORE_RANGE_MESSAGE,
  passScoreErrorMessage,
  setPassScore,
} from '@/lib/passScore'

// The Settings screen's "Challenges" section: the minimum score both the
// diagram and the explanation need for a challenge to count as passed. It is
// saved on request, not on every keystroke, since a half-typed number is
// not a choice. See specs/phases/phase-03-architecture-lab/06-completion-rules.md.
function PassScoreSection() {
  // The input keeps the raw text, so an emptied field is not coerced to 0.
  const [value, setValue] = useState<string | null>(null)
  const [loadFailed, setLoadFailed] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)

  const load = useCallback(() => {
    void getPassScore().then(
      (score) => setValue(String(score)),
      () => setLoadFailed(true),
    )
  }, [])

  function retry() {
    setLoadFailed(false)
    load()
  }

  // Stryker disable ArrayDeclaration: mount-once effect — its dependency
  // array's content is not itself observable behavior.
  useEffect(() => {
    load()
  }, [load])
  // Stryker restore ArrayDeclaration

  function edit(next: string) {
    setValue(next)
    setError('')
    setSaved(false)
  }

  async function save(event: React.FormEvent) {
    event.preventDefault()
    setError('')
    setSaved(false)
    const score = Number(value)
    if (value === '' || !Number.isInteger(score)) {
      setError(PASS_SCORE_RANGE_MESSAGE)
      return
    }
    setSaving(true)
    try {
      await setPassScore(score)
      setSaved(true)
    } catch (err) {
      setError(passScoreErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  const heading = <h2 className="font-heading text-lg font-semibold text-foreground">Challenges</h2>

  if (loadFailed) {
    return (
      <div className="flex flex-col gap-4">
        {heading}
        <Alert variant="destructive">
          <AlertDescription>Could not load the minimum passing score.</AlertDescription>
        </Alert>
        <Button type="button" variant="outline" className="self-start" onClick={retry}>
          Retry
        </Button>
      </div>
    )
  }

  if (value === null) {
    return (
      <div className="flex flex-col gap-4">
        {heading}
        <p className="text-sm text-muted-foreground">Loading...</p>
      </div>
    )
  }

  return (
    <form className="flex flex-col gap-4" onSubmit={(event) => void save(event)} noValidate>
      {heading}
      <div className="flex flex-col gap-1.5 text-left">
        <Label htmlFor="pass-score">Minimum passing score</Label>
        <Input
          id="pass-score"
          type="number"
          min={50}
          max={100}
          step={1}
          value={value}
          onChange={(event) => edit(event.target.value)}
          aria-invalid={error !== ''}
          disabled={saving}
          className="max-w-32"
        />
        <p className="text-xs text-muted-foreground">
          A challenge is passed when both the diagram and your explanation reach this score.
          Changing it does not affect attempts you already finished.
        </p>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      {saved && (
        <p role="status" className="text-sm text-foreground">
          Saved.
        </p>
      )}

      <Button type="submit" className="self-start" disabled={saving}>
        {saving ? 'Saving...' : 'Save'}
      </Button>
    </form>
  )
}

export { PassScoreSection }
