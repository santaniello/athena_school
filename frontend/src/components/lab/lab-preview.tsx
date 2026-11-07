import { useState } from 'react'
import { MAX_HINTS } from '@/lib/lab'
import { LabLayout } from './lab-layout'
import {
  previewChallenge,
  previewCriteria,
  previewDeadline,
  previewHints,
  previewStatement,
} from './lab-preview-fixtures'

function noop() {}

// Development-only preview of the lab layout, rendered from fixtures. It is
// reachable only when import.meta.env.DEV is true, is not in the navigation
// and creates no attempt. See
// specs/phases/phase-03-architecture-lab/08-lab-layout-sketch.md.
function LabPreview() {
  const [deadline] = useState(() => previewDeadline(Date.now()))
  const [hintsShown, setHintsShown] = useState(0)

  return (
    <LabLayout
      title={previewChallenge.title}
      topic={previewChallenge.topic}
      difficulty={previewChallenge.difficulty}
      statement={previewStatement}
      criteria={previewCriteria}
      deadline={deadline}
      hints={previewHints.slice(0, hintsShown)}
      hintsLeft={MAX_HINTS - hintsShown}
      canFinish={false}
      board={
        <p className="flex h-full items-center justify-center p-6 text-center text-sm text-muted-foreground">
          The diagram editor will appear here.
        </p>
      }
      onExpire={noop}
      onAskHint={() => setHintsShown((shown) => Math.min(shown + 1, MAX_HINTS))}
      onFinish={noop}
      onLeave={noop}
    />
  )
}

export { LabPreview }
